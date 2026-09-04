// Package osp 实现 boat 自研的 OSP（Ops Service Protocol）通信协议与服务端。
//
// 设计目标：控制台通过**自定义端口**向执行机下发命令/脚本任务，并实时掌握执行机存活状态。
// 与 SSH 不同，OSP 采用「Agent 主动外连、长连接待命」的反向通道模式——执行机无需开放任何入站端口，
// 即便 SSH 无法登录（密码过期、账户锁定、sshd 配置错误、sudoers 写坏），只要 Agent 进程存活，
// 控制台依旧能下发指令完成应急修复。
//
// 协议安全：
//   - 握手：ECDH P-256 密钥协商 + 服务端 RSA-PSS 签名（防中间人）+ 接入令牌（身份认证）
//   - 会话密钥：HKDF-SHA256(ecdhSecret, salt=nonceA||nonceB, info="boat-osp-v1|"+token) → 32 字节
//   - 传输：AES-256-GCM 加密每帧，帧头携带明文 seq 参与 AAD 校验，seq 严格递增防重放
//
// 帧格式（大端）：
//
//	握手前（明文帧）: [4B 长度][1B flag=0][JSON]
//	握手后（加密帧）: [4B 长度][1B flag=1][8B seq][12B nonce][密文+16B tag]
package osp

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
)

// ---------- 协议常量 ----------

const (
	// ProtoVersion 协议版本
	ProtoVersion = 1
	// MaxFrameSize 单帧最大字节数（8MB），防止恶意超大帧打爆内存
	MaxFrameSize = 8 << 20
	// NonceSize AES-GCM 标准 nonce 长度
	NonceSize = 12
	// SeqSize 帧内序号长度（明文携带，参与 AAD）
	SeqSize = 8
	// HeadSize 帧头长度：4B 长度 + 1B flag
	HeadSize = 5
)

// 帧类型标记
const (
	framePlain  byte = 0 // 明文帧（仅握手阶段使用）
	frameSealed byte = 1 // 加密帧（握手完成后所有消息）
)

// 消息类型
const (
	MsgHello      = "hello"       // agent → server：握手请求（明文，携带令牌与 ECDH 公钥）
	MsgWelcome    = "welcome"     // server → agent：握手响应（明文，携带 ECDH 公钥与 RSA 签名）
	MsgReject     = "reject"      // server → agent：拒绝接入（明文）
	MsgHeartbeat  = "heartbeat"   // agent → server：心跳 + 系统指标
	MsgTask       = "task"        // server → agent：下发任务
	MsgTaskResult = "task_result" // agent → server：任务执行结果
	MsgCancel     = "cancel"      // server → agent：取消任务
	MsgPing       = "ping"        // server → agent：探活
	MsgPong       = "pong"        // agent → server：探活响应
)

// 节点状态
const (
	StatusOnline  = "online"
	StatusOffline = "offline"
)

// 任务状态
const (
	TaskRunning  = "running"
	TaskSuccess  = "success"
	TaskPartial  = "partial"
	TaskFailed   = "failed"
	TaskCanceled = "canceled"
)

// 单节点结果状态
const (
	ResultPending   = "pending"
	ResultRunning   = "running"
	ResultSuccess   = "success"
	ResultFailed    = "failed"
	ResultTimeout   = "timeout"
	ResultOffline   = "offline"
	ResultCanceled  = "canceled"
	ResultUnmatched = "unmatched" // 令牌未匹配到节点
)

// 常见错误
var (
	ErrTimeout      = errors.New("等待 Agent 响应超时")
	ErrDisconnected = errors.New("Agent 连接已断开")
	ErrNodeDisabled = errors.New("节点已被禁用")
	ErrBadToken     = errors.New("接入令牌无效")
	ErrBadFrame     = errors.New("协议帧非法")
	ErrReplay       = errors.New("消息序号非法，疑似重放")
	ErrRejected     = errors.New("服务端拒绝接入")
)

// ---------- 消息信封 ----------

