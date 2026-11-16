//go:build !windows

package main

import "os"

// replaceFileAtomic 在 POSIX 系统上直接 rename —— 它本身就是原子的覆盖操作。
func replaceFileAtomic(src, dst string) error {
	return os.Rename(src, dst)
}
