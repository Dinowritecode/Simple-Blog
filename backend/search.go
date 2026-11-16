package main

// ============================================================
// search.go —— 站内搜索（文章 / 分类 / 标签 / 用户）
//
// 接口：GET /api/search?q=关键字&limit=每组条数
//   · 公开可调用（不登录也能搜文章 / 分类 / 标签）
//   · 用户搜索**需要登录**：昵称和 9 位桃信 ID 虽然本来就是公开信息，
//     但「按任意关键字枚举全站用户」是另一回事，登录后才放开（响应里用
//     usersLocked 告诉前端「这里本来可能有结果，登录后才能看」）。
//
// 文章匹配范围：
//   标题 / 摘要 / 分类 / 标签 / 公开短码（都在 posts 表的轻量索引列上，走 SQL）
//   + 正文（在文件系统里，走 postcontent 的全文索引，命中后拼进 SQL 的 id IN (…)）
// 分类 / 标签是「从文章里派生」的，没有独立的表，所以同样是扫已发布文章统计。
//
// 与 /api/posts 的关系：两边共用 buildPostFilter，保证「搜索页看到的篇数」
// 和「点进去按同一关键字筛的文章列表」完全一致。
// ============================================================

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	searchKeywordMaxRunes = 50 // 关键字长度上限（超长直接截断，避免拼出巨大的 LIKE）
	searchDefaultLimit    = 6  // 不带 limit 时每类返回几条
	searchMaxLimit        = 30 // 每类最多返回几条
)

// SearchCategory 搜索命中的分类（count = 该分类下的已发布文章数）。
type SearchCategory struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// SearchTag 搜索命中的标签（count = 带这个标签的已发布文章数）。
type SearchTag struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// SearchUser 搜索命中的用户。
// 只暴露公开字段：**绝不返回邮箱**，昵称为空时用邮箱前缀兜底展示。
type SearchUser struct {
	ID       int64  `json:"id"`       // 内部 ID（打开资料卡用）
	UserID   string `json:"userId"`   // 9 位桃信 ID
	Nickname string `json:"nickname"` // 昵称（已做空值兜底）
	Avatar   string `json:"avatar"`
	Level    int    `json:"level"`
	Role     string `json:"role"` // admin / user
}

// SearchTotals 每一类的命中总数（前端用它显示「查看全部 N 条」）。
type SearchTotals struct {
	Posts      int64 `json:"posts"`
	Categories int64 `json:"categories"`
	Tags       int64 `json:"tags"`
	Users      int64 `json:"users"`
}

// SearchResult 综合搜索的响应体。
type SearchResult struct {
	Q           string           `json:"q"`
	Posts       []PostListItem   `json:"posts"`
	Categories  []SearchCategory `json:"categories"`
	Tags        []SearchTag      `json:"tags"`
	Users       []SearchUser     `json:"users"`
	Totals      SearchTotals     `json:"totals"`
	UsersLocked bool             `json:"usersLocked"` // 未登录：用户搜索被锁
}

// ------------------------------------------------------------
// 关键字 / 匹配条件
// ------------------------------------------------------------

// escapeLike 转义 LIKE 模式里的通配符，让用户输入的 % 和 _ 按字面量匹配。
// （否则搜一个 "%" 就等于「匹配所有文章」。）
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return s
}

// likeContains 生成「包含」匹配模式：%关键字%。
func likeContains(s string) string {
	return "%" + escapeLike(s) + "%"
}

// optionalUser 取当前登录用户；未登录返回 nil（不写 401 响应）。
// 与 parseAndLoadUser 的区别：这个不会中断请求，给「登录与否都能用、
// 但登录后结果更全」的公开接口用。
func optionalUser(c *gin.Context) *User {
	if u := authUser(c); u != nil {
		return u
	}
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil
	}
	u, err := userFromToken(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		return nil
	}
	return u
}

// parseSearchLimit 解析每类返回条数（默认 6，上限 30）。
func parseSearchLimit(c *gin.Context) int {
	n, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(searchDefaultLimit)))
	if err != nil || n < 1 {
		return searchDefaultLimit
	}
	if n > searchMaxLimit {
		return searchMaxLimit
	}
	return n
}

