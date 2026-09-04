package osp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/hkdf"
)

// KeyExchange ECDH 密钥协商上下文（每次连接独立生成，具备前向安全性）
type KeyExchange struct {
	priv *ecdh.PrivateKey
	// Pub 本端公钥（未压缩点编码），需发送给对端
	Pub []byte
}

// NewKeyExchange 生成一次性 ECDH P-256 密钥对
func NewKeyExchange() (*KeyExchange, error) {
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &KeyExchange{priv: priv, Pub: priv.PublicKey().Bytes()}, nil
}

// Derive 根据对端公钥、双方随机数与接入令牌派生会话密钥。
// 即使令牌泄露，攻击者没有 ECDH 私钥也无法解密既有会话流量。
func (k *KeyExchange) Derive(peerPub, nonceA, nonceB []byte, token string) (*Session, error) {
	pub, err := ecdh.P256().NewPublicKey(peerPub)
	if err != nil {
		return nil, fmt.Errorf("对端 ECDH 公钥非法: %w", err)
	}
	secret, err := k.priv.ECDH(pub)
	if err != nil {
		return nil, fmt.Errorf("ECDH 协商失败: %w", err)
	}
	salt := make([]byte, 0, len(nonceA)+len(nonceB))
	salt = append(salt, nonceA...)
	salt = append(salt, nonceB...)
	info := []byte("boat-osp-v1|" + token)
	rd := hkdf.New(sha256.New, secret, salt, info)
	key := make([]byte, 32)
	if _, err := io.ReadFull(rd, key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Session{gcm: gcm}, nil
}

// Session 会话加密上下文：AES-256-GCM，逐帧随机 nonce，序号防重放
type Session struct {
	gcm cipher.AEAD

	mu      sync.Mutex
	sendSeq uint64
	recvSeq uint64
}

// aadFor 构造附加认证数据：协议版本 + 帧序号（序号为明文，解密前即可校验）
func aadFor(seq uint64) []byte {
	aad := make([]byte, 0, 9)
	aad = append(aad, byte(ProtoVersion))
	return binary.BigEndian.AppendUint64(aad, seq)
}

// Seal 加密消息体，返回帧序号、随机 nonce 与密文（含 GCM tag）
func (s *Session) Seal(body []byte) (seq uint64, nonce, sealed []byte, err error) {
	s.mu.Lock()
	s.sendSeq++
	seq = s.sendSeq
	s.mu.Unlock()

	nonce = make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return 0, nil, nil, err
	}
	sealed = s.gcm.Seal(nil, nonce, body, aadFor(seq))
	return seq, nonce, sealed, nil
}

// Open 校验并解密消息体。序号必须严格递增，重复或回退视为重放攻击。
func (s *Session) Open(seq uint64, nonce, sealed []byte) ([]byte, error) {
	s.mu.Lock()
	if seq <= s.recvSeq {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: 收到 %d，已处理至 %d", ErrReplay, seq, s.recvSeq)
	}
	s.recvSeq = seq
	s.mu.Unlock()

	body, err := s.gcm.Open(nil, nonce, sealed, aadFor(seq))
	if err != nil {
		return nil, fmt.Errorf("解密失败（密钥不匹配或数据被篡改）: %w", err)
	}
	return body, nil
}

// RecvSeq 已接收的最大序号（诊断用）
func (s *Session) RecvSeq() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recvSeq
}

// SendSeq 已发送的最大序号（诊断用）
func (s *Session) SendSeq() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sendSeq
}
