package postcontent

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPathFormula 验证路径公式与真实落盘位置一致。
func TestPathFormula(t *testing.T) {
	got := RelPath(4, "2026-09-25 19:14:56")
	want := filepath.Join("2026", "09", "4.html")
	if got != want {
		t.Fatalf("RelPath = %q, want %q", got, want)
	}
}

// TestParseTime 验证项目里的时间串能被解析（解析失败会落到 unknown 目录）。
func TestParseTime(t *testing.T) {
	if _, err := parseTime("2026-09-25 19:14:56"); err != nil {
		t.Fatalf("解析 MySQL DATETIME 串失败: %v", err)
	}
}

// TestSaveLoadRoundTrip 验证写入后能按内容读回。
func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	content := "<p>这是一篇用于预览卡片布局的测试文章。</p>"
	m := Meta{ID: 4, Slug: "testcard02", Title: "Git 使用教程", CreatedAt: "2026-09-25 19:14:56"}

	if err := s.Save(m, content); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// 文件应该落在 2026/09/4.html
	p := s.Path(4, m.CreatedAt)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("文件未落在预期路径 %s: %v", p, err)
	}

	got, err := s.Load(4, m.CreatedAt)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got != content {
		t.Fatalf("Load 内容不一致:\n got=%q\nwant=%q", got, content)
	}

	// 首部元数据也应能读回
	rm, err := s.ReadMeta(4, m.CreatedAt)
	if err != nil {
		t.Fatalf("ReadMeta 失败: %v", err)
	}
	if rm.Slug != "testcard02" || rm.Title != "Git 使用教程" {
		t.Fatalf("ReadMeta 内容不对: %+v", rm)
	}
}

// TestLoadAgainstRealFile 用真实生成的文件验证读取（定位线上为空的原因）。
func TestLoadAgainstRealFile(t *testing.T) {
	real := `D:\website\data\posts`
	if _, err := os.Stat(real); err != nil {
		t.Skip("真实正文目录不存在，跳过")
	}
	s, err := New(real)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := "2026-09-25 19:14:56"
	t.Logf("推导路径: %s", s.Path(4, createdAt))
	text, err := s.Load(4, createdAt)
	if err != nil {
		t.Fatalf("Load(4) 失败: %v", err)
	}
	t.Logf("读到正文: %q", text)
	if text == "" {
		t.Fatalf("正文为空 —— 读取路径与落盘路径不一致")
	}
}