// buildPostFilter 组装文章筛选条件，返回 (WHERE 片段, 参数)。
// 三个条件都可以为空，为空就不参与筛选。
//
//	category —— 精确匹配分类
//	tag      —— 精确匹配「逗号分隔标签串」里的某一项
//	            （库里存的是规范化后的 "a, b, c"，先去掉 ", " 里的空格再套 ,…, 比对，
//	              这样搜 "Go" 不会命中 "Golang"）
//	keyword  —— 模糊匹配标题 / 摘要 / 分类 / 标签 / 公开短码，外加正文全文命中
//
// 只返回已发布文章。
func buildPostFilter(category, tag, keyword string) (string, []any) {
	where := []string{"status = 'published'"}
	args := []any{}

	if category != "" {
		where = append(where, "category = ?")
		args = append(args, category)
	}
	if tag != "" {
		where = append(where, `(',' || REPLACE(tags, ', ', ',') || ',') ILIKE ?`)
		args = append(args, "%,"+escapeLike(tag)+",%")
	}
	if keyword != "" {
		like := likeContains(keyword)
		parts := []string{"title ILIKE ?", "summary ILIKE ?", "category ILIKE ?", "tags ILIKE ?", "slug = ?"}
		args = append(args, like, like, like, like, keyword)

		// 正文命中：正文在文件系统里，先问 postcontent 要一批 ID，再拼成 id IN (?,?,…)
		if ids := postContent.SearchIDs(keyword); len(ids) > 0 {
			holders := make([]string, 0, len(ids))
			for _, id := range ids {
				holders = append(holders, "?")
				args = append(args, id)
			}
			parts = append(parts, "id IN ("+strings.Join(holders, ",")+")")
		}
		where = append(where, "("+strings.Join(parts, " OR ")+")")
	}

	return strings.Join(where, " AND "), args
}

// ------------------------------------------------------------
// 各类搜索
// ------------------------------------------------------------

// searchPosts 搜文章，返回 (当前页结果, 命中总数)。
// 排序：标题命中优先，其次按更新时间倒序（布尔值在 PostgreSQL 里可参与 ORDER BY，false < true）。
func searchPosts(keyword string, limit int) ([]PostListItem, int64) {
	where, args := buildPostFilter("", "", keyword)

	var total int64
	if err := q.QueryRow(`SELECT COUNT(*) FROM posts WHERE `+where, args...).Scan(&total); err != nil {
		return []PostListItem{}, 0
	}

	orderArgs := append(append([]any{}, args...), likeContains(keyword))
	orderArgs = append(orderArgs, limit)
	rows, err := q.Query(
		`SELECT `+postListFields+` FROM posts WHERE `+where+
			` ORDER BY (title ILIKE ?) DESC, updated_at DESC, id DESC LIMIT ?`,
		orderArgs...,
	)
	if err != nil {
		return []PostListItem{}, 0
	}
	defer rows.Close()
	items, err := scanListRows(rows)
	if err != nil {
		return []PostListItem{}, 0
	}
	return items, total
}

// searchCategories 搜分类（从已发布文章派生），按文章数倒序。
func searchCategories(keyword string, limit int) ([]SearchCategory, int64) {
	like := likeContains(keyword)

	var total int64
	if err := q.QueryRow(
		`SELECT COUNT(DISTINCT category) FROM posts
		 WHERE status = 'published' AND category <> '' AND category ILIKE ?`, like,
	).Scan(&total); err != nil {
		return []SearchCategory{}, 0
	}

	rows, err := q.Query(
		`SELECT category, COUNT(*) AS n FROM posts
		 WHERE status = 'published' AND category <> '' AND category ILIKE ?
		 GROUP BY category ORDER BY n DESC, category ASC LIMIT ?`, like, limit)
	if err != nil {
		return []SearchCategory{}, 0
	}
	defer rows.Close()

	items := []SearchCategory{}
	for rows.Next() {
		var it SearchCategory
		if err := rows.Scan(&it.Name, &it.Count); err != nil {
			return []SearchCategory{}, 0
		}
		items = append(items, it)
	}
	return items, total
}

// tagCounts 统计所有「已发布文章」用到的标签及出现次数。
// 标签在库里是逗号分隔字符串、没有独立表，所以只能取出来在内存里拆分统计
// （和 handleGetStats 的口径一致）；文章量级下这点开销可以忽略。
// 同一篇文章里重复写同一个标签只算一次。
func tagCounts() map[string]int64 {
	counts := map[string]int64{}
	rows, err := q.Query(`SELECT tags FROM posts WHERE status = 'published' AND tags <> ''`)
	if err != nil {
		return counts
	}
	defer rows.Close()

	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, t := range splitTags(raw) {
			if seen[t] {
				continue
			}
			seen[t] = true
			counts[t]++
		}
	}
	return counts
}

