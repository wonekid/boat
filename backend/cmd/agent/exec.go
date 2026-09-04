package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"boat/internal/osp"
)

// Executor 任务执行器
type Executor struct {
	cfg     *Config
	mu      sync.Mutex
	cancels map[uint]context.CancelFunc
}

func newExecutor(cfg *Config) *Executor {
	return &Executor{cfg: cfg, cancels: make(map[uint]context.CancelFunc)}
}

// Cancel 取消正在执行的任务
func (e *Executor) Cancel(resultID uint) {
	e.mu.Lock()
	cancel, ok := e.cancels[resultID]
	e.mu.Unlock()
	if ok && cancel != nil {
		logf("[exec] 收到取消指令 resultId=%d", resultID)
		cancel()
	}
}

// Run 执行任务（命令或脚本），返回结果
func (e *Executor) Run(t *osp.TaskPayload) *osp.TaskResultPayload {
	start := time.Now()
	res := &osp.TaskResultPayload{
		ResultID: t.ResultID,
		TaskID:   t.TaskID,
		Status:   osp.ResultFailed,
		ExitCode: -1,
	}
	timeout := time.Duration(t.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	e.mu.Lock()
	e.cancels[t.ResultID] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.cancels, t.ResultID)
		e.mu.Unlock()
	}()

	cmd, cleanup, err := buildCommand(ctx, e.cfg, t)
	if err != nil {
		res.Error = err.Error()
		res.Duration = time.Since(start).Milliseconds()
		return res
	}
	if cleanup != nil {
		defer cleanup()
	}

	var stdout, stderr limitedWriter
	stdout.limit, stderr.limit = outputLimit, outputLimit
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()
	res.Duration = time.Since(start).Milliseconds()

	switch {
	case ctx.Err() == context.DeadlineExceeded:
		res.Status = osp.ResultTimeout
		res.Error = fmt.Sprintf("执行超时（%d 秒）", t.Timeout)
	case errors.Is(ctx.Err(), context.Canceled):
		res.Status = osp.ResultCanceled
		res.Error = "任务已被控制台取消"
	case runErr != nil:
		res.Status = osp.ResultFailed
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
			if res.ExitCode == 0 {
				res.ExitCode = -1
			}
			res.Error = fmt.Sprintf("进程退出码 %d", res.ExitCode)
		} else {
			res.Error = runErr.Error()
		}
	default:
		res.Status = osp.ResultSuccess
		res.ExitCode = 0
	}
	return res
}

// outputLimit 单段输出最大保留字节数（防止超长输出撑爆内存与数据库）
const outputLimit = 512 * 1024

// limitedWriter 限量写入器：超出上限后静默丢弃
type limitedWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	remain := w.limit - w.buf.Len()
	if remain <= 0 {
		return len(p), nil
	}
	if len(p) > remain {
		w.buf.Write(p[:remain])
		return len(p), nil
	}
	w.buf.Write(p)
	return len(p), nil
}

func (w *limitedWriter) String() string { return w.buf.String() }

// buildCommand 根据任务类型与语言构造可执行文件；返回的 cleanup 用于清理落盘脚本
func buildCommand(ctx context.Context, cfg *Config, t *osp.TaskPayload) (*exec.Cmd, func(), error) {
	lang := strings.ToLower(t.Lang)
	if lang == "" {
		lang = "shell"
	}
	workDir := t.WorkDir
	if workDir == "" {
		workDir = cfg.WorkDir
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("工作目录不可用: %w", err)
	}

	var (
		name    string
		args    []string
		cleanup func()
	)
	isWindows := runtime.GOOS == "windows"

	switch lang {
	case "python", "python3", "py":
		script := filepath.Join(workDir, fmt.Sprintf("osp_task_%d.py", t.ResultID))
		if err := os.WriteFile(script, []byte(t.Content), 0o700); err != nil {
			return nil, nil, err
		}
		cleanup = func() { _ = os.Remove(script) }
		name = "python3"
		if _, err := exec.LookPath(name); err != nil {
			if _, err2 := exec.LookPath("python"); err2 == nil {
				name = "python"
			}
		}
		args = []string{script}
	case "powershell", "pwsh", "ps1":
		script := filepath.Join(workDir, fmt.Sprintf("osp_task_%d.ps1", t.ResultID))
		if err := os.WriteFile(script, []byte(t.Content), 0o700); err != nil {
			return nil, nil, err
		}
		cleanup = func() { _ = os.Remove(script) }
		name = "powershell"
		if !isWindows {
			if _, err := exec.LookPath("pwsh"); err == nil {
				name = "pwsh"
			}
			args = []string{"-NoProfile", "-File", script}
		} else {
			args = []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script}
		}
	case "batch", "bat", "cmd":
		script := filepath.Join(workDir, fmt.Sprintf("osp_task_%d.bat", t.ResultID))
		if err := os.WriteFile(script, []byte(t.Content), 0o700); err != nil {
			return nil, nil, err
		}
		cleanup = func() { _ = os.Remove(script) }
		if isWindows {
			name, args = "cmd", []string{"/C", script}
		} else {
			name, args = shellPath(), []string{script}
		}
	default: // shell / bash / sh
		shell := shellPath()
		if t.Type == "script" {
			// 脚本类任务落盘执行，避免超长命令行与转义问题
			ext := ".sh"
			if isWindows {
				ext = ".ps1"
			}
			script := filepath.Join(workDir, fmt.Sprintf("osp_task_%d%s", t.ResultID, ext))
			if err := os.WriteFile(script, []byte(t.Content), 0o700); err != nil {
				return nil, nil, err
			}
			cleanup = func() { _ = os.Remove(script) }
			if isWindows {
				name, args = "powershell", []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script}
			} else {
				name, args = shell, []string{script}
			}
		} else {
			if isWindows {
				name, args = "powershell", []string{"-NoProfile", "-Command", t.Content}
			} else {
				name, args = shell, []string{"-c", t.Content}
			}
		}
	}

	// 指定执行用户（Linux：runuser 优先，su 兜底；Windows：暂不支持，记录忽略）
	if t.RunAsUser != "" && !isWindows && !isRootUser(t.RunAsUser) {
		full := append([]string{name}, args...)
		if _, err := exec.LookPath("runuser"); err == nil {
			name = "runuser"
			args = append([]string{"-u", t.RunAsUser, "--"}, full...)
		} else {
			name = "su"
			args = []string{t.RunAsUser, "-c", joinQuoted(full)}
		}
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	if envs := parseEnv(t.Env); len(envs) > 0 {
		cmd.Env = append(os.Environ(), envs...)
	}
	return cmd, cleanup, nil
}

// shellPath 选择默认 shell（优先 bash，回退 sh）
func shellPath() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	if p, err := exec.LookPath("bash"); err == nil {
		return p
	}
	if p, err := exec.LookPath("sh"); err == nil {
		return p
	}
	return "/bin/sh"
}

// isRootUser 判断目标用户是否即当前运行用户（避免无谓提权/降权包装）
func isRootUser(user string) bool {
	cur := currentUser()
	return user == "" || user == cur
}

// parseEnv 解析环境变量串（KEY=VALUE，分号或换行分隔）
func parseEnv(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == '\n' || r == '\r' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// joinQuoted 拼接并逐项加引号（供 su -c 使用）
func joinQuoted(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, it := range items {
		quoted = append(quoted, shellQuote(it))
	}
	return strings.Join(quoted, " ")
}

// shellQuote 单引号包裹并转义内部单引号
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[agent] "+format+"\n", args...)
}
