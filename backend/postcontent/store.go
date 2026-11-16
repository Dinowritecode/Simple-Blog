// Package postcontent 是「文章正文」的文件系统存储层。
//
// 为什么把正文从数据库搬到文件：
//   · 正文是 wangEditor 产出的大段 HTML（LONGTEXT），占 posts 表的绝大部分体积，
//     但它**从来不参与检索** —— 列表查询只按 title/summary/category/tags/status 过滤，
//     没有任何一处 LIKE content。留在库里纯粹是让每次 SELECT 都拖着大字段。
//   · 搬到文件后：数据库只留轻量索引列（列表/筛选/分页照旧走 SQL），
//     正文按需读取；同时正文天然可被 git 版本管理、可离线编辑。
//
// 存储约定：
//   路径   <root>/<created_at 的 年>/<月>/<id>.html
//   内容   <!--post:{"id":1,...}-->  首部元数据（JSON，便于以后加字段时平滑兼容）
//          <正文 HTML>
//   写入   先写同目录临时文件 → 同步落盘 → rename 原子替换（避免写一半损坏正文）
//
// 依赖方向：本包只依赖标准库，不依赖 main 包（main 通过 postcontent.Store 使用它）。
package postcontent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrNotFound 该文章的正文文件不存在。
var ErrNotFound = errors.New("postcontent: 正文文件不存在")

const (
	headerPrefix = "<!--post:"
	headerSuffix = "-->"
	// fileExt 正文文件后缀。存的是 wangEditor 的 HTML，所以用 .html
	//（用编辑器/浏览器直接打开也能看）。
	fileExt = ".html"
)

