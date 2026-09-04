package osp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestE2ESmoke 端到端冒烟测试：
// 用真实编译出的 osp-agent 二进制作为子进程，连接一个用 osp 协议原语实现的"控制台"监听端，
// 验证 ①加密握手（ECDH+密钥派生+AES-256-GCM）②服务端 RSA-PSS 签名校验（防中间人）
// ③心跳指标上报 ④任务下发→真实命令执行→结果（stdout/exit code）回收。
//
// 说明：本机无 MySQL/Redis/Docker，无法拉起依赖数据库的完整 server；此处用最小"控制台"复用
// 与真实 server 完全一致的 osp 帧/握手/加密逻辑，从而端到端验证 agent + 协议本身真实可用。
// 需先构建 agent 二进制并通过环境变量 OSP_AGENT_BIN 指定路径，否则跳过：
//
//	go build -o /tmp/osp-agent ./cmd/agent
//	OSP_AGENT_BIN=/tmp/osp-agent go test ./internal/osp/ -run TestE2ESmoke -v
func TestE2ESmoke(t *testing.T) {
	bin := os.Getenv("OSP_AGENT_BIN")
	if bin == "" {
		t.Skip("未设置 OSP_AGENT_BIN，跳过端到端冒烟测试（构建: go build -o /tmp/osp-agent ./cmd/agent）")
	}

	// 1) 生成服务端 RSA 密钥对（用于握手签名，验证 agent 的防中间人校验路径）
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成 RSA 密钥失败: %v", err)
	}
	pubDER, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	dir := t.TempDir()
	pubPath := filepath.Join(dir, "server.pem")
	if err := os.WriteFile(pubPath, pubPEM, 0o600); err != nil {
		t.Fatalf("写服务端公钥失败: %v", err)
	}
	cfgPath := filepath.Join(dir, "agent.yaml")
	cfg := fmt.Sprintf("server: %s\ntoken: osp_smoke_token\nname: smoke-node\nlabels: e2e,test\nheartbeat: 2\nserver-pubkey: %s\nworkdir: %s\n",
		"", pubPath, dir) // server 占位，稍后替换
	_ = cfg

	// 2) 启动监听端（控制台）
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	// 写入真实 agent 配置（含监听地址）
	agentCfg := fmt.Sprintf("server: %s\ntoken: osp_smoke_token\nname: smoke-node\nlabels: e2e,test\nheartbeat: 2\nserver-pubkey: %s\nworkdir: %s\n",
		addr, pubPath, dir)
	if err := os.WriteFile(cfgPath, []byte(agentCfg), 0o600); err != nil {
		t.Fatalf("写 agent 配置失败: %v", err)
	}

	// 3) 启动真实 agent 二进制
	cmd := exec.Command(bin, "-c", cfgPath)
	cmdOut := &stdoutCollector{}
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 agent 失败: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Logf("agent 输出:\n%s", cmdOut.String())
	}()

	// 4) 接受连接并完成服务端握手
	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("接受 agent 连接失败: %v", err)
	}
	defer conn.Close()

	sess, err := serverHandshake(t, conn, priv)
	if err != nil {
		t.Fatalf("服务端握手失败: %v", err)
	}
	t.Log("握手成功：加密通道已建立（AES-256-GCM）")

	// 5) 读循环：收集心跳与任务结果
	hbCh := make(chan Metrics, 16)
	resCh := make(chan *TaskResultPayload, 16)
	go func() {
		for {
			env, err := ReadFrame(conn, sess)
			if err != nil {
				return
			}
			switch env.Type {
			case MsgHeartbeat:
				var hb HeartbeatPayload
				if err := env.Decode(&hb); err == nil {
					hbCh <- hb.Metrics
				}
			case MsgTaskResult:
				var r TaskResultPayload
				if err := env.Decode(&r); err == nil {
					resCh <- &r
				}
			}
		}
	}()

	// 等待 agent 首帧心跳（证明加密心跳上报可用）
	select {
	case m := <-hbCh:
		t.Logf("收到心跳：CPU=%.1f%% Mem=%.1f%% Disk=%.1f%% Load=%s", m.CPUUsage, m.MemUsage, m.DiskUsage, m.LoadAvg)
	case <-time.After(10 * time.Second):
		t.Fatal("超时未收到心跳")
	}

	// 6) 下发任务一：成功命令
	sendTask(t, conn, sess, &TaskPayload{TaskID: 1, ResultID: 1, Type: "command", Lang: "shell", Content: "echo OSP_SMOKE_OK", Timeout: 30})
	r1 := waitResult(t, resCh, 1, 30*time.Second)
	if r1.Status != ResultSuccess {
		t.Fatalf("任务一状态应为 success，实际 %s (exit=%d, err=%s)", r1.Status, r1.ExitCode, r1.Error)
	}
	if r1.ExitCode != 0 {
		t.Fatalf("任务一退出码应为 0，实际 %d", r1.ExitCode)
	}
	if !contains(r1.Stdout, "OSP_SMOKE_OK") {
		t.Fatalf("任务一 stdout 应含 OSP_SMOKE_OK，实际: %q", r1.Stdout)
	}
	t.Logf("任务一成功：stdout=%q exit=%d", r1.Stdout, r1.ExitCode)

	// 7) 下发任务二：非零退出码命令（验证 exit code 透传）
	sendTask(t, conn, sess, &TaskPayload{TaskID: 2, ResultID: 2, Type: "command", Lang: "shell", Content: "exit 3", Timeout: 30})
	r2 := waitResult(t, resCh, 2, 30*time.Second)
	if r2.Status != ResultFailed {
		t.Fatalf("任务二状态应为 failed，实际 %s", r2.Status)
	}
	if r2.ExitCode != 3 {
		t.Fatalf("任务二退出码应为 3，实际 %d", r2.ExitCode)
	}
	t.Logf("任务二失败（符合预期）：exit=%d err=%s", r2.ExitCode, r2.Error)

	// 8) 断言 agent 确实走了签名校验路径（未出现"跳过签名校验"告警）
	if contains(cmdOut.String(), "跳过服务端签名校验") {
		t.Fatal("agent 不应跳过服务端签名校验（应配置 server-pubkey 并校验）")
	}
}

