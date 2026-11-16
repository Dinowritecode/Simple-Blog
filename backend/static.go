package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// serveFrontend 负责托管前端页面（SPA，单页应用）。
//
// 原理说明：
//  1. 前端用 vue-router 的 history 模式，路由是「假路径」（如 /blog/2），
//     服务器上并不存在这个文件，所以除真实文件外的一切路径都要回退到 index.html，
//     由前端路由自己决定渲染哪个页面 —— 这就是「SPA 回退」。
//  2. /api 开头的路径不是页面，直接返回 404 JSON，避免被回退逻辑吞掉。
func serveFrontend(r *gin.Engine, staticDir string) {
	// NoRoute：所有未匹配到显式路由的请求都会进到这里
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path

		// ① API 请求不属于前端页面，返回 404 JSON
		if strings.HasPrefix(p, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}

		// ② 如果确实是磁盘上存在的静态资源（如带 hash 的 JS/CSS、图片），直接返回文件
		// filepath.Clean 用于防止路径穿越（如 /../ 注入）
		file := filepath.Join(staticDir, filepath.Clean("/"+p))
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			c.File(file)
			return
		}

		// ③ 其余一切路径都回退到 index.html，交给前端 vue-router 处理
		index := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			c.File(index)
			return
		}

		// ④ 连 index.html 都没有，说明前端还没构建过
		c.JSON(http.StatusNotFound, gin.H{"error": "前端尚未构建，请先在 frontend 目录执行 npm run build"})
	})
}