// Envelope 消息信封（加密通道内的 JSON 载体）
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewEnvelope 构造消息（payload 为 nil 时为空对象）
func NewEnvelope(msgType string, payload interface{}) (*Envelope, error) {
	env := &Envelope{Type: msgType, Payload: json.RawMessage("{}")}
	if payload == nil {
		return env, nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	env.Payload = b
	return env, nil
}

// Decode 将 payload 反序列化到 v
func (e *Envelope) Decode(v interface{}) error {
	if len(e.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(e.Payload, v)
}

// ---------- 握手与业务载荷 ----------

// HelloPayload 握手请求（agent → server，明文帧传输）
type HelloPayload struct {
	Version  string `json:"version"`  // agent 版本
	Token    string `json:"token"`    // 接入令牌（节点唯一凭证）
	NonceA   string `json:"nonceA"`   // base64，16 字节随机数
	ECDHPubA string `json:"ecdhPubA"` // base64，P-256 公钥（未压缩点）
	Hostname string `json:"hostname"` // 执行机主机名
	OS       string `json:"os"`       // linux / windows / darwin
	Arch     string `json:"arch"`     // amd64 / arm64 ...
	IP       string `json:"ip"`       // 执行机上报的主 IP
	Labels   string `json:"labels"`   // 标签（逗号分隔）
}

// WelcomePayload 握手响应（server → agent，明文帧传输）
type WelcomePayload struct {
	SessionID  string `json:"sessionId"`  // 会话 ID
	NodeID     uint   `json:"nodeId"`     // 节点 ID
	NodeName   string `json:"nodeName"`   // 节点名称
	NonceB     string `json:"nonceB"`     // base64，16 字节随机数
	ECDHPubB   string `json:"ecdhPubB"`   // base64，服务端 P-256 公钥
	Signature  string `json:"signature"`  // base64，RSA-PSS 对 (nonceA||nonceB||ecdhPubB) 的签名
	Heartbeat  int    `json:"heartbeat"`  // 心跳周期（秒）
	ServerTime int64  `json:"serverTime"` // 服务端 Unix 时间戳，便于 agent 校正
}

// RejectPayload 拒绝接入原因
type RejectPayload struct {
	Reason string `json:"reason"`
}

// Metrics 执行机实时指标（心跳上报）
type Metrics struct {
	CPUUsage  float64 `json:"cpuUsage"`  // CPU 使用率 %
	MemUsage  float64 `json:"memUsage"`  // 内存使用率 %
	DiskUsage float64 `json:"diskUsage"` // 根分区使用率 %
	LoadAvg   string  `json:"loadAvg"`   // 1/5/15 分钟负载
	Uptime    int64   `json:"uptime"`    // 已运行秒数
	MemTotal  uint64  `json:"memTotal"`  // 内存总量 MB
	MemUsed   uint64  `json:"memUsed"`   // 已用内存 MB
	DiskTotal uint64  `json:"diskTotal"` // 磁盘总量 GB
	DiskUsed  uint64  `json:"diskUsed"`  // 已用磁盘 GB
	AgentVer  string  `json:"agentVer"`  // agent 版本
}

// HeartbeatPayload 心跳（agent → server）
type HeartbeatPayload struct {
	Metrics Metrics `json:"metrics"`
}

// TaskPayload 下发任务（server → agent）
type TaskPayload struct {
	TaskID    uint   `json:"taskId"`
	ResultID  uint   `json:"resultId"`  // 单节点结果记录 ID，回传时原样带回
	Type      string `json:"type"`      // command | script
	Lang      string `json:"lang"`      // shell | python | powershell | batch
	Content   string `json:"content"`   // 命令或脚本正文
	Timeout   int    `json:"timeout"`   // 超时秒数
	RunAsUser string `json:"runAsUser"` // 指定执行用户（空表示使用 agent 自身运行用户）
	Env       string `json:"env"`       // 追加环境变量（KEY=VALUE，分号分隔）
	WorkDir   string `json:"workDir"`   // 工作目录（空表示 agent 默认临时目录）
}

// TaskResultPayload 任务结果（agent → server）
type TaskResultPayload struct {
	ResultID uint   `json:"resultId"`
	TaskID   uint   `json:"taskId"`
	Status   string `json:"status"`   // success | failed | timeout | canceled
	ExitCode int    `json:"exitCode"` // 进程退出码（-1 表示异常终止）
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Error    string `json:"error"`    // 执行失败原因（非业务错误）
	Duration int64  `json:"duration"` // 耗时毫秒
}

// CancelPayload 取消任务（server → agent）
type CancelPayload struct {
	ResultID uint `json:"resultId"`
}

// ---------- 帧读写 ----------

// WriteFrame 写一帧。sess 为 nil 时写明文帧（握手阶段），否则写加密帧。
func WriteFrame(w io.Writer, sess *Session, env *Envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		return err
	}
	var payload []byte
	flag := framePlain
	if sess != nil {
		seq, nonce, sealed, err := sess.Seal(body)
		if err != nil {
			return err
		}
		payload = make([]byte, 0, SeqSize+NonceSize+len(sealed))
		payload = binary.BigEndian.AppendUint64(payload, seq)
		payload = append(payload, nonce...)
		payload = append(payload, sealed...)
		flag = frameSealed
	} else {
		payload = body
	}
	if len(payload) > MaxFrameSize {
		return fmt.Errorf("%w: 帧体过大(%d)", ErrBadFrame, len(payload))
	}
	head := make([]byte, HeadSize)
	binary.BigEndian.PutUint32(head[0:4], uint32(len(payload)))
	head[4] = flag
	if _, err := w.Write(head); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

// ReadFrame 读一帧。sess 为 nil 时按明文帧解析，否则按加密帧校验并解密。
func ReadFrame(r io.Reader, sess *Session) (*Envelope, error) {
	head := make([]byte, HeadSize)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(head[0:4])
	flag := head[4]
	if length == 0 || int(length) > MaxFrameSize {
		return nil, fmt.Errorf("%w: 帧长度非法(%d)", ErrBadFrame, length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	var body []byte
	switch flag {
	case framePlain:
		if sess != nil {
			// 握手完成后不允许再出现明文帧，避免降级攻击
			return nil, fmt.Errorf("%w: 加密通道内出现明文帧", ErrBadFrame)
		}
		body = payload
	case frameSealed:
		if sess == nil {
			return nil, fmt.Errorf("%w: 未建立会话却收到加密帧", ErrBadFrame)
		}
		if len(payload) < SeqSize+NonceSize {
			return nil, fmt.Errorf("%w: 加密帧过短(%d)", ErrBadFrame, len(payload))
		}
		seq := binary.BigEndian.Uint64(payload[0:SeqSize])
		nonce := payload[SeqSize : SeqSize+NonceSize]
		sealed := payload[SeqSize+NonceSize:]
		out, err := sess.Open(seq, nonce, sealed)
		if err != nil {
			return nil, err
		}
		body = out
	default:
		return nil, fmt.Errorf("%w: 未知帧标记(%d)", ErrBadFrame, flag)
	}
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFrame, err)
	}
	if env.Type == "" {
		return nil, fmt.Errorf("%w: 缺少消息类型", ErrBadFrame)
	}
	return &env, nil
}

// ---------- 工具 ----------

// GenerateToken 生成节点接入令牌（32 字节随机 → base64url，带 osp_ 前缀）
func GenerateToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// 极端情况下退化为时间戳，实际不会触发
		return "osp_fallback"
	}
	return "osp_" + base64.RawURLEncoding.EncodeToString(buf)
}

// RandomNonce 生成握手随机数（16 字节 → base64）
func RandomNonce() ([]byte, string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return nil, "", err
	}
	return buf, base64.StdEncoding.EncodeToString(buf), nil
}

// LocalIP 取本机出口 IP（用于 agent 上报自身地址）
func LocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return ""
	}
	return addr.IP.String()
}
