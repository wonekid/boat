package ssh

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// AuthBuild 构建 SSH 认证方式
func buildAuth(cred model.Credential) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if cred.Type == 1 && cred.AuthPassword != "" {
		pwd, err := utils.DecryptSecret(cred.AuthPassword)
		if err != nil {
			return nil, fmt.Errorf("解密密码失败: %w", err)
		}
		methods = append(methods, ssh.Password(pwd))
	}
	if cred.Type == 2 && cred.PrivateKey != "" {
		key, err := utils.DecryptSecret(cred.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("解密私钥失败: %w", err)
		}
		signer, err := utils.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("凭证无可用认证方式")
	}
	return methods, nil
}

// credentialUsable 凭证是否具备可执行的认证材料
func credentialUsable(c model.Credential) bool {
	switch c.Type {
	case 1:
		return c.AuthPassword != ""
	case 2:
		return c.PrivateKey != ""
	}
	return false
}

// resolveCredential 按优先级解析可用于连接主机的凭证：
//  1. 主机直接绑定的凭证 (host.credential_id)
//  2. 执行用户被「授权管理」授予的凭证
//  3. 超级管理员(uid=1)回退任意可用凭证
func resolveCredential(host model.Host, uid uint) (model.Credential, error) {
	if host.CredentialID > 0 {
		var c model.Credential
		if err := database.DB.First(&c, host.CredentialID).Error; err == nil && credentialUsable(c) {
			return c, nil
		}
	}
	var auths []model.Authorization
	database.DB.Where("user_id = ? AND target_type = ?", uid, "credential").Find(&auths)
	for _, a := range auths {
		var c model.Credential
		if err := database.DB.First(&c, a.TargetID).Error; err == nil && credentialUsable(c) {
			return c, nil
		}
	}
	if uid == 1 {
		var c model.Credential
		if err := database.DB.Where("type IN (1,2)").First(&c).Error; err == nil && credentialUsable(c) {
			return c, nil
		}
	}
	return model.Credential{}, fmt.Errorf("主机 %s 未找到可用凭证：请在「主机管理」绑定凭证，或在「授权管理」为该用户分配凭证", host.IP)
}

// ConnectToHost 按主机配置建立 SSH 连接（uid 为执行用户，用于授权回退）
func ConnectToHost(host model.Host, uid uint) (*ssh.Client, error) {
	cred, err := resolveCredential(host, uid)
	if err != nil {
		return nil, err
	}
	user := host.User
	if user == "" {
		user = cred.Username
	}
	if user == "" {
		user = "root"
	}
	auth, err := buildAuth(cred)
	if err != nil {
		return nil, err
	}
	port := host.Port
	if port == 0 {
		port = 22
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 演示：忽略主机密钥校验
		Timeout:         8 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", host.IP, port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("连接 %s 失败: %w", addr, err)
	}
	return client, nil
}

// RunCommand 在远程主机执行单条命令，返回标准输出+错误输出
func RunCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

// shellQuote 将命令用单引号包裹，处理内部单引号（用于 /bin/sh -c 安全传参）
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// SudoPasswordForHost 解析用于提权的登录用户与密码（供 Web Terminal 交互式提权使用）
func SudoPasswordForHost(host model.Host, uid uint) (password string, loginUser string, err error) {
	cred, e := resolveCredential(host, uid)
	if e != nil {
		return "", "", e
	}
	loginUser = host.User
	if loginUser == "" {
		loginUser = cred.Username
	}
	if loginUser == "" {
		loginUser = "root"
	}
	if cred.Type == 1 && cred.AuthPassword != "" {
		password, _ = utils.DecryptSecret(cred.AuthPassword)
	}
	return password, loginUser, nil
}

// RunCommandElevated 以 root 身份执行命令（sudo -S 从 stdin 读取密码，避免交互卡顿）
func RunCommandElevated(client *ssh.Client, cred model.Credential, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	pw := ""
	if cred.Type == 1 && cred.AuthPassword != "" {
		pw, _ = utils.DecryptSecret(cred.AuthPassword)
	}
	wrapped := "sudo -S -p '' -- /bin/sh -c " + shellQuote(cmd)
	session.Stdin = strings.NewReader(pw + "\n")
	out, err := session.CombinedOutput(wrapped)
	return string(out), err
}

// ExecCmd 在远程主机执行命令；若主机开启了 BecomeRoot 则自动提权到 root
func ExecCmd(client *ssh.Client, host model.Host, uid uint, cmd string) (string, error) {
	if host.BecomeRoot {
		cred, err := resolveCredential(host, uid)
		if err != nil {
			return "", err
		}
		return RunCommandElevated(client, cred, cmd)
	}
	return RunCommand(client, cmd)
}
