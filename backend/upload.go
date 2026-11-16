package main

// ============================================================
// upload.go —— 图片上传模块
// 职责：接收富文本编辑器/封面图上传的图片文件，保存到磁盘，
//       返回一个可被浏览器直接访问的 URL（/uploads/xxx.jpg）。
//
// 安全措施：
//   1. 只允许图片扩展名（jpg/jpeg/png/gif/webp）
//   2. 限制大小（10MB）
//   3. 文件名用随机十六进制重命名（避免用户文件名冲突/路径注入）
//   4. 该接口挂在 /api/admin 下，必须登录才能上传
// ============================================================

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// maxUploadSize 单张图片大小上限：10MB
const maxUploadSize = 10 << 20 // 10 * 1024 * 1024 字节

// allowedImageExt 允许的图片扩展名集合（小写）
var allowedImageExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

// handleUpload 上传接口：POST /api/admin/upload
// 请求：multipart/form-data，文件字段名必须叫 file（与前端 api.uploadImage 对应）
// 响应：{"url": "/uploads/3f8a1c2b....jpg"}
func handleUpload(c *gin.Context) {
	// ① 从表单里取文件（字段名 file）
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未接收到文件"})
		return
	}

	// ② 大小校验
	if file.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片不能超过 10MB"})
		return
	}

	// ③ 扩展名校验（按文件名后缀白名单过滤）
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExt[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 jpg / jpeg / png / gif / webp 图片"})
		return
	}

	// ④ 生成随机文件名（16 位十六进制），避免重名覆盖和路径注入
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务端错误"})
		return
	}
	name := hex.EncodeToString(buf) + ext
	dst := filepath.Join(uploadDir, name)

	// ⑤ 保存到 uploads 目录（uploadDir 是 main.go 里设置的包级变量）
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	// ⑥ 返回可访问的 URL（/uploads 目录由 gin 静态服务托管，见 main.go）
	c.JSON(http.StatusOK, gin.H{"url": "/uploads/" + name})
}
