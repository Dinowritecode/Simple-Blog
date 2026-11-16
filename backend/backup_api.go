package main

// ============================================================
// backup_api.go —— 备份 / 迁移的对外入口
//
// 两个入口共用 backup.go 里的实现：
//   · HTTP：后台「备份与迁移」页（管理员权限）
//       GET  /api/admin/backup/info    当前数据概览（各表行数 / 文件数 / 体积）
//       GET  /api/admin/backup/export  下载备份 zip（流式）
//       POST /api/admin/backup/import  上传备份并恢复（危险：会整体替换）
//   · 命令行：给「迁移到新机器」用 —— 那边往往还没法登录后台
//       blog-server --backup  out.zip
//       blog-server --restore in.zip --yes
// ============================================================

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// maxRestoreBytes 上传备份的体积上限。备份里含 uploads 时可能不小，
	// 但也不能不设上限：upload 会落成临时文件，没有上限就是拿磁盘赌运气。
	maxRestoreBytes = 2 << 30 // 2 GiB

	// restoreConfirmWord 恢复必须显式带上的确认词。
	// 「整体替换」会清空现有数据，加一道防手滑。
	restoreConfirmWord = "REPLACE"
)

// backupOverview 当前数据概览（后台页面进来时展示「这次会备份些什么」）。
type backupOverview struct {
	Tables      map[string]int64 `json:"tables"`
	TotalRows   int64            `json:"totalRows"`
	Posts       int              `json:"posts"`
	PostsBytes  int64            `json:"postsBytes"`
	Uploads     int              `json:"uploads"`
	UploadsByte int64            `json:"uploadsBytes"`
	DBName      string           `json:"dbName"`
	ContentDir  string           `json:"contentDir"`
	UploadDir   string           `json:"uploadDir"`
}

// parseParts 解析 parts=db,posts,uploads；缺省 = 全选。
func parseParts(raw string) backupParts {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "all" {
		return backupParts{DB: true, Posts: true, Uploads: true}
	}
	p := backupParts{}
	for _, seg := range strings.Split(raw, ",") {
		switch strings.TrimSpace(seg) {
		case "db":
			p.DB = true
		case "posts":
			p.Posts = true
		case "uploads":
			p.Uploads = true
		}
	}
	return p
}

// collectOverview 统计当前数据规模，供 info 接口与导出前提示使用。
func collectOverview() (*backupOverview, error) {
	ov := &backupOverview{Tables: map[string]int64{}, DBName: dbCfg.Name}
	tables, err := appTables()
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		var n int64
		if err := q.QueryRow(`SELECT COUNT(*) FROM ` + quoteIdent(t)).Scan(&n); err != nil {
			return nil, err
		}
		ov.Tables[t] = n
		ov.TotalRows += n
	}
	if postContent.Enabled() {
		ov.ContentDir = postContent.Root()
		files, size, err := collectFiles(postContent.Root(), "posts")
		if err != nil {
			return nil, err
		}
		ov.Posts, ov.PostsBytes = len(files), size
	}
	if abs, err := filepath.Abs(uploadDir); err == nil {
		ov.UploadDir = abs
	} else {
		ov.UploadDir = uploadDir
	}
	files, size, err := collectFiles(uploadDir, "uploads")
	if err != nil {
		return nil, err
	}
	ov.Uploads, ov.UploadsByte = len(files), size
	return ov, nil
}

// handleBackupInfo 管理员：当前数据概览。GET /api/admin/backup/info
func handleBackupInfo(c *gin.Context) {
	ov, err := collectOverview()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "统计失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, ov)
}

