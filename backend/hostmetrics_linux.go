//go:build linux

package main

// hostmetrics_linux.go —— Linux 上读真实的 CPU / 内存 / 磁盘
//
// 全部走 /proc 和 statfs，不引入任何第三方依赖（服务器上 go build 不需要额外模块）。
// 读不到的项一律给 0，前端会显示成「—」。

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func readHostMetrics() hostStatus {
	h := hostStatus{Supported: true, CPUCount: runtime.NumCPU()}
	h.Load1 = readLoad1()
	h.MemTotal, h.MemUsed = readMem()
	h.DiskTotal, h.DiskUsed = readDisk("/")
	h.CPUPercent = readCPUPercent()
	h.Load1 = round2(h.Load1)
	h.MemTotal, h.MemUsed = round2(h.MemTotal), round2(h.MemUsed)
	h.DiskTotal, h.DiskUsed = round2(h.DiskTotal), round2(h.DiskUsed)
	h.CPUPercent = round2(h.CPUPercent)
	h.LoadOK, h.CPUOK, h.MemOK, h.DiskOK = true, true, true, true
	return h
}

// /proc/loadavg 的第一列就是 1 分钟平均负载
func readLoad1() float64 {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(f[0], 64)
	return v
}

// /proc/meminfo：MemTotal - MemAvailable（单位 kB → GB）
func readMem() (total, used float64) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var totalKB, availKB float64
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.ParseFloat(f[1], 64)
		switch f[0] {
		case "MemTotal:":
			totalKB = v
		case "MemAvailable:":
			availKB = v
		}
	}
	const kbPerGB = 1024 * 1024
	return totalKB / kbPerGB, (totalKB - availKB) / kbPerGB
}

// statfs：总量与已用（GB）
func readDisk(path string) (total, used float64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	const bPerGB = 1024 * 1024 * 1024
	total = float64(st.Blocks) * float64(st.Bsize) / bPerGB
	free := float64(st.Bavail) * float64(st.Bsize) / bPerGB
	return total, total - free
}

// CPU 使用率：/proc/stat 两次采样（间隔 200ms）算差值
func readCPUPercent() float64 {
	idle1, total1, ok1 := readCPUSample()
	if !ok1 {
		return 0
	}
	time.Sleep(200 * time.Millisecond)
	idle2, total2, ok2 := readCPUSample()
	if !ok2 || total2 <= total1 {
		return 0
	}
	dTotal := float64(total2 - total1)
	dIdle := float64(idle2 - idle1)
	if dTotal <= 0 {
		return 0
	}
	pct := (dTotal - dIdle) / dTotal * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct
}

func readCPUSample() (idle, total uint64, ok bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	line, _, _ := strings.Cut(string(b), "\n")
	f := strings.Fields(line)
	if len(f) < 5 || f[0] != "cpu" {
		return 0, 0, false
	}
	var vals []uint64
	for _, s := range f[1:] {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		vals = append(vals, v)
	}
	for i, v := range vals {
		total += v
		if i == 3 { // idle
			idle = v
		}
	}
	return idle, total, true
}
