//go:build !linux

package main

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"boat/internal/osp"
)

// collectMetrics 非 Linux 平台的指标采集（命令兜底，采集不到时置 0 不影响通信）
//
// 说明：Linux 平台走 /proc 直接读取（见 metrics_linux.go），精度更高且无外部命令依赖。
// Windows / macOS 这里仅做 best-effort：CPU、磁盘、内存尽量取，取不到则保持 0。
func collectMetrics(version string) osp.Metrics {
	m := osp.Metrics{AgentVer: version}
	m.CPUUsage = cpuUsageFallback()
	total, used := memInfoFallback()
	m.MemTotal, m.MemUsed = total, used
	if total > 0 {
		m.MemUsage = round1(float64(used) / float64(total) * 100)
	}
	dt, du := diskUsageFallback()
	m.DiskTotal, m.DiskUsed = dt, du
	if dt > 0 {
		m.DiskUsage = round1(float64(du) / float64(dt) * 100)
	}
	m.LoadAvg = loadAvgFallback()
	m.Uptime = uptimeFallback()
	return m
}

// cpuUsageFallback 兜底 CPU 使用率（Windows: wmic / macOS: ps 求和）
func cpuUsageFallback() float64 {
	if runtime.GOOS == "windows" {
		out := outputOf("wmic", "cpu", "get", "loadpercentage")
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "LoadPercentage") {
				continue
			}
			if v, err := strconv.ParseFloat(strings.Fields(line)[0], 64); err == nil {
				return round1(v)
			}
		}
		return 0
	}
	out := outputOf("ps", "-A", "-o", "%cpu=")
	var sum float64
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if v, err := strconv.ParseFloat(line, 64); err == nil {
			sum += v
		}
	}
	return round1(sum)
}

// memInfoFallback 兜底内存（MB，取不到返回 0）
func memInfoFallback() (total, used uint64) {
	if runtime.GOOS == "windows" {
		out := outputOf("wmic", "OS", "get", "TotalVisibleMemorySize,FreePhysicalMemory")
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) != 2 {
				continue
			}
			t, err1 := strconv.ParseFloat(fields[0], 64)
			f, err2 := strconv.ParseFloat(fields[1], 64)
			if err1 != nil || err2 != nil {
				continue
			}
			return uint64(t / 1024), uint64((t - f) / 1024)
		}
		return 0, 0
	}
	// macOS / 其他类 Unix：sysctl 取总内存，vm_stat 取空闲页数（best-effort）
	out := outputOf("sysctl", "-n", "hw.memsize")
	if v, err := strconv.ParseUint(strings.TrimSpace(out), 10, 64); err == nil && v > 0 {
		return v >> 20, 0
	}
	return 0, 0
}

// diskUsageFallback 兜底磁盘（GB，df 或 wmic）
func diskUsageFallback() (total, used uint64) {
	if runtime.GOOS == "windows" {
		out := outputOf("wmic", "logicaldisk", "get", "size,freespace")
		for _, line := range strings.Split(out, "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) != 2 {
				continue
			}
			free, err1 := strconv.ParseFloat(fields[0], 64)
			size, err2 := strconv.ParseFloat(fields[1], 64)
			if err1 != nil || err2 != nil || size <= 0 {
				continue
			}
			return uint64(size) >> 30, uint64(size-free) >> 30
		}
		return 0, 0
	}
	out := outputOf("df", "-P", "/")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return 0, 0
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 3 {
		return 0, 0
	}
	size, err1 := strconv.ParseFloat(fields[1], 64)
	usedKB, err2 := strconv.ParseFloat(fields[2], 64)
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	// df -P 单位为 512B 块
	return uint64(size*512) >> 30, uint64(usedKB*512) >> 30
}

// loadAvgFallback 兜底负载（类 Unix 解析 uptime，Windows 无对应指标）
func loadAvgFallback() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	out := outputOf("uptime")
	idx := strings.LastIndex(out, "load average")
	if idx < 0 {
		idx = strings.LastIndex(out, "load averages")
	}
	if idx < 0 {
		return ""
	}
	tail := strings.TrimSpace(out[idx:])
	parts := strings.SplitN(tail, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(parts[1], ",", " /"))
}

// uptimeFallback 兜底运行时长（秒）
func uptimeFallback() int64 {
	if runtime.GOOS == "windows" {
		return 0
	}
	out := outputOf("sysctl", "-n", "kern.boottime")
	// 形如: { sec = 1690000000, usec = 0 } Mon Jul ...
	if idx := strings.Index(out, "sec = "); idx >= 0 {
		rest := out[idx+6:]
		end := strings.IndexAny(rest, ",}")
		if end > 0 {
			if v, err := strconv.ParseInt(strings.TrimSpace(rest[:end]), 10, 64); err == nil && v > 0 {
				return timeNowUnix() - v
			}
		}
	}
	return 0
}

// outputOf 执行命令并返回标准输出（失败返回空串）
func outputOf(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(string(out), "\r", "")
}
