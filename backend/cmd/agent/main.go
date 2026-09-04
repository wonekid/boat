// osp-agent —— boat 运维平台执行机代理程序。
//
// 运行方式：部署在被管执行机上，以系统服务（systemd）常驻，主动外连控制台 OSP 端口，
// 建立加密长连接后待命：接收控制台下发的命令/脚本任务并执行、回传结果，同时周期性上报心跳与系统指标。
//
// 与 SSH 通道的本质区别：本通道由执行机**主动外连**，控制台无需知道执行机账号密码，
// 因此即便执行机 SSH 无法登录（密码过期、账户锁定、sshd 配置错误、sudoers 损坏），
// 只要 agent 进程存活，控制台依然可以下发指令完成应急修复。
//
// 用法：
//
//	osp-agent -c /opt/osp-agent/agent.yaml
//	OSP_SERVER=10.0.0.1:9090 OSP_TOKEN=osp_xxx osp-agent
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"boat/internal/osp"

	"gopkg.in/yaml.v3"
)

// Version Agent 版本
const Version = "1.0.0"

// Config Agent 配置
type Config struct {
	Server       string `yaml:"server"`        // 控制台 OSP 地址，host:port
	Token        string `yaml:"token"`         // 节点接入令牌
	Name         string `yaml:"name"`          // 节点名称（为空则用主机名）
	Labels       string `yaml:"labels"`        // 标签，逗号分隔
	Heartbeat    int    `yaml:"heartbeat"`     // 心跳周期（秒），服务端可覆盖
	ServerPubKey string `yaml:"server-pubkey"` // 服务端 RSA 公钥文件路径（校验握手签名，防中间人）
	WorkDir      string `yaml:"workdir"`       // 脚本临时目录
	Insecure     bool   `yaml:"insecure"`      // 跳过服务端签名校验（仅内网测试用，不建议生产开启）
}

var (
	cfgPath  = flag.String("c", "agent.yaml", "配置文件路径")
	srvAddr  = flag.String("server", "", "控制台 OSP 地址 host:port（覆盖配置文件）")
	tokenArg = flag.String("token", "", "接入令牌（覆盖配置文件）")
	nameArg  = flag.String("name", "", "节点名称（覆盖配置文件）")
	labels   = flag.String("labels", "", "节点标签（覆盖配置文件）")
	insecure = flag.Bool("insecure", false, "跳过服务端签名校验（不推荐）")
	showVer  = flag.Bool("v", false, "打印版本")
)

func main() {
	flag.Parse()
	if *showVer {
		fmt.Printf("osp-agent %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		return
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("[agent] 配置加载失败: %v", err)
	}
	if cfg.Server == "" {
		log.Fatal("[agent] 未配置控制台地址（server）")
	}
	if cfg.Token == "" {
		log.Fatal("[agent] 未配置接入令牌（token）")
	}
	if cfg.Heartbeat <= 0 {
		cfg.Heartbeat = 10
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = os.TempDir()
	}
	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		log.Printf("[agent] 工作目录不可用(%s): %v，回退系统临时目录", cfg.WorkDir, err)
		cfg.WorkDir = os.TempDir()
	}

	pubKey, keyWarn := loadServerPubKey(cfg.ServerPubKey)
	if keyWarn != "" {
		log.Printf("[agent] 安全提示: %s", keyWarn)
	}

	sys := collectSysInfo(cfg)
	log.Printf("[agent] osp-agent %s 启动：节点 %s（%s/%s）→ 控制台 %s", Version, sys.Hostname, sys.OS, sys.Arch, cfg.Server)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[agent] 收到退出信号，正在关闭")
		cancel()
		os.Exit(0)
	}()

	// 断线重连：指数退避 1s → 60s
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := serve(ctx, cfg, sys, pubKey)
		if err != nil {
			log.Printf("[agent] 连接中断: %v，%v 后重试", err, backoff)
		} else {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 60*time.Second {
			backoff *= 2
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
		}
	}
}

