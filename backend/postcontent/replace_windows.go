//go:build windows

package postcontent

import "os"

// replaceFile 把 src 原子地替换成 dst。
//
// Windows 上 os.Rename **不能覆盖已存在的文件**（POSIX 的 rename 可以），
// 所以这里先删掉目标再改名。两步之间有一个极短的空窗期（文件不存在），
// 读取侧遇到 ErrNotFound 时的行为是「正文为空」而不是报错，不会影响使用。
func replaceFile(src, dst string) error {
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(src, dst)
}
