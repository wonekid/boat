package utils

import (
	"crypto"
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

// SignData 使用平台 RSA 私钥签名（RSA-PSS + SHA256），返回 base64。
// 用于 OSP Agent 握手时服务端对密钥协商数据签名，防止中间人攻击。
func SignData(data []byte) (string, error) {
	if rsaPrivateKey == nil {
		return "", errors.New("RSA 未初始化")
	}
	hashed := sha256.Sum256(data)
	sig, err := rsa.SignPSS(rand.Reader, rsaPrivateKey, crypto.SHA256, hashed[:], nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifySignature 使用平台 RSA 公钥验签（与 SignData 配对）
func VerifySignature(data []byte, sigB64 string) error {
	if rsaPrivateKey == nil {
		return errors.New("RSA 未初始化")
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return errors.New("签名格式非法")
	}
	hashed := sha256.Sum256(data)
	return rsa.VerifyPSS(&rsaPrivateKey.PublicKey, crypto.SHA256, hashed[:], sig, nil)
}

// PublicKeyPEM 导出服务端 RSA 公钥 PEM（下发到 Agent 侧，用于校验握手签名）
func PublicKeyPEM() (string, error) {
	if rsaPrivateKey == nil {
		return "", errors.New("RSA 未初始化")
	}
	der, err := x509.MarshalPKIXPublicKey(&rsaPrivateKey.PublicKey)
	if err != nil {
		return "", err
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	return string(pem.EncodeToMemory(block)), nil
}

// ParsePrivateKey 解析 PEM 私钥为 ssh.Signer（用于密钥登录）
func ParsePrivateKey(pemData string) (ssh.Signer, error) {
	return ssh.ParsePrivateKey([]byte(pemData))
}