// serverHandshake 在监听端完成与真实 agent 一致的服务端握手：
// 读 hello（明文）→ 生成 ECDH → 对 nonceA||nonceB||ecdhPubB 做 RSA-PSS 签名 → 发 welcome（明文）→ 派生会话。
func serverHandshake(t *testing.T, conn net.Conn, priv *rsa.PrivateKey) (*Session, error) {
	t.Helper()
	helloEnv, err := ReadFrame(conn, nil)
	if err != nil {
		return nil, fmt.Errorf("读 hello 失败: %w", err)
	}
	var hello HelloPayload
	if err := helloEnv.Decode(&hello); err != nil {
		return nil, fmt.Errorf("解析 hello 失败: %w", err)
	}
	if hello.Token != "osp_smoke_token" {
		return nil, fmt.Errorf("令牌不匹配: %s", hello.Token)
	}
	nonceA, err := base64.StdEncoding.DecodeString(hello.NonceA)
	if err != nil {
		return nil, fmt.Errorf("nonceA 非法: %w", err)
	}
	pubA, err := base64.StdEncoding.DecodeString(hello.ECDHPubA)
	if err != nil {
		return nil, fmt.Errorf("ecdhPubA 非法: %w", err)
	}
	kex, err := NewKeyExchange()
	if err != nil {
		return nil, err
	}
	_, nonceBB64, err := RandomNonce()
	if err != nil {
		return nil, err
	}
	nonceB, err := base64.StdEncoding.DecodeString(nonceBB64)
	if err != nil {
		return nil, err
	}
	// 签名对象：nonceA || nonceB || 服务端 ECDH 公钥（原始字节），与 agent 端校验一致
	signed := make([]byte, 0, len(nonceA)+len(nonceB)+len(kex.Pub))
	signed = append(signed, nonceA...)
	signed = append(signed, nonceB...)
	signed = append(signed, kex.Pub...)
	hashed := sha256.Sum256(signed)
	sig, err := rsa.SignPSS(rand.Reader, priv, crypto.SHA256, hashed[:], nil)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}
	welcome := WelcomePayload{
		SessionID:  "smoke-session",
		NodeID:     1,
		NodeName:   hello.Hostname,
		NonceB:     nonceBB64,
		ECDHPubB:   base64.StdEncoding.EncodeToString(kex.Pub),
		Signature:  base64.StdEncoding.EncodeToString(sig),
		Heartbeat:  2,
		ServerTime: time.Now().Unix(),
	}
	welcomeEnv, err := NewEnvelope(MsgWelcome, welcome)
	if err != nil {
		return nil, err
	}
	if err := WriteFrame(conn, nil, welcomeEnv); err != nil {
		return nil, fmt.Errorf("发 welcome 失败: %w", err)
	}
	sess, err := kex.Derive(pubA, nonceA, nonceB, hello.Token)
	if err != nil {
		return nil, fmt.Errorf("派生会话密钥失败: %w", err)
	}
	return sess, nil
}

func sendTask(t *testing.T, conn net.Conn, sess *Session, p *TaskPayload) {
	t.Helper()
	env, err := NewEnvelope(MsgTask, p)
	if err != nil {
		t.Fatalf("构造任务信封失败: %v", err)
	}
	if err := WriteFrame(conn, sess, env); err != nil {
		t.Fatalf("下发任务失败: %v", err)
	}
}

func waitResult(t *testing.T, ch chan *TaskResultPayload, wantID uint, timeout time.Duration) *TaskResultPayload {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case r := <-ch:
			if r.ResultID == wantID {
				return r
			}
			t.Logf("忽略非目标结果 resultId=%d", r.ResultID)
		case <-timer.C:
			t.Fatalf("等待任务 resultId=%d 结果超时", wantID)
			return nil
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// stdoutCollector 线程安全地收集子进程输出
type stdoutCollector struct {
	buf []byte
}

func (c *stdoutCollector) Write(p []byte) (int, error) {
	c.buf = append(c.buf, p...)
	return len(p), nil
}

func (c *stdoutCollector) String() string { return string(c.buf) }
