package controller

import (
	"boat/internal/config"
	"boat/internal/database"
	"boat/internal/model"
	boatssh "boat/internal/ssh"
	"boat/internal/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type wsControl struct {
	Type  string `json:"type"`
	Cols  int    `json:"cols"`
	Rows  int    `json:"rows"`
	Data  string `json:"data"`
}

// TerminalWS Web Terminal WebSocket 端点
// 查询参数: hostId, cols, rows
func TerminalWS(c *gin.Context) {
	hostID := c.Query("hostId")
	if hostID == "" {
		utils.Fail(c, "缺少 hostId")
		return
	}
	// WebSocket 鉴权（令牌经 query 传递，浏览器 WS 无法自定义header）
	token := c.Query("token")
	claims, err := utils.ParseToken(token)
	if err != nil {
		utils.Unauthorized(c, "令牌无效或已过期")
		return
	}
	uid, username := claims.UserID, claims.Username
	var host model.Host
	if err := database.DB.First(&host, hostID).Error; err != nil {
		utils.Fail(c, "主机不存在")
		return
	}
	client, err := boatssh.ConnectToHost(host, uid)
	if err != nil {
		utils.Fail(c, "SSH 连接失败: "+err.Error())
		return
	}
	defer client.Close()

	// 会话记录
	session := model.Session{
		UserID: uid, Username: username, HostID: host.ID,
		HostName: host.Name, HostIP: host.IP, Protocol: "ssh",
		StartedAt: time.Now(), Status: 1,
	}
	database.DB.Create(&session)

	// 录像文件
	recPath := filepath.Join(config.Global.Record.Path, fmt.Sprintf("%d", host.ID), fmt.Sprintf("%d.cast", session.ID))
	_ = os.MkdirAll(filepath.Dir(recPath), 0o755)
	recFile, _ := os.OpenFile(recPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	var rec *castRecorder
	if recFile != nil {
		defer recFile.Close()
		session.RecordPath = recPath
		database.DB.Model(&session).Update("record_path", recPath)
	}

	ws, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	cols, rows := 80, 24
	fmt.Sscanf(c.Query("cols"), "%d", &cols)
	fmt.Sscanf(c.Query("rows"), "%d", &rows)
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	if recFile != nil {
		rec = newCastRecorder(recFile, cols, rows)
	}

	sshSession, err := client.NewSession()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("创建会话失败: "+err.Error()))
		endSession(&session)
		return
	}
	defer sshSession.Close()

	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := sshSession.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("请求 PTY 失败: "+err.Error()))
		endSession(&session)
		return
	}
	stdin, _ := sshSession.StdinPipe()
	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("获取输出流失败: "+err.Error()))
		endSession(&session)
		return
	}
	// Stderr 未设置时，ssh 会话默认将标准错误合并到标准输出

	// 远端输出 -> ws + 录像
	outWriter := func(p []byte) {
		ws.WriteMessage(websocket.BinaryMessage, p)
		if rec != nil {
			rec.Write(p)
		}
	}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				outWriter(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	if err := sshSession.Shell(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("启动 Shell 失败: "+err.Error()))
		endSession(&session)
		return
	}

	// 提权切换 root：支持禁止 root 直登的主机（以普通用户登录后再 sudo -i）
	// 用 sudo -S -p '' 从 stdin 读取密码，避免交互阻塞；密码单独写入不回显，不会泄露到屏幕
	if host.BecomeRoot {
		if pw, lu, e := boatssh.SudoPasswordForHost(host, uid); e == nil && lu != "root" {
			if pw != "" {
				stdin.Write([]byte("sudo -S -p '' -i\r\n"))
				stdin.Write([]byte(pw + "\r\n"))
			} else {
				// 无密码（密钥 / NOPASSWD）：直接 sudo -i，依赖远端不提示密码
				stdin.Write([]byte("sudo -i\r\n"))
			}
		}
	}

	// 行缓冲 + 高危指令拦截
	var line []byte
	writeLocal := func(b []byte) { ws.WriteMessage(websocket.BinaryMessage, b) }

	// 主循环：读取前端输入
	for {
		mt, data, err := ws.ReadMessage()
		if err != nil {
			break
		}
		// 控制消息（JSON）
		if mt == websocket.TextMessage && len(data) > 0 && data[0] == '{' {
			var ctrl wsControl
			if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "resize" {
				_ = sshSession.WindowChange(ctrl.Rows, ctrl.Cols)
				continue
			}
		}
		// 原始按键流：逐字节处理（支持本地回显与拦截）
		for _, b := range data {
			switch {
			case b == '\r' || b == '\n':
				cmd := string(line)
				writeLocal([]byte("\r\n"))
				hit := CheckCommand(strings.TrimSpace(cmd))
				if uid != 1 && hit.Blocked && hit.Rule != nil {
					msg := fmt.Sprintf("\r\n\x1b[31m⛔ 高危指令已被拦截（规则: %s）\x1b[0m\r\n", hit.Rule.Name)
					writeLocal([]byte(msg))
					line = line[:0]
					continue
				}
				stdin.Write(line)
				stdin.Write([]byte{'\n'})
				if rec != nil {
					rec.Write(line)
					rec.Write([]byte{'\n'})
				}
				line = line[:0]
			case b == 0x7f || b == 0x08: // 退格
				if len(line) > 0 {
					line = line[:len(line)-1]
					writeLocal([]byte{'\b', ' ', '\b'})
				}
			case b == 0x03: // Ctrl-C
				stdin.Write([]byte{b})
				line = line[:0]
			case b >= 0x20 && b != 0x7f:
				line = append(line, b)
				writeLocal([]byte{b})
			default:
				// 忽略其它控制字符
			}
		}
	}
	sshSession.Close()
	endSession(&session)
}

func endSession(s *model.Session) {
	now := time.Now()
	database.DB.Model(&model.Session{}).Where("id = ?", s.ID).
		Updates(map[string]interface{}{"status": 0, "ended_at": &now})
}

// castRecorder 以 asciinema v2 格式落盘会话录像，便于前端在线回放
// 头部: {"version":2,"width":W,"height":H,"timestamp":...}
// 事件: [<延迟秒>, "o", "<JSON转义后的字节>"]
type castRecorder struct {
	mu      sync.Mutex
	file    *os.File
	last    time.Time
	started bool
}

func newCastRecorder(f *os.File, cols, rows int) *castRecorder {
	r := &castRecorder{file: f, last: time.Now()}
	header := fmt.Sprintf("{\"version\":2,\"width\":%d,\"height\":%d,\"timestamp\":%d,\"title\":\"boat-session\"}\n",
		cols, rows, time.Now().Unix())
	f.Write([]byte(header))
	return r
}

// Write 写入一帧终端输出，自动计算距上一帧的延迟并转义
func (r *castRecorder) Write(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	delay := 0.0
	if r.started {
		delay = now.Sub(r.last).Seconds()
		if delay < 0 {
			delay = 0
		}
	}
	r.started = true
	r.last = now
	data, _ := json.Marshal(string(p))
	line := fmt.Sprintf("[%.6f,\"o\",%s]\n", delay, data)
	r.file.Write([]byte(line))
}