// Meta 正文文件首部的元数据。
//
// 这些字段在 MariaDB 的 posts 表里同样有一份（作为列表/筛选用的索引），
// 写进文件首部是为了让正文文件**自描述**：单独把 data/posts 拷走也能还原文章。
// 读取时以数据库为准，首部只作参考与灾备。
type Meta struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Tags      string `json:"tags"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Version   int    `json:"v"` // 文件格式版本，便于以后调整首部结构
}

// formatVersion 当前的文件格式版本。
const formatVersion = 1

// Store 正文文件存储。
type Store struct {
	root string

	// 正文全文索引（惰性构建，见 index.go）：
	// Store 的零值/新建实例都可用；只有真正搜索时才会去读文件建索引。
	idxMu    sync.RWMutex
	idxDocs  map[int64]string // 文章 ID → 正文纯文本（已转小写）
	idxBuilt bool
}

// New 在 root 目录下创建（或复用）正文存储。
// root 为空时返回 nil，调用方据此判断「未启用文件存储」并回落到数据库正文列。
func New(root string) (*Store, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析正文目录失败: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("创建正文目录失败: %w", err)
	}
	return &Store{root: abs}, nil
}

// Root 返回正文根目录的绝对路径。
func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// Enabled 是否可用（nil 接收者安全，方便调用方直接 s.Enabled()）。
func (s *Store) Enabled() bool { return s != nil && s.root != "" }

// RelPath 计算某篇文章正文的「相对路径」（相对 root）。
// 按年/月分目录，避免单个目录堆上万文件（Windows 资源管理器与 ext4 都会变慢）。
// createdAt 支持 "2006-01-02 15:04:05"，解析失败则落到 unknown 目录。
func RelPath(id int64, createdAt string) string {
	t, err := parseTime(createdAt)
	if err != nil {
		return filepath.Join("unknown", fmt.Sprintf("%d%s", id, fileExt))
	}
	return filepath.Join(t.Format("2006"), t.Format("01"), fmt.Sprintf("%d%s", id, fileExt))
}

// Path 返回某篇文章正文文件的绝对路径。
func (s *Store) Path(id int64, createdAt string) string {
	return filepath.Join(s.root, RelPath(id, createdAt))
}

// Load 读取正文 HTML。文件不存在返回 ErrNotFound。
func (s *Store) Load(id int64, createdAt string) (string, error) {
	if !s.Enabled() {
		return "", ErrNotFound
	}
	raw, err := os.ReadFile(s.Path(id, createdAt))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return splitFile(string(raw)), nil
}

// Save 原子写入正文（自动创建年月目录）。
func (s *Store) Save(m Meta, content string) error {
	if !s.Enabled() {
		return errors.New("postcontent: 存储未启用")
	}
	m.Version = formatVersion
	head, err := json.Marshal(m)
	if err != nil {
		return err
	}

	path := s.Path(m.ID, m.CreatedAt)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	body := headerPrefix + string(head) + headerSuffix + "\n" + content
	// 末尾补一个换行：让文件以 newline 结束（git / 编辑器都更友好）
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}

	tmp, err := os.CreateTemp(dir, fmt.Sprintf(".%d-*.tmp", m.ID))
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// 任何失败路径都要把临时文件清掉，避免目录里堆垃圾
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := replaceFile(tmpName, path); err != nil {
		return err
	}
	tmpName = "" // 已 rename，不需要再删
	s.invalidateIndex() // 正文变了，全文索引作废（下次搜索时重建）
	return nil
}

// Remove 删除正文文件（不存在视为成功，方便重复调用）。
func (s *Store) Remove(id int64, createdAt string) error {
	if !s.Enabled() {
		return nil
	}
	path := s.Path(id, createdAt)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// 顺手清掉写坏时可能残留的临时文件
	if matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), fmt.Sprintf(".%d-*.tmp", id))); err == nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
	s.invalidateIndex() // 正文没了，全文索引作废
	return nil
}

// Meta 读取文件首部的元数据（文件缺失或首部损坏时返回零值 + error）。
// 注意：正常业务流程以数据库为权威，本方法主要给迁移/巡检工具用。
func (s *Store) ReadMeta(id int64, createdAt string) (Meta, error) {
	var m Meta
	if !s.Enabled() {
		return m, ErrNotFound
	}
	raw, err := os.ReadFile(s.Path(id, createdAt))
	if errors.Is(err, os.ErrNotExist) {
		return m, ErrNotFound
	}
	if err != nil {
		return m, err
	}
	head, ok := headerOf(string(raw))
	if !ok {
		return m, errors.New("postcontent: 正文文件缺少元数据首部")
	}
	if err := json.Unmarshal([]byte(head), &m); err != nil {
		return m, fmt.Errorf("postcontent: 元数据无法解析: %w", err)
	}
	return m, nil
}

// splitFile 把文件内容拆成「正文」：丢掉首部元数据行，并去掉写入时补的那个结尾换行。
func splitFile(raw string) string {
	// 去掉 UTF-8 BOM（手工用记事本编辑过的文件可能有）
	raw = strings.TrimPrefix(raw, "\uFEFF")
	if _, ok := headerOf(raw); ok {
		if idx := strings.Index(raw, headerSuffix); idx >= 0 {
			body := raw[idx+len(headerSuffix):]
			// 首部行后紧跟的那个换行不属于正文
			body = strings.TrimPrefix(body, "\r\n")
			body = strings.TrimPrefix(body, "\n")
			// Save 会在末尾补一个换行（让文件以 newline 结束），读回时去掉，
			// 保证 Save → Load 是严格可逆的。
			return strings.TrimSuffix(body, "\n")
		}
	}
	// 没有首部（比如手工丢进来的 html）：整个文件就是正文
	return raw
}

// headerOf 取出首部的 JSON 字符串。
func headerOf(raw string) (string, bool) {
	raw = strings.TrimPrefix(raw, "\uFEFF")
	if !strings.HasPrefix(raw, headerPrefix) {
		return "", false
	}
	idx := strings.Index(raw, headerSuffix)
	if idx < 0 {
		return "", false
	}
	return raw[len(headerPrefix):idx], true
}

// parseTime 解析项目里统一的 "2006-01-02 15:04:05" 时间串（也容忍 RFC3339）。
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("空时间")
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}