// serve 建立一次完整会话：握手 → 心跳与读循环（返回即代表连接结束）
func serve(ctx context.Context, cfg *Config, sys SysInfo, pubKey []byte) error {
	dialer := net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.Server)
	if err != nil {
		return fmt.Errorf("连接控制台失败: %w", err)
	}
	defer conn.Close()
	log.Printf("[agent] 已连接控制台 %s", cfg.Server)

	welcome, sess, err := handshake(conn, cfg, sys, pubKey)
	if err != nil {
		return err
	}
	hb := welcome.Heartbeat
	if hb <= 0 {
		hb = cfg.Heartbeat
	}
	log.Printf("[agent] 握手成功：节点 %s(id=%d) 会话 %s，心跳 %ds，加密通道已建立(AES-256-GCM)",
		welcome.NodeName, welcome.NodeID, welcome.SessionID, hb)

	// 握手后：清除握手阶段设置的整体 deadline，并启用 TCP keepalive 检测死连接。
	// 服务端空闲时不会主动向 agent 发帧，故 agent 侧不设置应用层读超时，
	// 否则静默期会被误判为断线；server 优雅关闭会发 FIN、崩溃/网络中断由 keepalive 探测。
	_ = conn.SetDeadline(time.Time{})
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(15 * time.Second)
	}

	// 会话级 context：连接结束（serve 返回）时取消，停止心跳协程，避免泄漏。
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()

	exec := newExecutor(cfg)
	writeMu := new(sync.Mutex)
	send := func(msgType string, payload interface{}) error {
		env, err := osp.NewEnvelope(msgType, payload)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		return osp.WriteFrame(conn, sess, env)
	}

	// 心跳协程：上报实时指标（监听 sessionCtx，连接结束即停止）
	go func() {
		ticker := time.NewTicker(time.Duration(hb) * time.Second)
		defer ticker.Stop()
		sendHeartbeat(send)
		for {
			select {
			case <-sessionCtx.Done():
				return
			case <-ticker.C:
				sendHeartbeat(send)
			}
		}
	}()

	// 读循环：接收任务（阻塞读；连接断开/超时由底层 TCP 感知，返回即结束本次会话）
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		env, err := osp.ReadFrame(conn, sess)
		if err != nil {
			return fmt.Errorf("读取消息失败: %w", err)
		}
		switch env.Type {
		case osp.MsgTask:
			var t osp.TaskPayload
			if err := env.Decode(&t); err != nil {
				log.Printf("[agent] 任务解析失败: %v", err)
				continue
			}
			// 并发执行，长任务不阻塞心跳与后续任务
			go func(task osp.TaskPayload) {
				log.Printf("[agent] 收到任务 resultId=%d 类型=%s 语言=%s", task.ResultID, task.Type, task.Lang)
				res := exec.Run(&task)
				if err := send(osp.MsgTaskResult, res); err != nil {
					log.Printf("[agent] 结果回传失败: %v", err)
					return
				}
				log.Printf("[agent] 任务 resultId=%d 完成，状态=%s 退出码=%d 耗时=%dms",
					task.ResultID, res.Status, res.ExitCode, res.Duration)
			}(t)
		case osp.MsgCancel:
			var c osp.CancelPayload
			if err := env.Decode(&c); err == nil {
				exec.Cancel(c.ResultID)
			}
		case osp.MsgPing:
			_ = send(osp.MsgPong, nil)
		default:
			log.Printf("[agent] 忽略未知消息: %s", env.Type)
		}
	}
}

func sendHeartbeat(send func(string, interface{}) error) {
	if err := send(osp.MsgHeartbeat, &osp.HeartbeatPayload{Metrics: collectMetrics(Version)}); err != nil {
		log.Printf("[agent] 心跳上报失败: %v", err)
	}
}

