package postcontent

// ============================================================
// index.go —— 正文全文索引（给站内搜索用）
//
// 为什么需要它：正文已经搬到文件系统（见 store.go 顶部的说明），数据库里
// 只剩标题 / 摘要 / 分类 / 标签这些**轻量索引列**。所以「按标题找文章」走 SQL 就行，
// 但「搜正文里的字」必须真的去读文件。
//
// 做法：第一次搜索时把 root 下所有 .html 读一遍，剥掉 HTML 标签留纯文本，
// 建一张 id → 文本（小写）的内存表；之后每次搜索只做内存匹配。
// Save / Remove 会直接把索引标记为作废，下次搜索重建 —— 个人博客的文章量
// （几十到几千篇）重建一次是毫秒级，不值得为它引入更复杂的增量更新。
//
// 找不到文件 / 读失败都不会让搜索整体失败：跳过那一篇即可。
// ============================================================

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// maxIndexFileSize 单篇正文参与索引的体积上限（8 MB）。
// 超过就跳过：正常文章远小于它，真遇到超大文件也不该把内存吃光。
const maxIndexFileSize = 8 << 20

var (
	// scriptStyleRe 先整体干掉 <script> / <style>：它们的内容不是给人看的正文
	scriptStyleRe = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</\s*(script|style)\s*>`)
	// tagRe 其余标签一律换成空格（换成空串会把相邻词粘成一个词）
	tagRe = regexp.MustCompile(`(?s)<[^>]*>`)
	// spaceRe 连续空白压成一个空格
	spaceRe = regexp.MustCompile(`\s+`)
)

// htmlEntities 只处理最常见的几个实体。
// 不做完整的 HTML 反解：搜索是「包含匹配」，多留几个 &amp; 之类也不影响命中。
var htmlEntities = strings.NewReplacer(
	"&nbsp;", " ",
	"&amp;", "&",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", `"`,
	"&#39;", "'",
	"&#x27;", "'",
	"&hellip;", "…",
	"&mdash;", "—",
	"&ldquo;", "“",
	"&rdquo;", "”",
)

// HTMLToText 把正文 HTML 转成用于检索的纯文本（小写）。
// 只保留文字内容，标签、脚本、样式全部丢掉。
func HTMLToText(html string) string {
	s := scriptStyleRe.ReplaceAllString(html, " ")
	s = tagRe.ReplaceAllString(s, " ")
	s = htmlEntities.Replace(s)
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.ToLower(strings.TrimSpace(s))
}

// invalidateIndex 让索引作废（下次 SearchIDs 时重建）。
func (s *Store) invalidateIndex() {
	if s == nil {
		return
	}
	s.idxMu.Lock()
	s.idxBuilt = false
	s.idxDocs = nil
	s.idxMu.Unlock()
}

// InvalidateIndex 是 invalidateIndex 的导出形式。
// 给「整体恢复备份」用：那批正文文件是绕过 Save() 直接落到磁盘上的，
// 存储层自己察觉不到，必须由调用方显式通知。
func (s *Store) InvalidateIndex() { s.invalidateIndex() }

// buildIndex 遍历 root 下的正文文件，重建内存索引。调用方需自行持有写锁。
func (s *Store) buildIndex() map[int64]string {
	docs := map[int64]string{}
	if !s.Enabled() {
		return docs
	}
	_ = filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 单个目录/文件读不了就跳过，不影响整体
		}
		if d.IsDir() {
			// 跳过隐藏目录（.git 等）
			if path != s.root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), fileExt) {
			return nil
		}
		// 文件名就是文章 ID（见 RelPath）：<id>.html
		id, err := strconv.ParseInt(strings.TrimSuffix(d.Name(), fileExt), 10, 64)
		if err != nil || id < 1 {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxIndexFileSize {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		docs[id] = HTMLToText(splitFile(string(raw)))
		return nil
	})
	return docs
}

// SearchIDs 返回「正文里包含 keyword」的文章 ID 列表。
// keyword 为空、存储未启用、或索引为空时返回 nil（调用方据此跳过正文条件）。
// 匹配是大小写不敏感的「包含」。
func (s *Store) SearchIDs(keyword string) []int64 {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" || !s.Enabled() {
		return nil
	}

	s.idxMu.RLock()
	docs, built := s.idxDocs, s.idxBuilt
	s.idxMu.RUnlock()

	if !built {
		// 重建：先构建（不持锁读，构建期间其它 goroutine 仍可用旧索引/空索引）
		fresh := s.buildIndex()
		s.idxMu.Lock()
		// 双检：可能已经有别的 goroutine 建好了
		if !s.idxBuilt {
			s.idxDocs = fresh
			s.idxBuilt = true
		}
		docs = s.idxDocs
		s.idxMu.Unlock()
	}

	var ids []int64
	for id, text := range docs {
		if strings.Contains(text, keyword) {
			ids = append(ids, id)
		}
	}
	return ids
}
