package main

// ============================================================
// models.go —— 数据模型
// 职责：定义 HTTP 接口与数据库行之间的「数据结构」。
// Go 里通过 json 标签控制字段在 JSON 里的名字（即前端看到的字段名）。
// ============================================================

// Post 文章完整模型（详情页 / 编辑页使用，包含正文 content）。
// 对应数据库 posts 表的一行。
type Post struct {
	ID        int64  `json:"id"`        // 文章 ID（后台编辑用）
	Slug      string `json:"slug"`      // 公开链接随机短码（/blog/<slug>）
	Title     string `json:"title"`     // 标题
	Summary   string `json:"summary"`   // 摘要
	Cover     string `json:"cover"`     // 封面图 URL
	Category  string `json:"category"`  // 分类（单选，空串 = 未分类）
	Tags      string `json:"tags"`      // 标签（多个，逗号分隔）
	Content   string `json:"content"`   // 正文 HTML（wangEditor 输出）
	Status    string `json:"status"`    // 状态：draft / published
	Likes     int    `json:"likes"`     // 点赞数
	Views     int    `json:"views"`     // 浏览量
	Comments  int    `json:"comments"`  // 评论数（含回复）
	Liked     bool   `json:"liked"`     // 当前访客是否已赞（仅详情接口返回）
	CreatedAt string `json:"createdAt"` // 创建时间（如 2026-08-27 10:10:26）
	UpdatedAt string `json:"updatedAt"` // 更新时间
}

// PostListItem 文章列表项（不含正文 content）。
// 列表接口只返回它 —— 正文很大，列表用不到，省流量、省内存。
type PostListItem struct {
	ID        int64  `json:"id"`        // 文章 ID（后台编辑用）
	Slug      string `json:"slug"`      // 公开链接随机短码
	Title     string `json:"title"`     // 标题
	Summary   string `json:"summary"`   // 摘要
	Cover     string `json:"cover"`     // 封面图 URL
	Category  string `json:"category"`  // 分类（单选）
	Tags      string `json:"tags"`      // 标签（逗号分隔）
	Status    string `json:"status"`    // 状态
	Likes     int    `json:"likes"`     // 点赞数
	Views     int    `json:"views"`     // 浏览量
	Comments  int    `json:"comments"`  // 评论数（含回复）
	CreatedAt string `json:"createdAt"` // 创建时间
	UpdatedAt string `json:"updatedAt"` // 更新时间
}

// PageResult 统一分页返回结构，所有列表接口都用它。
// 前端拿到后根据 total 计算总页数，渲染分页按钮。
type PageResult struct {
	Items    any   `json:"items"`    // 当前页的数据（PostListItem 切片）
	Total    int64 `json:"total"`    // 总条数
	Page     int   `json:"page"`     // 当前页码（从 1 开始）
	PageSize int   `json:"pageSize"` // 每页条数
}

// PostInput 创建/更新文章的请求体。
// 注意：更新接口支持「部分字段更新」，某字段传空字符串就表示「不改这个字段」。
type PostInput struct {
	Title    string `json:"title"`    // 标题（创建时必填，更新时可选）
	Summary  string `json:"summary"`  // 摘要
	Cover    string `json:"cover"`    // 封面图 URL
	Category string `json:"category"` // 分类（单选，后台可直接输入新分类）
	Tags     string `json:"tags"`     // 标签（多个，逗号分隔，自由输入）
	Content  string `json:"content"`  // 正文 HTML
	Status   string `json:"status"`   // 状态：draft / published
}

// Stats 全站统计（首页右侧统计卡片用）。
type Stats struct {
	Posts      int64 `json:"posts"`      // 已发布文章数
	Categories int64 `json:"categories"` // 分类数（去重，不含空分类）
	Tags       int64 `json:"tags"`       // 标签数（去重，按逗号拆分）
}

// ============================================================
// 用户系统相关模型（console.go 与各 handler 共用）
// ============================================================

// User 用户模型（对外返回时不含 password_hash）。
type User struct {
	ID        int64  `json:"id"`        // 用户 ID（数据库主键，内部用）
	UserID    string `json:"userId"`    // 桃信 ID：9 位数字，全站唯一，别人靠它搜索到你
	Email     string `json:"email"`     // 邮箱（登录标识）
	Username  string `json:"username"`  // 用户名（管理员才有，普通用户为空）
	Nickname  string `json:"nickname"`  // 昵称（展示用）
	Avatar    string `json:"avatar"`    // 头像 URL
	Cover     string `json:"cover"`     // 个人主页封面图（空 = 用站点首页背景）
	Level     int    `json:"level"`     // 等级（由 points 换算出来，见 level.go）
	Points    int    `json:"points"`    // 累计积分（目前只能靠每日签到获得）
	Role      string `json:"role"`      // 角色：admin / user
	CreatedAt string `json:"createdAt"` // 注册时间
	// 站长资料（出生日期 / 性别 / GitHub / 联系方式 / 简介）+ 派生的星座、年龄
	extraProfile
}

// UserBrief 用户简版信息（评论区 / 通知 / 桃信列表里嵌套展示，免去不必要的字段）。
type UserBrief struct {
	ID       int64  `json:"id"`       // 用户 ID
	UserID   string `json:"userId"`   // 桃信 9 位 ID
	Nickname string `json:"nickname"` // 昵称（空则回退邮箱前缀，前端处理）
	Avatar   string `json:"avatar"`   // 头像 URL
	Level    int    `json:"level"`    // 等级
	Role     string `json:"role"`     // 角色（评论展示站长徽标用）
	Online   bool   `json:"online"`   // 是否在线（来自 Redis；未启用 Redis 时恒为 false）
}

