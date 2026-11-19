//go:build !linux && !windows

package main

// hostmetrics_other.go —— macOS / BSD 等其它平台
//
// 这些平台不是部署目标（服务器是 Linux、本地开发用 Windows），
// 只保证能编译通过：全部标记为不可用，前端显示「—」。

import "runtime"

func readHostMetrics() hostStatus {
	return hostStatus{
		Supported: false,
		CPUCount:  runtime.NumCPU(),
	}
}