// handleBackupExport 管理员：下载备份。GET /api/admin/backup/export?parts=db,posts,uploads
//
// 直接往响应里流式写 zip：备份可能到 GB 级，先落临时文件再发会白白占一份磁盘。
// 代价是「写到一半出错」时 HTTP 状态已经发出去了 —— zip 每个条目都带 CRC32，
// 坏包在导入侧会被校验出来，不会静默恢复半份数据。
func handleBackupExport(c *gin.Context) {
	parts := parseParts(c.Query("parts"))
	if !parts.any() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少要选择一项备份内容"})
		return
	}

	name := "rinblog-backup-" + time.Now().Format("20060102-150405") + ".zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Header("Cache-Control", "no-store")

	start := time.Now()
	manifest, err := writeBackup(c.Writer, parts)
	if err != nil {
		// 响应头已经发出，改不了状态码了，只能记日志（前端拿到的是坏包）
		log.Printf("备份导出失败：%v", err)
		return
	}
	log.Printf("备份导出完成：%s（耗时 %s，表 %d 张 / 正文 %d 篇 / 上传 %d 个）",
		name, time.Since(start).Round(time.Millisecond), len(manifest.Tables), manifest.Posts, manifest.Uploads)
}

// handleBackupImport 管理员：上传备份并恢复。
// POST /api/admin/backup/import   multipart: file=<zip>, confirm=REPLACE
//
// 这是**破坏性**操作：现有数据会被整体替换。多道保护：
//  1. confirm 必须精确等于 REPLACE
//  2. 备份先完整校验（格式 / 版本 / manifest），再解到暂存目录
//  3. 数据库那一步在单事务里，失败全部回滚
func handleBackupImport(c *gin.Context) {
	// 确认词按 form 字段收；也兼容放在查询串里（curl / 脚本调用时更顺手）
	confirm := strings.TrimSpace(c.PostForm("confirm"))
	if confirm == "" {
		confirm = strings.TrimSpace(c.Query("confirm"))
	}
	if confirm != restoreConfirmWord {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "确认词不正确。恢复会清空现有数据，请在确认框里输入 " + restoreConfirmWord,
		})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRestoreBytes)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有收到备份文件（或文件超过 2 GiB）"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传 .zip 备份文件"})
		return
	}

	// 存到临时文件：zip 需要随机读取中央目录，不能边收边解。
	tmp, err := os.CreateTemp("", "blog-restore-*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时文件失败"})
		return
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := c.SaveUploadedFile(fileHeader, tmpPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败"})
		return
	}

	start := time.Now()
	report, err := restoreBackup(tmpPath)
	if err != nil {
		log.Printf("备份恢复失败：%v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "恢复失败：" + err.Error()})
		return
	}
	log.Printf("备份恢复完成：表 %d 张 / 正文 %d 篇 / 上传 %d 个（耗时 %s）",
		len(report.Tables), report.Posts, report.Uploads, time.Since(start).Round(time.Millisecond))
	c.JSON(http.StatusOK, gin.H{"ok": true, "report": report})
}

// ------------------------------------------------------------
// 控制台命令：backup / restore
//
// 为什么要在控制台里也留一份：迁移到新机器的顺序通常是
// 「装二进制 → 建库 → 灌数据 → 起服务」，这时候既没有后台可点，
// 新库上也还没有管理员账号可用，命令行是唯一顺手的入口。
//
//   backup [文件] [parts]     默认写 backup-<时间戳>.zip，parts 默认 db,posts,uploads
//   restore <文件> [REPLACE]  交互模式下会再问一次确认词
// ------------------------------------------------------------

