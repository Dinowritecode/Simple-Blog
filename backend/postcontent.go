package main

// ============================================================
// postcontent.go —— 文章正文「存哪」的统一出入口
//
// 改造后的存储分工：
//   posts 表        —— 列表 / 筛选 / 分页用的轻量索引列（标题、摘要、封面、
//                      分类、标签、状态、计数、时间）
//   POST_CONTENT_DIR —— 正文 HTML（每篇一个文件，见 postcontent 包）
//
// 「数据库优先、文件兜底」的读策略（loadPostContent）：
//   扫描时 content 列用 sql.NullString 承接：
//     · 非 NULL  → 还是老数据（或文件存储未启用），直接用数据库里的正文
//     · NULL     → 说明正文已迁到文件，去文件里读
//   这样改造可以分批进行：已迁移的读文件，没迁移的读数据库，中间状态也不会出错。
//
// 写策略（savePostContent）：
//   启用文件存储时写文件，并把数据库的 content 列置为 NULL（不再重复存一份）。
//   写文件放在「数据库写入之前」：万一文件写失败，数据库不会先被改掉。
// ============================================================

import (
	"context"
	"database/sql"
	"log"
	"time"

	"rineyce/blog/postcontent"
)

// postMeta 从 Post 组装文件首部元数据。
func postMeta(p *Post) postcontent.Meta {
	return postcontent.Meta{
		ID:        p.ID,
		Slug:      p.Slug,
		Title:     p.Title,
		Category:  p.Category,
		Tags:      p.Tags,
		Status:    p.Status,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

// loadPostContent 取出某篇文章的正文。
// 返回的第二个值表示「是从文件读的」（调用方一般不需要，测试与日志用）。
//
// 判定规则：content 列为 NULL 或空串都视为「正文不在数据库里」——
// 空串是历史遗留（早期迁移把 NULL 写成了 ''），把它当空正文会让文章显示不出内容。
func loadPostContent(p *Post) (string, bool) {
	if p.Content != "" {
		return p.Content, false // 数据库里还有正文（老数据 / 未启用文件存储）
	}
	if !postContent.Enabled() {
		return "", false
	}
	text, err := postContent.Load(p.ID, p.CreatedAt)
	if err == nil {
		return text, true
	}
	// 文件缺失不是致命错误：文章仍然可以打开，只是正文为空
	log.Printf("文章正文：读取文件失败（id=%d, created_at=%s）：%v", p.ID, p.CreatedAt, err)
	return "", false
}

// savePostContent 把正文写到文件（若启用），并返回「数据库 content 列应该写什么值」。
//
// 返回的 dbValue 有三种情况：
//   · 启用文件存储：返回 nil，表示把 content 列置为 NULL（正文只在文件里）
//   · 未启用：返回正文本身，继续写数据库
func savePostContent(p *Post, content string) (dbValue any) {
	if !postContent.Enabled() {
		return content
	}
	if err := postContent.Save(postMeta(p), content); err != nil {
		// 文件写失败：退回把正文写数据库，至少不丢内容（并在日志里留痕）
		log.Printf("文章正文：写入文件失败（id=%d），已退回数据库存储：%v", p.ID, err)
		return content
	}
	return nil
}

// removePostContent 删除正文文件（删除文章时调用）。
func removePostContent(p *Post) {
	if !postContent.Enabled() {
		return
	}
	if err := postContent.Remove(p.ID, p.CreatedAt); err != nil {
		log.Printf("文章正文：删除文件失败（id=%d）：%v", p.ID, err)
	}
}

// scanNullContent 把 sql.NullString 归一到 Post.Content（NULL → 空串）。
func scanNullContent(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// ensureContentColumnNullable 确保 posts.content 列允许 NULL。
//
// 背景：最初的建表语句里 content 是 `NOT NULL`（正文必定在库里）。
// 改成文件存储后，「正文已迁出」用 NULL 表示，所以必须去掉这个约束。
// PostgreSQL 的 DROP NOT NULL 是幂等的（列本来就可空时也不报错）。
func ensureContentColumnNullable() error {
	if !postContent.Enabled() {
		return nil
	}
	if _, err := q.Exec(`ALTER TABLE posts ALTER COLUMN content DROP NOT NULL`); err != nil {
		return err
	}
	return nil
}

// migratePostToFile 把一篇文章的正文从数据库搬到文件，成功后把 content 列置为 NULL。
func migratePostToFile(p *Post) error {
	text := p.Content
	if err := postContent.Save(postMeta(p), text); err != nil {
		return err
	}
	// 文件写好之后再清空数据库列。必须写 NULL 而不是空串：
	// 读取侧靠「content 为 NULL/空」判断正文是否在文件里，置空串虽然也能工作，
	// 但 NULL 语义更准确，也让「正文是否已迁出」一眼可查（IS NULL）。
	if _, err := q.Exec(`UPDATE posts SET content = NULL WHERE id = ?`, p.ID); err != nil {
		return err
	}
	log.Printf("文章正文已迁出数据库：id=%d《%s》%d 字节", p.ID, p.Title, len(text))
	return nil
}

// MigrateAllPostsToFiles 把所有仍在数据库里的正文迁到文件（后台命令 / 启动时可用）。
// 返回迁移的篇数。幂等：已经迁过的（content 为空且文件存在）会跳过。
func MigrateAllPostsToFiles() (int, error) {
	if !postContent.Enabled() {
		return 0, nil
	}
	rows, err := q.Query(
		`SELECT ` + postDetailFields + ` FROM posts ORDER BY id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		var (
			p  Post
			ns sql.NullString
		)
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Summary, &p.Cover, &p.Category, &p.Tags,
			&ns, &p.Status, &p.Likes, &p.Views, &p.Comments, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return 0, err
		}
		p.Content = scanNullContent(ns)
		posts = append(posts, &p)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	moved := 0
	for _, p := range posts {
		if p.Content == "" {
			// 空正文：可能是「本来就没写正文」，也可能是历史迁移把 NULL 写成了空串。
			// 两种情况都只需把列归一成 NULL（读取侧靠 NULL/空判断正文在文件里）。
			if _, err := q.Exec(`UPDATE posts SET content = NULL WHERE id = ? AND content = ''`, p.ID); err != nil {
				log.Printf("归一化文章 %d 的 content 列失败：%v", p.ID, err)
			}
			continue
		}
		if err := migratePostToFile(p); err != nil {
			log.Printf("迁移文章 %d 失败：%v", p.ID, err)
			continue
		}
		moved++
	}
	return moved, nil
}

// migratePostsOnStart 启动时的自动迁移：只在「确实有正文还在数据库里」时执行，
// 避免每次启动都扫全表（迁移本身幂等，重复跑也只是空转）。
func migratePostsOnStart() {
	if !postContent.Enabled() {
		return
	}
	// 先把列改成可空（老库是 NOT NULL），否则下一步无法写 NULL
	if err := ensureContentColumnNullable(); err != nil {
		log.Printf("文章正文：调整 content 列可空失败（仍可按数据库方式工作）：%v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// 统计「内容还留在数据库列里」的行数：
	// 非空串 = 待迁移；空串 = 只待归一化成 NULL（也要进一次迁移流程）。
	// 注意不能用 `content <> ''` 过滤，否则空串永远刷不成 NULL。
	var pending int
	err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM posts WHERE content IS NOT NULL`).Scan(&pending)
	if err != nil || pending == 0 {
		return
	}
	log.Printf("文章正文：发现 %d 篇的 content 列还留在数据库里，开始处理…", pending)
	moved, err := MigrateAllPostsToFiles()
	if err != nil {
		log.Printf("文章正文自动迁移失败：%v", err)
		return
	}
	log.Printf("文章正文：自动迁移完成，共 %d 篇", moved)
}
