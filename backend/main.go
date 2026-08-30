package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// uploadDir 图片上传目录（包级变量，upload.go / console.go 也会用到）。
// 由环境变量 UPLOAD_DIR 指定，默认是 backend 目录下的 uploads。
var uploadDir string

// Runtime config as package-level vars (read by console.go commands like info / changepass).
var (
	port      string   // HTTP listen port
	adminUser string   // admin account name
	adminPass string   // initial admin password (only used when users table is empty)
	dbCfg     DBConfig // MySQL connection config (defined in db.go)
)

// main 程序入口：负责「读配置 → 初始化数据库 → 初始化管理员 → 注册路由 → 启动 HTTP 服务」。
// 整个后端是一个二进制文件，同时提供 API、图片托管、前端页面托管三件事，
// 并附带一个终端命令控制台（console.go），支持 info / changepass 等运维命令。
func main() {
	// ---------- ① 读取环境变量（均有默认值，不设置也能跑） ----------
	port = getenv("PORT", "8080") // HTTP 监听端口
	// 前端构建产物目录（默认指向 ../frontend/dist，即 backend 的上一级目录下的 frontend/dist）
	staticDir := getenv("STATIC_DIR", filepath.Join("..", "frontend", "dist"))
	// 图片上传目录
	uploadDir = getenv("UPLOAD_DIR", "uploads")
	// 初始管理员账号密码（只在 users 表为空时生效）
	adminUser = getenv("ADMIN_USER", "admin")
	adminPass = getenv("ADMIN_PASSWORD", "admin123")

	// MySQL 连接配置（DBConfig 定义在 db.go）
	dbCfg = DBConfig{
		Host:     getenv("DB_HOST", "127.0.0.1"),
		Port:     getenv("DB_PORT", "3306"),
		User:     getenv("DB_USER", "root"),
		Password: getenv("DB_PASSWORD", ""),
		Name:     getenv("DB_NAME", "blog"),
	}

	// ---------- ② 初始化：数据库 + 管理员 + 上传目录 ----------
	// initDB 会连接 MySQL，自动创建数据库和三张表（幂等，可重复执行）
	if err := initDB(dbCfg); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	// ensureAdmin 在 users 表为空时创建默认管理员（密码 bcrypt 加密存储）
	if err := ensureAdmin(adminUser, adminPass); err != nil {
		log.Fatalf("初始化管理员失败: %v", err)
	}
	// 确保上传目录存在
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}

	// ---------- ②.5 命令行模式（一次性命令，执行完即退出） ----------
	// 例：go run . info                    —— 查看服务/数据库信息
	//     go run . changepass <新密码>      —— 修改管理员密码
	//     go run . changepass <用户> <密码>  —— 修改指定用户密码
	if len(os.Args) > 1 {
		os.Exit(runCommandOnce(os.Args[1], os.Args[2:]))
	}

	// ---------- ③ 创建 gin 路由 ----------
	gin.SetMode(gin.ReleaseMode) // 生产模式（关闭调试日志细节）
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger()) // 崩溃恢复 + 请求日志

	// 上传的图片通过 /uploads/xxx.jpg 直接访问（由 gin 静态文件服务托管）
	r.Static("/uploads", uploadDir)

	// ---------- ④ 注册 /api 接口 ----------
	// 路由分组：/api 下的所有接口
	api := r.Group("/api")
	{
		// 【公开接口】登录：任何人可调用
		api.POST("/auth/login", handleLogin)

		// 【公开接口】博客浏览：任何人可调用
		api.GET("/posts", handleListPosts)          // 已发布文章分页列表
		api.GET("/posts/:id", handleGetPost)        // 文章详情
		api.POST("/posts/:id/like", handleLikePost) // 点赞

		// 【管理接口】需要登录：整个分组挂上 authMiddleware（JWT 校验）
		admin := api.Group("/admin", authMiddleware())
		{
			admin.GET("/posts", handleAdminListPosts)        // 全部文章（含草稿）
			admin.GET("/posts/:id", handleAdminGetPost)      // 文章详情（含草稿）
			admin.POST("/posts", handleCreatePost)           // 新建文章
			admin.PUT("/posts/:id", handleUpdatePost)        // 更新文章（部分字段）
			admin.DELETE("/posts/:id", handleDeletePost)     // 删除文章
			admin.POST("/upload", handleUpload)              // 上传图片
		}
	}

	// ---------- ⑤ 前端页面托管（SPA 回退） ----------
	// 必须是最后注册，用 NoRoute 兜住所有「不是 API」的路径
	serveFrontend(r, staticDir)

	// ---------- ⑥ 启动 ----------
	// 交互式命令控制台（goroutine，不阻塞 HTTP；stdin 不可用时自动静默退出）
	startConsole()

	log.Printf("The backend service has been successfully started: http://localhost:%s", port)
	log.Printf("MySQL service loading completed. %s@%s:%s/%s", dbCfg.User, dbCfg.Host, dbCfg.Port, dbCfg.Name)
	log.Printf("Administrator %s / %s please modify via ADMIN_PASSWORD environment variable as soon as possible", adminUser, adminPass)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