// sortTags 把标签统计整理成「次数多的在前，其次按名称」的稳定顺序。
func sortTags(counts map[string]int64) []SearchTag {
	items := make([]SearchTag, 0, len(counts))
	for name, n := range counts {
		items = append(items, SearchTag{Name: name, Count: n})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// searchTags 搜标签（按关键字做大小写不敏感的包含匹配）。
func searchTags(keyword string, limit int) ([]SearchTag, int64) {
	kw := strings.ToLower(keyword)
	all := sortTags(tagCounts())

	matched := []SearchTag{}
	for _, t := range all {
		if strings.Contains(strings.ToLower(t.Name), kw) {
			matched = append(matched, t)
		}
	}

	total := int64(len(matched))
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, total
}

// searchUsers 搜用户（需登录时才会被调用）。
// 只按「昵称（空则退回邮箱前缀）/ 用户名 / 9 位桃信 ID」匹配，**不匹配邮箱**，
// 也不返回邮箱字段。
func searchUsers(keyword string, selfID int64, limit int) ([]SearchUser, int64) {
	// 昵称为空时退回邮箱前缀，与全站展示规则保持一致
	const nicknameExpr = `COALESCE(NULLIF(nickname,''), SPLIT_PART(email,'@',1), '')`
	like := likeContains(keyword)

	var total int64
	if err := q.QueryRow(
		`SELECT COUNT(*) FROM users
		 WHERE (`+nicknameExpr+` ILIKE ? OR username ILIKE ? OR user_id = ?)`,
		like, like, keyword,
	).Scan(&total); err != nil {
		return []SearchUser{}, 0
	}

	rows, err := q.Query(
		`SELECT id, user_id, `+nicknameExpr+` AS nick, avatar, level, role FROM users
		 WHERE (`+nicknameExpr+` ILIKE ? OR username ILIKE ? OR user_id = ?)
		 ORDER BY (user_id = ?) DESC, (id = ?) DESC, id ASC
		 LIMIT ?`,
		like, like, keyword, keyword, selfID, limit,
	)
	if err != nil {
		return []SearchUser{}, 0
	}
	defer rows.Close()

	items := []SearchUser{}
	for rows.Next() {
		var u SearchUser
		if err := rows.Scan(&u.ID, &u.UserID, &u.Nickname, &u.Avatar, &u.Level, &u.Role); err != nil {
			return []SearchUser{}, 0
		}
		items = append(items, u)
	}
	return items, total
}

// ------------------------------------------------------------
// 接口
// ------------------------------------------------------------

// handleListTags 公开：全部标签及出现次数。GET /api/tags
// 顺序与搜索里的标签一致（次数多的在前）。搜索页「还没输入关键字」时用它铺标签云。
func handleListTags(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": sortTags(tagCounts())})
}

// handleSearch 公开：综合搜索。GET /api/search?q=…&limit=…
// 关键字为空时返回空结果（200），方便前端做「边打边搜」，不必先做空值分支。
func handleSearch(c *gin.Context) {
	keyword := cutRunes(trimSpace(c.Query("q")), searchKeywordMaxRunes)
	limit := parseSearchLimit(c)
	me := optionalUser(c) // 登录了就顺带把用户搜索一起做了；没登录则只锁「用户」这一类

	res := SearchResult{
		Q:           keyword,
		Posts:       []PostListItem{},
		Categories:  []SearchCategory{},
		Tags:        []SearchTag{},
		Users:       []SearchUser{},
		UsersLocked: me == nil,
	}
	if keyword == "" {
		c.JSON(http.StatusOK, res)
		return
	}

	res.Posts, res.Totals.Posts = searchPosts(keyword, limit)
	res.Categories, res.Totals.Categories = searchCategories(keyword, limit)
	res.Tags, res.Totals.Tags = searchTags(keyword, limit)

	if me != nil {
		res.Users, res.Totals.Users = searchUsers(keyword, me.ID, limit)
	}

	c.JSON(http.StatusOK, res)
}
