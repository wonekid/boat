//go:build linux

package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"

	"boat/internal/osp"
)

// cpuSnapshot /proc/stat 快照（增量法计算 CPU 使用率）
type cpuSnapshot struct {
	total uint64
	idle  uint64
}

var (
	lastCPU     cpuSnapshot
	lastCPUOnce bool
)

// collectMetrics 采集 Linux 实时指标（全部读取 /proc，无外部命令依赖、无第三方库）
func collectMetrics(version string) osp.Metrics {
	m := osp.Metrics{AgentVer: version}
	m.CPUUsage = cpuUsage()

	totalKB, availKB := memInfo()
	m.MemTotal = totalKB / 1024
	m.MemUsed = (totalKB - availKB) / 1024
	if totalKB > 0 {
		m.MemUsage = round1(float64(totalKB-availKB) / float64(totalKB) * 100)
	}

	totalGB, usedGB := diskUsage("/")
	m.DiskTotal, m.DiskUsed = totalGB, usedGB
	if totalGB > 0 {
		m.DiskUsage = round1(float64(usedGB) / float64(totalGB) * 100)
	}

	m.LoadAvg = loadAvg()
	m.Uptime = uptimeSeconds()
	return m
}

// cpuUsage 基于两次 /proc/stat 采样的增量计算 CPU 使用率（%）
func cpuUsage() float64 {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0
	}
	var total, idle uint64
	for i, s := range fields[1:] {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			continue
		}
		total += v
		// idle(第4列) 与 iowait(第5列) 计入空闲
		if i == 3 || i == 4 {
			idle += v
		}
	}
	cur := cpuSnapshot{total: total, idle: idle}
	if !lastCPUOnce {
		lastCPU, lastCPUOnce = cur, true
		return 0
	}
	dTotal := cur.total - lastCPU.total
	dIdle := cur.idle - lastCPU.idle
	lastCPU = cur
	if dTotal == 0 {
		return 0
	}
	return round1(float64(dTotal-dIdle) / float64(dTotal) * 100)
}

// memInfo 读取 /proc/meminfo，返回总量与可用量（KB）
func memInfo() (total, available uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	var free, buffers, cached uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseKB(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseKB(line)
		case strings.HasPrefix(line, "MemFree:"):
			free = parseKB(line)
		case strings.HasPrefix(line, "Buffers:"):
			buffers = parseKB(line)
		case strings.HasPrefix(line, "Cached:"):
			cached = parseKB(line)
		}
	}
	if available == 0 {
		available = free + buffers + cached
	}
	return total, available
}

func parseKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// diskUsage 读取根分区使用情况（GB）
func diskUsage(path string) (total, used uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	totalBytes := st.Blocks * uint64(st.Bsize)
	freeBytes := st.Bavail * uint64(st.Bsize)
	if totalBytes < freeBytes {
		return 0, 0
	}
	return totalBytes >> 30, (totalBytes - freeBytes) >> 30
}

// loadAvg 读取 1/5/15 分钟负载
func loadAvg() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return ""
	}
	return strings.Join(fields[0:3], " / ")
}

// uptimeSeconds 系统已运行秒数
func uptimeSeconds() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(v)
}
