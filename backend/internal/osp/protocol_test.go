package osp

import (
	"encoding/base64"
	"net"
	"testing"
)

// TestKeyExchangeDerive 验证两端派生出的会话密钥一致（模拟完整 ECDH 握手）
func TestKeyExchangeDerive(t *testing.T) {
	client, err := NewKeyExchange()
	if err != nil {
		t.Fatalf("客户端密钥交换失败: %v", err)
	}
	server, err := NewKeyExchange()
	if err != nil {
		t.Fatalf("服务端密钥交换失败: %v", err)
	}
	nonceA := []byte("0123456789abcdef")
	nonceB := []byte("fedcba9876543210")
	token := "osp_test_token"

	cs, err := client.Derive(server.Pub, nonceA, nonceB, token)
	if err != nil {
		t.Fatalf("客户端派生失败: %v", err)
	}
	ss, err := server.Derive(client.Pub, nonceA, nonceB, token)
	if err != nil {
		t.Fatalf("服务端派生失败: %v", err)
	}

	// 双方用各自密钥互发消息，能解开即证明密钥一致
	_, nonce, sealed, err := cs.Seal([]byte(`{"hello":"agent"}`))
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	body, err := ss.Open(1, nonce, sealed)
	if err != nil {
		t.Fatalf("服务端解密失败（密钥不一致）: %v", err)
	}
	if string(body) != `{"hello":"agent"}` {
		t.Fatalf("解密内容不符: %s", body)
	}

	// 反向
	_, nonce2, sealed2, err := ss.Seal([]byte(`{"type":"task"}`))
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	body2, err := cs.Open(1, nonce2, sealed2)
	if err != nil {
		t.Fatalf("客户端解密失败（密钥不一致）: %v", err)
	}
	if string(body2) != `{"type":"task"}` {
		t.Fatalf("解密内容不符: %s", body2)
	}
}

// TestTokenIsolatedSession 验证不同令牌派生出的会话互不可解
func TestTokenIsolatedSession(t *testing.T) {
	c, _ := NewKeyExchange()
	s, _ := NewKeyExchange()
	nonce := []byte("0123456789abcdef")
	good, _ := c.Derive(s.Pub, nonce, nonce, "osp_token_a")
	bad, _ := c.Derive(s.Pub, nonce, nonce, "osp_token_b")

	_, nonceOut, sealed, _ := good.Seal([]byte("secret"))
	if _, err := bad.Open(1, nonceOut, sealed); err == nil {
		t.Fatal("不同令牌的会话不应能解密（令牌未参与密钥派生？）")
	}
}

// TestSessionReplay 验证序号回退/重复会被判定为重放
func TestSessionReplay(t *testing.T) {
	c, _ := NewKeyExchange()
	s, _ := NewKeyExchange()
	nonce := []byte("0123456789abcdef")
	sender, _ := c.Derive(s.Pub, nonce, nonce, "t")
	receiver, _ := s.Derive(c.Pub, nonce, nonce, "t")

	_, n1, s1, _ := sender.Seal([]byte("msg-1"))
	if _, err := receiver.Open(1, n1, s1); err != nil {
		t.Fatalf("首帧解密失败: %v", err)
	}
	// 重放第一帧
	if _, err := receiver.Open(1, n1, s1); err == nil {
		t.Fatal("重复序号未被拦截")
	}
	// 序号回退
	_, n2, s2, _ := sender.Seal([]byte("msg-2"))
	if _, err := receiver.Open(0, n2, s2); err == nil {
		t.Fatal("回退序号未被拦截")
	}
	// 正常递增应当通过
	if _, err := receiver.Open(2, n2, s2); err != nil {
		t.Fatalf("递增序号被误判: %v", err)
	}
}

// TestFrameRoundTrip 验证明文帧与加密帧在线路上的读写一致性
func TestFrameRoundTrip(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// 握手：明文帧
	hello := &HelloPayload{Version: "1.0.0", Token: "osp_abc",
		NonceA: base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")),
		ECDHPubA: base64.StdEncoding.EncodeToString(make([]byte, 65))}
	env, err := NewEnvelope(MsgHello, hello)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := WriteFrame(clientConn, nil, env); err != nil {
			t.Errorf("写明文帧失败: %v", err)
		}
	}()
	got, err := ReadFrame(serverConn, nil)
	if err != nil {
		t.Fatalf("读明文帧失败: %v", err)
	}
	if got.Type != MsgHello {
		t.Fatalf("消息类型错误: %s", got.Type)
	}
	var decoded HelloPayload
	if err := got.Decode(&decoded); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if decoded.Token != "osp_abc" || decoded.Version != "1.0.0" {
		t.Fatalf("明文帧内容不符: %+v", decoded)
	}

	// 加密帧
	c, _ := NewKeyExchange()
	s, _ := NewKeyExchange()
	nonce := []byte("0123456789abcdef")
	cs, _ := c.Derive(s.Pub, nonce, nonce, "osp_abc")
	ss, _ := s.Derive(c.Pub, nonce, nonce, "osp_abc")

	task := &TaskPayload{TaskID: 9, ResultID: 3, Type: "command", Lang: "shell", Content: "id", Timeout: 30}
	taskEnv, _ := NewEnvelope(MsgTask, task)
	go func() {
		if err := WriteFrame(clientConn, cSession(cs), taskEnv); err != nil {
			t.Errorf("写加密帧失败: %v", err)
		}
	}()
	enc, err := ReadFrame(serverConn, ss)
	if err != nil {
		t.Fatalf("读加密帧失败: %v", err)
	}
	if enc.Type != MsgTask {
		t.Fatalf("加密帧类型错误: %s", enc.Type)
	}
	var gotTask TaskPayload
	if err := enc.Decode(&gotTask); err != nil {
		t.Fatalf("加密帧解码失败: %v", err)
	}
	if gotTask.Content != "id" || gotTask.ResultID != 3 {
		t.Fatalf("加密帧内容不符: %+v", gotTask)
	}
}

// cSession 测试辅助：保证并发安全取会话
func cSession(s *Session) *Session { return s }

// TestFrameRejectsPlainAfterHandshake 验证握手完成后拒绝明文帧（防降级）
func TestFrameRejectsPlainAfterHandshake(t *testing.T) {
	c, _ := NewKeyExchange()
	s, _ := NewKeyExchange()
	nonce := []byte("0123456789abcdef")
	cs, _ := c.Derive(s.Pub, nonce, nonce, "t")
	ss, _ := s.Derive(c.Pub, nonce, nonce, "t")

	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	plain, _ := NewEnvelope(MsgPing, nil)
	go func() { _ = WriteFrame(a, nil, plain) }()
	if _, err := ReadFrame(b, ss); err == nil {
		t.Fatal("加密通道内的明文帧未被拒绝")
	}
	_ = cs
}

// TestGenerateToken 验证令牌唯一性与前缀
func TestGenerateToken(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		tk := GenerateToken()
		if len(tk) < 8 || tk[:4] != "osp_" {
			t.Fatalf("令牌格式异常: %s", tk)
		}
		if _, ok := seen[tk]; ok {
			t.Fatalf("令牌重复: %s", tk)
		}
		seen[tk] = struct{}{}
	}
}
