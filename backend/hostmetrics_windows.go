//go:build windows

package main

// hostmetrics_windows.go —— Windows 上读真实的 CPU / 内存 / 磁盘
//
// 全部走 kernel32 的三个 API（syscall.NewLazyDLL），不引入 cgo、不加第三方依赖，
// 交叉编译和本地 go build 都一样。
//
//   GetSystemTimes            CPU 占用（两次采样算差值，kernel 时间里已含 idle）
//   GlobalMemoryStatusEx      物理内存总量 / 可用
//   GetDiskFreeSpaceExW       当前盘的总量 / 可用
//
// 负载（loadavg）是 Unix 概念，Windows 没有对应指标 —— 留 0 并标记 loadOK=false，
// 前端那一项显示「—」，其余三项照常显示真实数值。

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
)

type fileTime struct{ Low, High uint32 }

func (f fileTime) uint64() uint64 { return uint64(f.High)<<32 | uint64(f.Low) }

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func readHostMetrics() hostStatus {
	h := hostStatus{CPUCount: runtime.NumCPU()}

	h.MemTotal, h.MemUsed = readMemWindows()
	h.MemOK = h.MemTotal > 0

	h.DiskTotal, h.DiskUsed = readDiskWindows()
	h.DiskOK = h.DiskTotal > 0

	h.CPUPercent = readCPUWindows()
	h.CPUOK = true

	h.LoadOK = false // Windows 没有 loadavg
	h.Supported = h.CPUOK || h.MemOK || h.DiskOK

	h.MemTotal, h.MemUsed = round2(h.MemTotal), round2(h.MemUsed)
	h.DiskTotal, h.DiskUsed = round2(h.DiskTotal), round2(h.DiskUsed)
	h.CPUPercent = round2(h.CPUPercent)
	return h
}

// CPU 占用：GetSystemTimes(idle, kernel, user)，
// 两次采样相隔 200ms，占用 = 1 - Δidle / Δ(kernel+user)。
func readCPUWindows() float64 {
	idle1, total1, ok1 := cpuTimes()
	if !ok1 {
		return 0
	}
	time.Sleep(200 * time.Millisecond)
	idle2, total2, ok2 := cpuTimes()
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

func cpuTimes() (idle, total uint64, ok bool) {
	var i, k, u fileTime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&i)),
		uintptr(unsafe.Pointer(&k)),
		uintptr(unsafe.Pointer(&u)),
	)
	if r == 0 {
		return 0, 0, false
	}
	// kernel 时间已经包含 idle，所以总量是 kernel + user
	return i.uint64(), k.uint64() + u.uint64(), true
}

func readMemWindows() (totalGB, usedGB float64) {
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return 0, 0
	}
	const bPerGB = 1024 * 1024 * 1024
	total := float64(m.TotalPhys) / bPerGB
	used := float64(m.TotalPhys-m.AvailPhys) / bPerGB
	return total, used
}

// 磁盘：当前工作目录所在盘符（取不到就退回 C:\）
func readDiskWindows() (totalGB, usedGB float64) {
	root := `C:\`
	if wd, err := os.Getwd(); err == nil {
		if v := filepath.VolumeName(wd); v != "" {
			root = v + `\`
		}
	}
	p, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, 0
	}
	var freeAvail, total, totalFree uint64
	r, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeAvail)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r == 0 {
		return 0, 0
	}
	const bPerGB = 1024 * 1024 * 1024
	t := float64(total) / bPerGB
	u := float64(total-freeAvail) / bPerGB
	return t, u
}
