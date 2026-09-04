package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
)

// verifySignature 校验服务端握手签名：RSA-PSS + SHA256（防中间人）
func verifySignature(pubKeyPEM, data []byte, sigB64 string) error {
	block, _ := pem.Decode(pubKeyPEM)
	if block == nil {
		return errors.New("服务端公钥 PEM 解析失败")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return errors.New("服务端公钥不是 RSA 公钥")
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return errors.New("服务端签名格式非法")
	}
	hashed := sha256.Sum256(data)
	return rsa.VerifyPSS(rsaPub, crypto.SHA256, hashed[:], sig, nil)
}
