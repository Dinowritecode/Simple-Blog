//go:build !windows

package postcontent

import "os"

// replaceFile 在 POSIX 系统上直接 rename —— 它本身就是原子的覆盖操作。
func replaceFile(src, dst string) error {
	return os.Rename(src, dst)
}
