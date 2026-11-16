//go:build windows

package main

import "os"

// replaceFileAtomic 把 src 原子地替换成 dst。
//
// Windows 上 os.Rename **不能覆盖已存在的文件**（POSIX 的 rename 可以），
// 所以先删目标再改名。两步之间有极短的空窗期，对恢复流程没有影响
// （这段时间目标文件短暂不存在，而不是内容错乱）。
func replaceFileAtomic(src, dst string) error {
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(src, dst)
}