// Comment 评论（对外模型，嵌套作者信息）。
// parentID 指向顶级评论 ID；replyTo 记录回复 @ 的人，均可能为空。
type Comment struct {
	ID        int64      `json:"id"`        // 评论 ID
	PostID    int64      `json:"postId"`    // 所属文章
	ParentID  int64      `json:"parentId"`  // 顶级评论 ID（0 = 顶级评论）
	ReplyTo   *UserBrief `json:"replyTo"`   // 回复 @ 的人（nil = 顶级）
	Author    UserBrief  `json:"author"`    // 评论者
	Content   string     `json:"content"`   // 内容（纯文本）
	CreatedAt string     `json:"createdAt"` // 时间
	Replies   []Comment  `json:"replies"`   // 挂在它下面的回复（两级结构）
}

// CommentAdminItem 后台评论管理列表项（附带文章标题等管理信息）。
type CommentAdminItem struct {
	ID        int64      `json:"id"`        // 评论 ID
	PostID    int64      `json:"postId"`    // 文章 ID
	PostTitle string     `json:"postTitle"` // 文章标题
	PostSlug  string     `json:"postSlug"`  // 文章公开链接短码
	ParentID  int64      `json:"parentId"`  // 顶级评论 ID（0=顶级）
	ReplyTo   *UserBrief `json:"replyTo"`   // @ 的目标（nil=顶级）
	Author    UserBrief  `json:"author"`    // 评论者
	Content   string     `json:"content"`   // 内容
	CreatedAt string     `json:"createdAt"` // 时间
}

// UserAdminItem 后台用户管理列表项。
type UserAdminItem struct {
	User
	CommentCount int64 `json:"commentCount"` // 该用户的评论数
}

// Notification 站内通知模型。
type Notification struct {
	ID        int64      `json:"id"`        // 通知 ID
	Actor     *UserBrief `json:"actor"`     // 触发者（可能已被删，nil）
	PostID    int64      `json:"postId"`    // 关联文章 ID
	PostSlug  string     `json:"postSlug"`  // 关联文章公开短码（点击跳转用）
	CommentID int64      `json:"commentId"` // 关联评论
	Content   string     `json:"content"`   // 展示文本
	IsRead    bool       `json:"isRead"`    // 是否已读
	CreatedAt string     `json:"createdAt"` // 时间
}

// ============================================================
// 【桃信】私信 / 好友系统模型
// ============================================================

// MessengerUser 桃信用户公开资料（搜索 / 资料卡 / 好友列表都用它）。
// 注意：只暴露 9 位桃信 ID（userId），内部自增主键另外放在 id 里供接口调用。
type MessengerUser struct {
	ID             int64  `json:"id"`             // 内部用户 ID（接口参数用）
	UserID         string `json:"userId"`         // 9 位桃信 ID（搜索 / 展示用）
	Nickname       string `json:"nickname"`       // 昵称
	Avatar         string `json:"avatar"`         // 头像
	Level          int    `json:"level"`          // 等级
	Role           string `json:"role"`           // 角色：admin / user
	CreatedAt      string `json:"createdAt"`      // 注册时间
	Relation       string `json:"relation"`       // 与我的关系：self/friend/pending_out/pending_in/none
	ConversationID int64  `json:"conversationId"` // 已有会话 ID（0=还没聊过）
	Online         bool   `json:"online"`         // 是否在线（来自 Redis，未启用 Redis 时恒为 false）
}

// FriendItem 好友列表项（附带最近一条私信与未读数，桃信列表页直接渲染）。
type FriendItem struct {
	MessengerUser
	FriendSince   string `json:"friendSince"`   // 成为好友的时间
	LastMessage   string `json:"lastMessage"`   // 最近一条私信内容
	LastMessageAt string `json:"lastMessageAt"` // 最近一条私信时间
	LastSenderID  int64  `json:"lastSenderId"`  // 最近一条私信的发送者（前端显示「我：…」）
	Unread        int64  `json:"unread"`        // 未读私信数
}

// FriendRequestItem 好友申请项。
// RelationID 是 friends 表的行 ID（同意 / 拒绝接口用它），ID 是对方的用户 ID。
type FriendRequestItem struct {
	RelationID int64  `json:"relationId"` // friends 表行 ID
	ID         int64  `json:"id"`         // 对方用户 ID
	UserID     string `json:"userId"`     // 对方 9 位桃信 ID
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar"`
	Level      int    `json:"level"`
	Role       string `json:"role"`
	Status     string `json:"status"`    // pending / accepted / rejected
	Incoming   bool   `json:"incoming"`  // true=他申请加我；false=我申请加他
	CreatedAt  string `json:"createdAt"` // 申请时间
}

// DirectMessage 一条私信。
type DirectMessage struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversationId"`
	SenderID       int64  `json:"senderId"`     // 发送者内部用户 ID（前端对比「是不是我发的」）
	SenderUserID   string `json:"senderUserId"` // 发送者 9 位桃信 ID
	Content        string `json:"content"`
	CreatedAt      string `json:"createdAt"`
}

// ConversationItem 会话列表项（左侧「未读信息」列表用）。
type ConversationItem struct {
	ConversationID int64     `json:"conversationId"`
	Peer           UserBrief `json:"peer"`          // 对方
	LastMessage    string    `json:"lastMessage"`   // 最近一条私信
	LastMessageAt  string    `json:"lastMessageAt"` // 最近一条私信时间
	LastSenderID   int64     `json:"lastSenderId"`  // 最近一条私信的发送者（前端显示「我：…」）
	Unread         int64     `json:"unread"`        // 未读条数
	IsFriend       bool      `json:"isFriend"`      // 是否已是好友（列表里显示不同按钮）
}