// cmdBackup 导出备份。backup [文件] [parts]
func cmdBackup(args []string) bool {
	out := ""
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		out = strings.TrimSpace(args[0])
	} else {
		out = "backup-" + time.Now().Format("20060102-150405") + ".zip"
	}
	partsRaw := "db,posts,uploads"
	if len(args) > 1 {
		partsRaw = args[1]
	}
	parts := parseParts(partsRaw)
	if !parts.any() {
		fmt.Println("❌ parts 至少要包含 db / posts / uploads 之一")
		return false
	}
	if !strings.HasSuffix(strings.ToLower(out), ".zip") {
		out += ".zip"
	}

	f, err := os.Create(out)
	if err != nil {
		fmt.Printf("❌ 创建备份文件失败: %v\n", err)
		return false
	}
	defer f.Close()

	start := time.Now()
	manifest, err := writeBackup(f, parts)
	if err != nil {
		fmt.Printf("❌ 备份失败: %v\n", err)
		return false
	}
	if err := f.Sync(); err != nil {
		fmt.Printf("❌ 写入磁盘失败: %v\n", err)
		return false
	}
	var size int64
	if info, err := f.Stat(); err == nil {
		size = info.Size()
	}

	abs, _ := filepath.Abs(out)
	fmt.Println("✅ 备份完成")
	fmt.Printf("   文件：%s\n", abs)
	fmt.Printf("   内容：%s\n", strings.Join(manifest.Parts, " + "))
	fmt.Printf("   规模：表 %d 张 / 正文 %d 篇 / 上传 %d 个 / 共 %s\n",
		len(manifest.Tables), manifest.Posts, manifest.Uploads, humanBytes(size))
	fmt.Printf("   耗时：%s\n", time.Since(start).Round(time.Millisecond))
	return true
}

// cmdRestore 从备份恢复。restore <文件> [REPLACE]
func cmdRestore(scanner *bufio.Scanner, args []string) bool {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		fmt.Println("用法: restore <备份文件.zip>")
		return false
	}
	path := strings.TrimSpace(args[0])
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("❌ 找不到备份文件: %v\n", err)
		return false
	}

	// 先读 manifest：既能提前发现「这不是本系统的备份」，也顺便把要恢复的内容
	// 摊在用户面前，让人知道自己按下的这个键会做什么。
	manifest, err := readManifest(path)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return false
	}
	fmt.Println("即将从以下备份恢复：")
	fmt.Printf("   备份时间：%s\n", manifest.CreatedAt)
	fmt.Printf("   内容：%s\n", strings.Join(manifest.Parts, " + "))
	fmt.Printf("   规模：表 %d 张 / 正文 %d 篇 / 上传 %d 个\n",
		len(manifest.Tables), manifest.Posts, manifest.Uploads)
	fmt.Println("⚠️  这会**清空并整体替换**当前数据库与文件，且不可撤销。")

	// 确认：交互模式问一次；一次性命令模式要求把确认词写在参数里
	if scanner != nil {
		fmt.Printf("确认请输入 %s（其它任何输入都会取消）：", restoreConfirmWord)
		if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != restoreConfirmWord {
			fmt.Println("已取消。")
			return false
		}
	} else if len(args) < 2 || strings.TrimSpace(args[1]) != restoreConfirmWord {
		fmt.Printf("❌ 非交互模式需要显式确认：restore %s %s\n", path, restoreConfirmWord)
		return false
	}

	start := time.Now()
	report, err := restoreBackup(path)
	if err != nil {
		fmt.Printf("❌ 恢复失败: %v\n", err)
		return false
	}
	fmt.Println("✅ 恢复完成")
	fmt.Printf("   导入：%d 张表", len(report.Tables))
	for _, t := range orderTables(keysOf(report.Tables)) {
		fmt.Printf(" / %s %d", t, report.Tables[t])
	}
	fmt.Println()
	fmt.Printf("   文件：正文 %d 篇 / 上传 %d 个\n", report.Posts, report.Uploads)
	if len(report.Missing) > 0 {
		fmt.Printf("   ⚠️  备份与本机对不上的表：%s\n", strings.Join(report.Missing, "、"))
	}
	fmt.Printf("   耗时：%s\n", time.Since(start).Round(time.Millisecond))
	return true
}

// keysOf 取 map 的键（用于按依赖顺序打印表）。
func keysOf[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// humanBytes 把字节数格式化成人看的单位。
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	val := float64(n)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		val /= unit
		if val < unit {
			return fmt.Sprintf("%.1f %s", val, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", val/unit)
}
