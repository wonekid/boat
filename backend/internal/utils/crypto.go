package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"

	"golang.org/x/crypto/ssh"
)

// ---- 密码哈希 (bcrypt 由 golang.org/x/crypto/bcrypt 替代) ----
// 使用简单 SHA256+盐，生产环境建议替换为 bcrypt。这里用 RSA 做凭证加密演示。

var rsaPrivateKey *rsa.PrivateKey

// InitRSA 加载或生成 RSA 密钥用于凭证加密
func InitRSA(keyPath string) error {
	// 读取已有私钥
	if data, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err == nil {
				rsaPrivateKey = key
				return nil
			}
		}
	}
	// 生成新密钥
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	rsaPrivateKey = key
	// 写出私钥
	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	if err := os.MkdirAll(filepathDir(keyPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(keyPath, pem.EncodeToMemory(block), 0o600)
}

func filepathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

// EncryptSecret RSA 加密敏感字段（密码/私钥），返回 base64
func EncryptSecret(plain string) (string, error) {
	if rsaPrivateKey == nil {
		return "", errors.New("RSA 未初始化")
	}
	pub := &rsaPrivateKey.PublicKey
	hashed := sha256.New()
	out, err := rsa.EncryptOAEP(hashed, rand.Reader, pub, []byte(plain), nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptSecret RSA 解密
func DecryptSecret(cipher string) (string, error) {
	if rsaPrivateKey == nil {
		return "", errors.New("RSA 未初始化")
	}
	data, err := base64.StdEncoding.DecodeString(cipher)
	if err != nil {
		return "", err
	}
	hashed := sha256.New()
	out, err := rsa.DecryptOAEP(hashed, rand.Reader, rsaPrivateKey, data, nil)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// PasswordHash 简单密码哈希（演示用）
func PasswordHash(pwd string) string {
	h := sha256.Sum256([]byte("boat::" + pwd))
	return base64.StdEncoding.EncodeToString(h[:])
}

func PasswordVerify(pwd, hash string) bool {
	return PasswordHash(pwd) == hash
}

// ParsePrivateKey 解析 PEM 私钥为 ssh.Signer（用于密钥登录）
func ParsePrivateKey(pemData string) (ssh.Signer, error) {
	return ssh.ParsePrivateKey([]byte(pemData))
}