// handshake 完成身份认证、密钥协商与服务端签名校验
func handshake(conn net.Conn, cfg *Config, sys SysInfo, pubKey []byte) (*osp.WelcomePayload, *osp.Session, error) {
	kex, err := osp.NewKeyExchange()
	if err != nil {
		return nil, nil, err
	}
	nonceA, nonceAB64, err := osp.RandomNonce()
	if err != nil {
		return nil, nil, err
	}
	hello := &osp.HelloPayload{
		Version:  Version,
		Token:    cfg.Token,
		NonceA:   nonceAB64,
		ECDHPubA: base64.StdEncoding.EncodeToString(kex.Pub),
		Hostname: sys.Hostname,
		OS:       sys.OS,
		Arch:     sys.Arch,
		IP:       sys.IP,
		Labels:   cfg.Labels,
	}
	env, err := osp.NewEnvelope(osp.MsgHello, hello)
	if err != nil {
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err := osp.WriteFrame(conn, nil, env); err != nil {
		return nil, nil, err
	}
	resp, err := osp.ReadFrame(conn, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("读取握手响应失败: %w", err)
	}
	if resp.Type == osp.MsgReject {
		var rj osp.RejectPayload
		_ = resp.Decode(&rj)
		return nil, nil, fmt.Errorf("%w: %s", osp.ErrRejected, rj.Reason)
	}
	if resp.Type != osp.MsgWelcome {
		return nil, nil, fmt.Errorf("期望 welcome 消息，实际收到 %s", resp.Type)
	}
	var welcome osp.WelcomePayload
	if err := resp.Decode(&welcome); err != nil {
		return nil, nil, err
	}
	nonceB, err := base64.StdEncoding.DecodeString(welcome.NonceB)
	if err != nil {
		return nil, nil, fmt.Errorf("nonceB 非法")
	}
	pubB, err := base64.StdEncoding.DecodeString(welcome.ECDHPubB)
	if err != nil {
		return nil, nil, fmt.Errorf("服务端 ECDH 公钥非法")
	}

	// 校验服务端签名（防中间人）：签名对象为 nonceA || nonceB || ecdhPubB
	if pubKey != nil {
		signed := make([]byte, 0, len(nonceA)+len(nonceB)+len(pubB))
		signed = append(signed, nonceA...)
		signed = append(signed, nonceB...)
		signed = append(signed, pubB...)
		if err := verifySignature(pubKey, signed, welcome.Signature); err != nil {
			return nil, nil, fmt.Errorf("服务端签名校验失败（疑似中间人攻击）: %w", err)
		}
	} else if !cfg.Insecure {
		log.Println("[agent] 警告：未配置服务端公钥，无法校验服务端身份；建议配置 server-pubkey 或显式 -insecure")
	}

	sess, err := kex.Derive(pubB, nonceA, nonceB, cfg.Token)
	if err != nil {
		return nil, nil, err
	}
	return &welcome, sess, nil
}

// ---------- 配置与系统信息 ----------

func loadConfig(path string) (*Config, error) {
	cfg := &Config{Heartbeat: 10}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	// 命令行 / 环境变量覆盖（优先级：命令行 > 环境变量 > 配置文件）
	if *srvAddr != "" {
		cfg.Server = *srvAddr
	}
	if v := os.Getenv("OSP_SERVER"); v != "" && cfg.Server == "" {
		cfg.Server = v
	}
	if *tokenArg != "" {
		cfg.Token = *tokenArg
	}
	if v := os.Getenv("OSP_TOKEN"); v != "" && cfg.Token == "" {
		cfg.Token = v
	}
	if *nameArg != "" {
		cfg.Name = *nameArg
	}
	if *labels != "" {
		cfg.Labels = *labels
	}
	if *insecure {
		cfg.Insecure = true
	}
	return cfg, nil
}

// loadServerPubKey 读取服务端 RSA 公钥 PEM，返回密钥与提示信息
func loadServerPubKey(path string) ([]byte, string) {
	if path == "" {
		return nil, "未配置 server-pubkey，将跳过服务端签名校验"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Sprintf("读取服务端公钥失败(%s): %v，将跳过签名校验", path, err)
	}
	return data, ""
}
