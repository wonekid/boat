package bastion

import (
	"boat/internal/config"
	"boat/internal/database"
	"boat/internal/model"
	boatssh "boat/internal/ssh"
	"boat/internal/utils"
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

// Start 启动独立 SSH 堡垒机（默认端口 2222）
// 连接后可从授权主机列表中选择目标，建立带 PTY 的反向代理会话。
func Start() error {
	port := config.Global.Bastion.Port
	if port == 0 {
		port = 2222
	}
	keyPath := config.Global.Bastion.HostKeyPath
	if keyPath == "" {
		keyPath = "configs/ssh_host_key"
	}
	ensureHostKey(keyPath)

	server := &ssh.Server{
		Addr: fmt.Sprintf(":%d", port),
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return authUser(ctx.Value(ssh.ContextKeyUser).(string), password)
		},
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			// 密钥登录暂未接入凭证库，后续可扩展
			return false
		},
		Handler: proxyHandler,
	}
	if err := server.SetOption(ssh.HostKeyFile(keyPath)); err != nil {
		return fmt.Errorf("设置主机密钥失败: %w", err)
	}
	log.Printf("[bastion] SSH 堡垒机监听于 :%d", port)
	return server.ListenAndServe()
}

// ensureHostKey 生成 RSA 主机密钥（若不存在）
func ensureHostKey(path string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, pem.EncodeToMemory(block), 0o600)
}

// authUser 校验用户库密码，并写审计
func authUser(username, password string) bool {
	var u model.User
	if err := database.DB.Where("username = ?", username).First(&u).Error; err != nil {
		return false
	}
	if u.Status == 0 {
		return false
	}
	ok := utils.PasswordVerify(password, u.Password)
	if ok {
		now := time.Now()
		database.DB.Model(&u).Update("last_login_at", &now)
		writeAudit(u.ID, u.Username, "堡垒机登录成功", "堡垒机", "ssh", 1, "")
	} else {
		writeAudit(u.ID, u.Username, "堡垒机登录失败", "堡垒机", "ssh", 0, "密码错误")
	}
	return ok
}

// writeAudit 堡垒机审计日志
func writeAudit(userID uint, username, action, module, ip string, status int, detail string) {
	log := model.AuditLog{
		UserID:  userID,
		Username: username,
		Action:  action,
		Module:  module,
		IP:      ip,
		Status:  status,
		Detail:  detail,
	}
	database.DB.Create(&log)
}

// allowedHosts 取用户可用主机（管理员见全部，其余按授权）
func allowedHosts(username string) []model.Host {
	var u model.User
	if err := database.DB.Where("username = ?", username).First(&u).Error; err != nil {
		return nil
	}
	if u.ID == 1 {
		var all []model.Host
		database.DB.Find(&all)
		return all
	}
	ids := map[uint]bool{}
	var auths []model.Authorization
	database.DB.Where("user_id = ?", u.ID).Find(&auths)
	for _, a := range auths {
		if a.TargetType == "host" {
			ids[a.TargetID] = true
		}
		if a.TargetType == "hostGroup" {
			var hs []model.Host
			database.DB.Where("group_id = ?", a.TargetID).Find(&hs)
			for _, h := range hs {
				ids[h.ID] = true
			}
		}
	}
	var hosts []model.Host
	for id := range ids {
		var h model.Host
		if database.DB.First(&h, id).Error == nil {
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// proxyHandler 堡垒机会话：展示菜单 -> 选择主机 -> 反向代理
func proxyHandler(s ssh.Session) {
	user := s.Context().Value(ssh.ContextKeyUser).(string)
	var u model.User
	if err := database.DB.Where("username = ?", user).First(&u).Error; err != nil {
		s.Write([]byte("\r\n\x1b[31m用户不存在\x1b[0m\r\n"))
		return
	}
	hosts := allowedHosts(user)
	if len(hosts) == 0 {
		s.Write([]byte("\r\n无可用主机授权\r\n"))
		return
	}

	var menu strings.Builder
	menu.WriteString("\r\n\x1b[36m=== Boat 堡垒机 ===\x1b[0m\r\n可用主机:\r\n")
	for i, h := range hosts {
		fmt.Fprintf(&menu, "  \x1b[33m%d\x1b[0m) %s  \x1b[90m(%s:%d)\x1b[0m\r\n", i+1, h.Name, h.IP, h.Port)
	}
	menu.WriteString("\r\n请选择主机编号: ")
	s.Write([]byte(menu.String()))

	reader := bufio.NewReader(s)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return
	}
	choice := strings.TrimSpace(line)
	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(hosts) {
		s.Write([]byte("\r\n\x1b[31m无效选择\x1b[0m\r\n"))
		return
	}
	target := hosts[idx-1]
	s.Write([]byte(fmt.Sprintf("\r\n\x1b[36m-> 正在连接 %s ...\x1b[0m\r\n", target.Name)))

	client, err := boatssh.ConnectToHost(target, u.ID)
	if err != nil {
		s.Write([]byte(fmt.Sprintf("\r\n\x1b[31m连接失败: %s\x1b[0m\r\n", err.Error())))
		return
	}
	defer client.Close()

	term := "xterm"
	width, height := 80, 24
	pty, winCh, hasPty := s.Pty()
	if hasPty {
		term = pty.Term
		width, height = pty.Window.Width, pty.Window.Height
	}

	tsess, err := client.NewSession()
	if err != nil {
		s.Write([]byte(fmt.Sprintf("\r\n\x1b[31m创建会话失败: %s\x1b[0m\r\n", err.Error())))
		return
	}
	defer tsess.Close()

	modes := cryptossh.TerminalModes{
		cryptossh.ECHO:          1,
		cryptossh.TTY_OP_ISPEED: 14400,
		cryptossh.TTY_OP_OSPEED: 14400,
	}
	if err := tsess.RequestPty(term, height, width, modes); err != nil {
		s.Write([]byte(fmt.Sprintf("\r\n\x1b[31m请求 PTY 失败: %s\x1b[0m\r\n", err.Error())))
		return
	}
	stdin, _ := tsess.StdinPipe()
	stdout, _ := tsess.StdoutPipe()
	tsess.Stderr = s.Stderr()

	if hasPty {
		go func() {
			for win := range winCh {
				_ = tsess.WindowChange(win.Height, win.Width)
			}
		}()
	}

	if err := tsess.Shell(); err != nil {
		s.Write([]byte(fmt.Sprintf("\r\n\x1b[31m启动 Shell 失败: %s\x1b[0m\r\n", err.Error())))
		return
	}

	// 双向桥接：客户端输入 -> 目标；目标输出 -> 客户端
	go io.Copy(stdin, reader)
	io.Copy(s, stdout)
	tsess.Wait()
}
