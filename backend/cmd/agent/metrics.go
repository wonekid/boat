package main

import (
	"os"
	"os/user"
	"runtime"
	"time"

	"boat/internal/osp"
)

// SysInfo 执行机静态信息（握手时上报）
type SysInfo struct {
	Hostname string
	OS       string
	Arch     string
	IP       string
}

// collectSysInfo 采集主机名、系统、架构与出口 IP
func collectSysInfo(cfg *Config) SysInfo {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = cfg.Name
	}
	return SysInfo{
		Hostname: host,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		IP:       osp.LocalIP(),
	}
}

// currentUser 当前运行用户（agent 通常以 root/System 运行）
func currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}

// round1 保留一位小数
func round1(v float64) float64 {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return float64(int(v*10+0.5)) / 10
}

// timeNowUnix 当前 Unix 秒（供非 Linux 平台计算运行时长）
func timeNowUnix() int64 { return time.Now().Unix() }
