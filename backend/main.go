package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"rineyce/blog/chatstore"
	"rineyce/blog/postcontent"
)

// uploadDir 图片上传目录（包级变量，upload.go / console.go 也会用到）。
// 由环境变量 UPLOAD_DIR 指定，默认是 backend 目录下的 uploads。
var uploadDir string

// serverStart 本次进程的启动时间。
// 前端个人名片里的「本站已连续运行 …」用它计算：服务一重启就归零，是真正的 uptime。
var serverStart = time.Now()

// Runtime config as package-level vars (read by console.go commands like info / changepass).
var (
	port      string   // HTTP listen port
	adminUser string   // admin account name
	adminPass string   // initial admin password (only used when users table is empty)
	dbCfg     DBConfig // PostgreSQL connection config (defined in db.go)
)

// 聊天相关的两个全局依赖（都在 main 里初始化）：
//
//	chatStore —— 聊天数据的存储层（chatstore.Store 接口，PostgreSQL 分区表实现）
//	realtime  —— 聊天的实时状态层（Redis：在线 / 正在输入 / 限流 / 跨实例广播）
//
// 之所以做成「接口 + 可空」：
//
//	· 存储层做接口 → 将来换 PostgreSQL 只需新增一个实现文件
//	· 实时层可空   → 没配 Redis 时聊天功能照常，只是少了在线与输入中提示
var (
	chatStore chatstore.Store
	realtime  *rts

	// postContent 文章正文的文件存储（未配置 POST_CONTENT_DIR 时为 nil）。
	postContent *postcontent.Store
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
	// 站长账号名（默认 admin）；初始密码策略见 ②（首次部署自动生成随机密码，或用 ADMIN_PASSWORD 指定）
	adminUser = getenv("ADMIN_USER", "admin")
	adminPass = os.Getenv("ADMIN_PASSWORD")

	// PostgreSQL 连接配置（DBConfig 定义在 db.go）—— 全站唯一数据库
	dbCfg = DBConfig{
		Host:     getenv("DB_HOST", "127.0.0.1"),
		Port:     getenv("DB_PORT", "5432"),
		User:     getenv("DB_USER", "postgres"),
		Password: getenv("DB_PASSWORD", ""),
		Name:     getenv("DB_NAME", "blog"),
		SSLMode:  getenv("DB_SSLMODE", "disable"),
	}

	// ---------- ② 初始化：数据库 + 管理员 + 上传目录 ----------
	// initDB 会连接 PostgreSQL 并自动建表（幂等，可重复执行）
	if err := initDB(dbCfg); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// ---------- ②.1 聊天存储层（同为 PostgreSQL，消息表按月分区） ----------
	initChatStore()

	// ---------- ②.2 文章正文的文件存储层 ----------
	// 正文是不参与检索的大段 HTML，放在文件系统；posts 表只留列表/筛选用的索引列。
	// 目录未配置时用默认的 ./data/posts（相对后端工作目录）。
	var err error
	postContent, err = postcontent.New(getenv("POST_CONTENT_DIR", filepath.Join("..", "data", "posts")))
	if err != nil {
		log.Fatalf("初始化文章正文目录失败: %v", err)
	}
	if postContent.Enabled() {
		log.Printf("文章正文：使用文件存储（%s）", postContent.Root())
		// 首次启用时把库里已有的正文搬到文件（幂等：没有待迁移的就直接返回）
		migratePostsOnStart()
	} else {
		log.Printf("文章正文：未配置目录，正文仍存数据库")
	}

	// ---------- ②.3 聊天实时状态层（Redis，可选） ----------
	// 未配置 REDIS_ADDR → 返回 nil，聊天功能照常（在线状态一律显示离线）
	realtime = initRTS(loadRedisConfig())
	if realtime != nil {
		defer realtime.Close()
		// 订阅「其它实例」广播的事件，转投给本机对应的 SSE 连接
		// （单实例部署时这条通道基本不参与，但保留它才能水平扩容）
		realtime.onEvent(func(kind string, payload []byte) {
			var ue userEvent
			if err := json.Unmarshal(payload, &ue); err != nil {
				return
			}
			eventHub.broadcast(ue.UserID, event{Type: kind, Data: ue.Data})
		})
	}

	// ensureAdmin：不存在管理员时自动创建（部署即就绪，无需注册）。
	// 密码策略：显式设置了 ADMIN_PASSWORD 就用它；否则首次部署生成随机密码并打印到日志。
	adminPass = os.Getenv("ADMIN_PASSWORD")
	firstRun := false
	if adminPass == "" {
		var n int
		_ = q.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&n)
		if n == 0 {
			adminPass = randomPassword(12)
			firstRun = true
		} else {
			adminPass = "" // 已存在管理员，密码仅作日志展示占位
		}
	}
	if err := ensureAdmin(adminUser, adminPass); err != nil {
		log.Fatalf("初始化管理员失败: %v", err)
	}
	if firstRun {
		// 仅在「首次创建」时打印一次明文初始密码，之后不再出现（请尽快修改）
		log.Printf("首次部署：已自动创建站长账号 %s，初始密码为 %s（请尽快登录后台修改）", adminUser, adminPass)
	}
	// 确保上传目录存在（含头像子目录）
	if err := os.MkdirAll(filepath.Join(uploadDir, "avatars"), 0o755); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}

	// ---------- ②.5 命令行模式（一次性命令，执行完即退出） ----------
	// 例：go run . info                    —— 查看服务/数据库信息
	//     go run . changepass <新密码>      —— 修改管理员密码
	//     go run . changepass <用户> <密码>  —— 修改指定用户密码
	//     go run . backup out.zip           —— 导出备份
	//     go run . restore out.zip REPLACE  —— 从备份恢复
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
		// 【公开接口】登录 / 注册：任何人可调用
		api.POST("/auth/login", handleLogin)          // 邮箱或用户名 + 密码
		api.POST("/auth/register", handleRegister)    // 邮箱 + 密码（注册即自动登录）
		api.GET("/settings", handleGetPublicSettings) // 公开站点设置（首页背景图 + 公告等）
		api.GET("/stats", handleGetStats)             // 全站统计（已发布文章数 / 分类数 / 标签数）

		// 【公开接口】博客浏览：任何人可调用（URL 用随机短码 slug，如 /api/posts/aB3xYz9Q）
		api.GET("/posts", handleListPosts)                   // 已发布文章分页列表（可按 category / tag / q 过滤）
		api.GET("/categories", handleListCategories)         // 分类列表 + 每类文章数（首页分类器用）
		api.GET("/tags", handleListTags)                     // 标签列表 + 每个标签的文章数（搜索页标签云）
		api.GET("/search", handleSearch)                     // 站内搜索：文章 / 分类 / 标签 / 用户（用户需登录）
		api.GET("/posts/:slug", handleGetPost)               // 文章详情（含当前访客点赞状态）
		api.POST("/posts/:slug/like", handleLikePost)        // 点赞 / 取消（匿名、同一访客一次）
		api.GET("/posts/:slug/comments", handleListComments) // 评论列表（公开浏览）

		// 【说说】公开读 + 匿名点赞 / 记浏览（用 blog_visitor Cookie 去重）
		api.GET("/moments", handleListMoments)                     // 说说列表（分页）
		api.POST("/moments/:id/like", handleLikeMoment)            // 点赞 / 取消
		api.POST("/moments/:id/view", handleViewMoment)            // 记录浏览（同访客只计一次）
		api.GET("/moments/:id/comments", handleListMomentComments) // 评论列表（公开）

		// 【个人主页】公开：点任意头像 / 昵称都能看（只含公开字段，不含邮箱）
		api.GET("/users/:id/profile", handleGetUserProfile)

		// 【登录用户接口】个人中心 + 发表评论：需登录
		user := api.Group("/user", authMiddleware())
		{
			user.GET("/me", handleMe)                                     // 当前用户信息
			user.PUT("/profile", handleUpdateProfile)                     // 修改昵称
			user.PUT("/password", handleChangePassword)                   // 修改密码（需原密码）
			user.POST("/avatar", handleUploadAvatar)                      // 上传头像（≤5MB）
			user.GET("/notifications", handleNotifications)               // 回复通知列表
			user.POST("/notifications/read", handleMarkNotificationsRead) // 标记已读
			user.GET("/checkin", handleCheckinStatus)                     // 等级系统：签到状态 / 等级进度 / 本月日历
			user.POST("/checkin", handleCheckin)                          // 等级系统：每日签到（+5 积分，一天一次）

			// 说说：发布 / 删除（读走公开的 /api/moments）
			user.POST("/moments", handleCreateMoment)       // 发布一条说说（文字 + 最多 9 张图）
			user.DELETE("/moments/:id", handleDeleteMoment) // 删除自己的说说（管理员可删任意）
			user.POST("/upload", handleUpload)              // 上传说说配图（登录即可，规则同后台图片）
		}
		api.POST("/posts/:slug/comments", authMiddleware(), handleCreateComment)       // 文章评论 / 回复
		api.POST("/moments/:id/comments", authMiddleware(), handleCreateMomentComment) // 说说评论 / 回复

		// 【桃信】SSE 实时事件流：刻意注册在「公开组」，因为它自带鉴权（events.go 的 sseAuthUser）。
		// 原因：EventSource 不能自定义请求头，token 只能走 ?token=xxx；
		// 而用户组统一挂的 authMiddleware 只认 Authorization 头，会先一步把连接挡成 401。
		api.GET("/user/events", handleEventStream)

		// 【桃信】私信 / 好友：全部需要登录
		msg := api.Group("", authMiddleware())
		{
			msg.GET("/messenger/unread", handleUnreadSummary) // 消息按钮的红点汇总
			msg.GET("/users/search", handleSearchUsers)       // 按 9 位 ID / 昵称搜索用户
			msg.GET("/users/:id", handleGetUser)              // 某个用户的 Momo Talk 资料

			msg.GET("/friends", handleListFriends)                 // 好友列表（含申请）
			msg.GET("/friends/requests", handleListFriendRequests) // 只要申请列表
			msg.POST("/friends/request", handleFriendRequest)      // 发起好友申请
			msg.POST("/friends/:id/accept", handleFriendAccept)    // 同意
			msg.POST("/friends/:id/reject", handleFriendReject)    // 拒绝
			msg.DELETE("/friends/:id", handleFriendDelete)         // 删除好友 / 撤回申请

			msg.GET("/conversations", handleListConversations)          // 会话列表
			msg.POST("/conversations", handleOpenConversation)          // 打开（或创建）会话
			msg.GET("/conversations/:id/messages", handleListMessages)  // 会话消息（含标记已读）
			msg.POST("/conversations/:id/messages", handleSendMessage)  // 发私信
			msg.POST("/conversations/:id/read", handleReadConversation) // 标记已读
			msg.POST("/conversations/:id/typing", handleTyping)         // 「我正在输入」（Redis，瞬时状态）
		}

		// 【管理接口】仅管理员：adminMiddleware 在登录校验基础上要求 role='admin'
		admin := api.Group("/admin", adminMiddleware())
		{
			admin.GET("/posts", handleAdminListPosts)    // 全部文章（含草稿）
			admin.GET("/posts/:id", handleAdminGetPost)  // 文章详情（含草稿）
			admin.POST("/posts", handleCreatePost)       // 新建文章
			admin.PUT("/posts/:id", handleUpdatePost)    // 更新文章（部分字段）
			admin.DELETE("/posts/:id", handleDeletePost) // 删除文章（级联清理）
			admin.POST("/upload", handleUpload)          // 上传图片（正文/封面）

			admin.GET("/comments", handleAdminListComments)         // 评论管理
			admin.DELETE("/comments/:id", handleAdminDeleteComment) // 删除评论

			admin.GET("/users", handleAdminListUsers)         // 用户管理
			admin.DELETE("/users/:id", handleAdminDeleteUser) // 删除用户

			admin.PUT("/settings/notice", handleAdminUpdateNotice)     // 页面管理：首页公告
			admin.PUT("/settings/pagesize", handleAdminUpdatePageSize) // 页面管理：列表每页条数
			admin.PUT("/settings/theme", handleAdminUpdateTheme)       // 页面管理：外观定制（主题色/透明度）

			admin.PUT("/settings/home", handleAdminUpdateHomeBg)       // 页面管理：首页背景图
			admin.PUT("/settings/postbg", handleAdminUpdatePostBg)     // 页面管理：文章页背景图（与首页分开）
			admin.PUT("/settings/momotalk", handleAdminUpdateMomoTalk) // 功能开关：启用/关闭 Momo Talk
			admin.PUT("/settings/font", handleAdminUpdateFont)         // 页面管理：全站字体
			admin.PUT("/settings/clickfx", handleAdminUpdateClickFX)   // 页面管理：点击特效（ba-click-fx）参数
			admin.PUT("/settings/content", handleAdminUpdateContent)   // 页面管理：首页文案 + 首屏头像 + 登录页背景

			admin.GET("/onboarding", handleAdminOnboardingState)     // 首次进入后台的初始化引导：查状态
			admin.POST("/onboarding", handleAdminOnboardingComplete) // 首次进入后台的初始化引导：完成

			admin.GET("/backup/info", handleBackupInfo)      // 备份：当前数据概览（各表行数 / 文件数 / 体积）
			admin.GET("/backup/export", handleBackupExport)  // 备份：流式下载 zip
			admin.POST("/backup/import", handleBackupImport) // 备份：上传并恢复（整体替换，需确认词）
		}
	}

	// ---------- ⑤ 前端页面托管（SPA 回退） ----------
	// 必须是最后注册，用 NoRoute 兜住所有「不是 API」的路径
	serveFrontend(r, staticDir)

	// ---------- ⑤.1 后台维护任务 ----------
	// 每天把下个月的消息分区提前建好，并做一次存储层自检。
	// 放在独立的 goroutine 里，不阻塞 HTTP 启动。
	go startChatMaintenance()

	// ---------- ⑥ 启动 ----------
	// 交互式命令控制台（goroutine，不阻塞 HTTP；stdin 不可用时自动静默退出）
	startConsole()

	log.Printf("The backend service has been successfully started: http://localhost:%s", port)
	log.Printf("PostgreSQL connected: %s@%s:%s/%s", dbCfg.User, dbCfg.Host, dbCfg.Port, dbCfg.Name)
	// 站长登录提示：密码不落明文日志（首次部署的随机密码已在创建时打印过一次）
	if env := os.Getenv("ADMIN_PASSWORD"); env != "" {
		log.Printf("Administrator account %s password configured via ADMIN_PASSWORD", adminUser)
	} else {
		log.Printf("Administrator account %s (login at /admin/login; change password in dashboard if needed)", adminUser)
	}
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// startChatMaintenance 聊天存储的定时维护。
//
// 只做一件事：保证未来几个月的 messages 分区已经存在。
// 为什么必须做：分区表在「没有对应分区」时新数据会落进 pmax 兜底分区，
// 功能不会坏、但分区就失去意义了（所有兜底数据挤在一个分区里）。
// 启动时先跑一次，之后每 24 小时一次。
func startChatMaintenance() {
	run := func() {
		if chatStore == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := chatStore.Maintain(ctx); err != nil {
			log.Printf("聊天存储维护失败：%v", err)
		}
	}
	run()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		run()
	}
}
