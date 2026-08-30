package main

// ============================================================
// models.go —— 数据模型
// 职责：定义 HTTP 接口与数据库行之间的「数据结构」。
// Go 里通过 json 标签控制字段在 JSON 里的名字（即前端看到的字段名）。
// ============================================================

// Post 文章完整模型（详情页 / 编辑页使用，包含正文 content）。
// 对应数据库 posts 表的一行。
type Post struct {
	ID        int64  `json:"id"`        // 文章 ID
	Title     string `json:"title"`     // 标题
	Summary   string `json:"summary"`   // 摘要
	Cover     string `json:"cover"`     // 封面图 URL
	Content   string `json:"content"`   // 正文 HTML（wangEditor 输出）
	Status    string `json:"status"`    // 状态：draft / published
	Likes     int    `json:"likes"`     // 点赞数
	CreatedAt string `json:"createdAt"` // 创建时间（如 2026-08-27 10:10:26）
	UpdatedAt string `json:"updatedAt"` // 更新时间
}

// PostListItem 文章列表项（不含正文 content）。
// 列表接口只返回它 —— 正文很大，列表用不到，省流量、省内存。
type PostListItem struct {
	ID        int64  `json:"id"`        // 文章 ID
	Title     string `json:"title"`     // 标题
	Summary   string `json:"summary"`   // 摘要
	Cover     string `json:"cover"`     // 封面图 URL
	Status    string `json:"status"`    // 状态
	Likes     int    `json:"likes"`     // 点赞数
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
	Title   string `json:"title"`   // 标题（创建时必填，更新时可选）
	Summary string `json:"summary"` // 摘要
	Cover   string `json:"cover"`   // 封面图 URL
	Content string `json:"content"` // 正文 HTML
	Status  string `json:"status"`  // 状态：draft / published
}
