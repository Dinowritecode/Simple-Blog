package main

// ============================================================
// backup.go —— 数据备份 / 迁移
//
// 归档格式（一个 zip 文件，自描述、可离线保存）：
//
//	manifest.json              本次备份的元信息（格式版本 / 时间 / 各表行数 / 文件数）
//	db/<表名>.ndjson           每张表一条一行 JSON（NDJSON）
//	posts/<年>/<月>/<id>.html  文章正文，原样保存（目录结构与磁盘一致）
//	uploads/...                上传的图片与头像，原样保存
//
// 为什么用 NDJSON 而不是拼一份 .sql：
//   · 不用写 SQL 转义（正文里单引号、分号、换行都有，手拼字面量是最容易出安全
//     问题的地方）；JSON 编码器把这件事做得比手写更可靠。
//   · 可以直接流式读写：导出时一边查一边写 zip，导入时一边读一边插，不把整库
//     读进内存。
//   · 每个值都是「有类型的」：数字仍是数字、布尔仍是布尔，不用靠字符串往返。
//
// 恢复策略是「整体替换」而不是合并：
//   一个事务里先 TRUNCATE 掉全部业务表、再按备份逐表插入，最后把自增序列推到
//   备份里的最大值。这样才能保证「恢复完 = 备份时那一刻」，而不是新旧数据混在
//   一起。文件部分不参与事务，所以先把它们解到暂存目录，数据库提交成功后再搬
//   过去 —— 数据库失败时磁盘一个字节都不会动。
// ============================================================

import (
	"archive/zip"
	"bufio"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// backupFormat / backupFormatVersion 用来挡住「拿别的压缩包来恢复」这种情况。
	// 以后归档结构有破坏性改动时把 version 加一，导入侧据此明确拒绝。
	backupFormat        = "rineyce-blog-backup"
	backupFormatVersion = 1

	// backupManifestName manifest 必须是 zip 里的第一个条目：
	// 导出是流式的，只有第一个条目能在写数据之前把元信息写出去。
	backupManifestName = "manifest.json"

	insertBatchRows = 200 // 每 INSERT 攒多少行（多行 VALUES，减少往返）
	maxBackupFiles  = 200000
)

// backupParts 选择备份哪些部分（后台的勾选项）。
type backupParts struct {
	DB      bool
	Posts   bool
	Uploads bool
}

func (p backupParts) any() bool { return p.DB || p.Posts || p.Uploads }

// BackupManifest 归档里的 manifest.json。
type BackupManifest struct {
	Format        string           `json:"format"`
	FormatVersion int              `json:"formatVersion"`
	CreatedAt     string           `json:"createdAt"`
	Parts         []string         `json:"parts"`  // db / posts / uploads
	Tables        map[string]int64 `json:"tables"` // 表名 → 行数（仅 DB 部分）
	Posts         int              `json:"posts"`  // 正文文件数
	Uploads       int              `json:"uploads"`
	Bytes         int64            `json:"bytes"` // 文件部分（正文 + 上传）总字节
}

// RestoreReport 恢复完成后返回给调用方的概览。
type RestoreReport struct {
	Manifest BackupManifest   `json:"manifest"`
	Tables   map[string]int64 `json:"tables"`  // 实际导入的行数
	Posts    int              `json:"posts"`   // 实际落盘的正文件数
	Uploads  int              `json:"uploads"` // 实际落盘的上传文件数
	Missing  []string         `json:"missing"` // 备份与本机对不上的表（见 restoreDB）
}

// ------------------------------------------------------------
// 表 / 列 的发现
// ------------------------------------------------------------

type columnInfo struct {
	Name     string
	DataType string
	Serial   bool
}

// appTables 列出需要备份的业务表（按名称排序）。
//
// 刻意**排除分区子表**（messages_p202609 之类）：
//
//	· 它们的数据由父表 messages 统一读写，父表已经有全部行；
//	· 一起导出会重复一遍，导入时又会和 TRUNCATE ... CASCADE 打架。
//
// 判据是「不是任何表的继承子表」（pg_inherits）。
func appTables() ([]string, error) {
	rows, err := q.Query(`
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		  AND c.relkind IN ('r', 'p')
		  AND NOT EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhrelid = c.oid)
		ORDER BY c.relname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// tableColumns 读取某张表的列（按定义顺序），并标出哪些是自增列。
func tableColumns(table string) ([]columnInfo, error) {
	rows, err := q.Query(`
		SELECT column_name, data_type, COALESCE(column_default, '')
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = ?
		ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []columnInfo
	for rows.Next() {
		var c columnInfo
		var def string
		if err := rows.Scan(&c.Name, &c.DataType, &def); err != nil {
			return nil, err
		}
		// serial / identity 列：默认值是 nextval('...')
		c.Serial = strings.HasPrefix(def, "nextval(")
		out = append(out, c)
	}
	return out, rows.Err()
}

// ------------------------------------------------------------
// 导出
// ------------------------------------------------------------

// fileEntry 一个待备份的文件。
type fileEntry struct {
	Abs  string // 磁盘绝对路径
	Zip  string // zip 里的条目名（用 / 分隔）
	Size int64
}

// collectFiles 递归收集目录下的文件，跳过隐藏文件 / 目录与写入中的临时文件。
// prefix 是归档里的目录名（posts / uploads）。
func collectFiles(root, prefix string) ([]fileEntry, int64, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, 0, nil
	}
	if _, err := os.Stat(root); err != nil {
		return nil, 0, nil // 目录不存在 = 这部分本来就是空的
	}

	var out []fileEntry
	var total int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 单个文件读不了不影响整体备份
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".tmp") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		out = append(out, fileEntry{
			Abs:  path,
			Zip:  prefix + "/" + filepath.ToSlash(rel),
			Size: info.Size(),
		})
		total += info.Size()
		if len(out) > maxBackupFiles {
			return fmt.Errorf("文件数量超过上限（%d），已中止备份", maxBackupFiles)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Zip < out[j].Zip })
	return out, total, err
}

// writeBackup 把备份写成 zip 流。w 可以是 http.ResponseWriter（流式下载），
// 也可以是文件。返回写出的 manifest，供调用方记录/展示。
func writeBackup(w io.Writer, parts backupParts) (*BackupManifest, error) {
	if !parts.any() {
		return nil, fmt.Errorf("至少要选择一项备份内容")
	}

	manifest := &BackupManifest{
		Format:        backupFormat,
		FormatVersion: backupFormatVersion,
		CreatedAt:     time.Now().Format("2006-01-02 15:04:05"),
		Tables:        map[string]int64{},
	}
	if parts.DB {
		manifest.Parts = append(manifest.Parts, "db")
	}
	if parts.Posts {
		manifest.Parts = append(manifest.Parts, "posts")
	}
	if parts.Uploads {
		manifest.Parts = append(manifest.Parts, "uploads")
	}

	// ---------- 先把「要写什么」统计出来 ----------
	// manifest 是第一个条目，必须在开始写数据之前就拿到全部计数。
	var tables []string
	if parts.DB {
		var err error
		tables, err = appTables()
		if err != nil {
			return nil, fmt.Errorf("读取表列表失败: %w", err)
		}
		for _, t := range tables {
			var n int64
			if err := q.QueryRow(`SELECT COUNT(*) FROM ` + quoteIdent(t)).Scan(&n); err != nil {
				return nil, fmt.Errorf("统计表 %s 失败: %w", t, err)
			}
			manifest.Tables[t] = n
		}
	}

	var postFiles, uploadFiles []fileEntry
	if parts.Posts && postContent.Enabled() {
		files, size, err := collectFiles(postContent.Root(), "posts")
		if err != nil {
			return nil, err
		}
		postFiles = files
		manifest.Posts = len(files)
		manifest.Bytes += size
	}
	if parts.Uploads {
		files, size, err := collectFiles(uploadDir, "uploads")
		if err != nil {
			return nil, err
		}
		uploadFiles = files
		manifest.Uploads = len(files)
		manifest.Bytes += size
	}

	// ---------- 开始写 zip ----------
	zw := zip.NewWriter(w)

	// 1) manifest（必须是第一个）
	head, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeZipEntry(zw, backupManifestName, strings.NewReader(string(head)+"\n")); err != nil {
		return nil, err
	}

	// 2) 数据库
	for _, t := range tables {
		if err := writeTableNDJSON(zw, t); err != nil {
			return nil, err
		}
	}

	// 3) 文件（原样搬运，不压缩：图片和 HTML 压不动多少，还白费 CPU）
	for _, f := range append(postFiles, uploadFiles...) {
		if err := writeZipFileStored(zw, f); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return manifest, nil
}

// writeZipEntry 写一个小体积的文本条目（Deflate）。
func writeZipEntry(zw *zip.Writer, name string, r io.Reader) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}

// writeZipFileStored 把一个磁盘文件原样写进 zip（不压缩）。
func writeZipFileStored(zw *zip.Writer, f fileEntry) error {
	src, err := os.Open(f.Abs)
	if err != nil {
		// 备份过程中文件被删了：跳过这一条，不让整个备份失败
		return nil
	}
	defer src.Close()

	hdr := &zip.FileHeader{Name: f.Zip, Method: zip.Store, Modified: time.Now()}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	return err
}

// writeTableNDJSON 把一张表的所有行写成 NDJSON。
func writeTableNDJSON(zw *zip.Writer, table string) error {
	cols, err := tableColumns(table)
	if err != nil {
		return fmt.Errorf("读取表 %s 的列失败: %w", table, err)
	}
	w, err := zw.Create("db/" + table + ".ndjson")
	if err != nil {
		return err
	}

	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	query := `SELECT ` + strings.Join(quoteIdentAll(names), ", ") + ` FROM ` + quoteIdent(table)

	rows, err := q.Query(query)
	if err != nil {
		return fmt.Errorf("导出表 %s 失败: %w", table, err)
	}
	defer rows.Close()

	bw := bufio.NewWriterSize(w, 64*1024)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false) // 正文里的 < > 没必要转义成 \u003c，文件更小更可读

	// 每行扫成 []any，再按列名转成 map —— 导入侧按列名取值，
	// 不怕以后加列 / 调顺序。
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("读取表 %s 的数据失败: %w", table, err)
		}
		obj := make(map[string]any, len(cols))
		for i, c := range cols {
			obj[c.Name] = jsonValue(vals[i], c)
		}
		if err := enc.Encode(obj); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// jsonValue 把驱动返回的 Go 值转成「导入时能被无损还原」的 JSON 值。
//
// 主要处理时间：JSON 里时间只能是字符串，而 timestamp（无时区）与 timestamptz
// （有时区）的解析规则不同 —— 统一按 RFC3339 输出的话，无时区列会被按会话时区
// 做一次换算，可能出现整体偏移。所以这里按列类型分开格式化。
func jsonValue(v any, c columnInfo) any {
	switch x := v.(type) {
	case nil:
		return nil
	case time.Time:
		if c.DataType == "timestamp without time zone" {
			return x.Format("2006-01-02 15:04:05.999999")
		}
		return x.Format("2006-01-02 15:04:05.999999-07:00")
	case []byte:
		if c.DataType == "bytea" {
			return base64.StdEncoding.EncodeToString(x)
		}
		return string(x)
	default:
		return x
	}
}

// ------------------------------------------------------------
// 导入 / 恢复
// ------------------------------------------------------------

// restoreOrder 插入顺序：被外键引用的表排在前面。
// 表结构里唯一的跨表引用是 conversation_members.conversation_id → conversations(id)。
var restoreOrder = []string{
	"users", "posts", "settings", "comments", "notifications", "post_likes",
	"friends", "conversations", "conversation_members", "messages",
}

// orderTables 按 restoreOrder 排；出现的新表排在最后（按名字）。
func orderTables(tables []string) []string {
	rank := map[string]int{}
	for i, t := range restoreOrder {
		rank[t] = i
	}
	out := append([]string{}, tables...)
	sort.Slice(out, func(i, j int) bool {
		ri, okI := rank[out[i]]
		rj, okJ := rank[out[j]]
		if okI != okJ {
			return okI // 已知表优先
		}
		if okI && okJ && ri != rj {
			return ri < rj
		}
		return out[i] < out[j]
	})
	return out
}

// readManifest 只读归档里的 manifest.json（不碰任何数据），用于导入前校验。
func readManifest(zipPath string) (*BackupManifest, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("不是有效的 zip 文件: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != backupManifestName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		var m BackupManifest
		if err := json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&m); err != nil {
			return nil, fmt.Errorf("manifest.json 解析失败: %w", err)
		}
		if m.Format != backupFormat {
			return nil, fmt.Errorf("这不是本系统的备份文件（format=%s）", m.Format)
		}
		if m.FormatVersion > backupFormatVersion {
			return nil, fmt.Errorf("备份格式版本 %d 高于当前程序支持的 %d，请先升级程序",
				m.FormatVersion, backupFormatVersion)
		}
		return &m, nil
	}
	return nil, fmt.Errorf("备份文件里缺少 manifest.json")
}

// restoreBackup 从备份文件恢复。
//
// 执行顺序刻意如此：**先把文件解到暂存目录 → 再动数据库 → 最后搬文件**。
// 数据库那一步在单个事务里，失败会整体回滚，磁盘不受影响；文件部分无法参与
// 事务，所以放到最后，且逐个用「临时文件 + rename」原子替换。
func restoreBackup(zipPath string) (*RestoreReport, error) {
	manifest, err := readManifest(zipPath)
	if err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	// 归档内条目按前缀归类
	dbFiles := map[string]*zip.File{} // 表名 → ndjson
	var postEntries, uploadEntries []*zip.File
	for _, f := range zr.File {
		switch {
		case strings.HasPrefix(f.Name, "db/") && strings.HasSuffix(f.Name, ".ndjson"):
			dbFiles[strings.TrimSuffix(strings.TrimPrefix(f.Name, "db/"), ".ndjson")] = f
		case strings.HasPrefix(f.Name, "posts/"):
			postEntries = append(postEntries, f)
		case strings.HasPrefix(f.Name, "uploads/"):
			uploadEntries = append(uploadEntries, f)
		}
	}

	// ---------- ① 文件先解到暂存目录 ----------
	// 暂存目录放在目标盘的兄弟位置，保证后面 rename 是原子的（跨盘 rename 会退化成复制）。
	stageBase := absOrSelf(uploadDir)
	cleanStaleStages(stageBase) // 上次被强杀留下的残骸先扫掉
	stageRoot, err := os.MkdirTemp(stageBase, ".restore-*")
	if err != nil {
		return nil, fmt.Errorf("创建暂存目录失败: %w", err)
	}
	defer os.RemoveAll(stageRoot) // 无论成功失败都清掉

	stagePosts := filepath.Join(stageRoot, "posts")
	stageUploads := filepath.Join(stageRoot, "uploads")
	if err := extractZip(postEntries, "posts", stagePosts); err != nil {
		return nil, err
	}
	if err := extractZip(uploadEntries, "uploads", stageUploads); err != nil {
		return nil, err
	}

	// ---------- ② 数据库：单个事务整体替换 ----------
	report := &RestoreReport{Manifest: *manifest, Tables: map[string]int64{}}
	if err := restoreDB(dbFiles, report); err != nil {
		return nil, err
	}

	// ---------- ③ 文件搬到位 ----------
	if len(postEntries) > 0 {
		if !postContent.Enabled() {
			return nil, fmt.Errorf("备份里含文章正文，但本机未启用正文文件存储（POST_CONTENT_DIR 为空）")
		}
		n, err := moveTree(stagePosts, postContent.Root())
		if err != nil {
			return nil, fmt.Errorf("写入文章正文失败: %w", err)
		}
		report.Posts = n
		postContent.InvalidateIndex() // 正文整批换了，全文索引必须重建
	}
	if len(uploadEntries) > 0 {
		n, err := moveTree(stageUploads, uploadDir)
		if err != nil {
			return nil, fmt.Errorf("写入上传文件失败: %w", err)
		}
		report.Uploads = n
	}

	return report, nil
}

// restoreDB 在单个事务里「清空全部业务表 → 按备份插入 → 校准自增序列」。
func restoreDB(dbFiles map[string]*zip.File, report *RestoreReport) error {
	// ---------- 先把元数据全部查完（必须在开事务之前！） ----------
	//
	// 踩过的坑：information_schema.columns 会**对自己描述的表加 AccessShareLock**，
	// 而下面 TRUNCATE 拿的是 AccessExclusiveLock。如果在事务里、用另一条连接去查列
	// 信息，就会变成「事务等代码、代码等事务的锁」—— 自己把自己锁死，永远不返回
	// （实测表现：卡住不动，pg_locks 里一条 active 在等 relation 锁、另一条
	//   idle in transaction 攥着 AccessExclusiveLock）。
	// 所以表列表 / 列 / 自增标记统统一开始查好，事务里只做 TRUNCATE + INSERT + setval。
	local, err := appTables()
	if err != nil {
		return fmt.Errorf("读取本地表列表失败: %w", err)
	}
	colsOf := map[string][]columnInfo{}
	for _, t := range local {
		cols, err := tableColumns(t)
		if err != nil {
			return fmt.Errorf("读取表 %s 的列失败: %w", t, err)
		}
		colsOf[t] = cols
	}

	inBackup := map[string]bool{}
	for t := range dbFiles {
		inBackup[t] = true
	}
	for _, t := range local {
		if !inBackup[t] {
			// 本机有、备份里没有：清空后就是空的（这是「恢复成备份那一刻」的应有之义）
			report.Missing = append(report.Missing, t)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback() // 成功提交后再调用是空操作

	// ① 一次清空所有本地业务表。
	// 必须「全部先清、再插入」：比如 conversation_members 引用 conversations，
	// 如果按表逐个「清一张插一张」，后面清 conversations 时的 CASCADE 会把刚插
	// 进去的成员行一起删掉。
	if len(local) > 0 {
		stmt := `TRUNCATE TABLE ` + strings.Join(quoteIdentAll(local), ", ") + ` RESTART IDENTITY CASCADE`
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("清空旧数据失败: %w", err)
		}
	}

	// ② 按依赖顺序插入
	tables := make([]string, 0, len(dbFiles))
	for t := range dbFiles {
		tables = append(tables, t)
	}
	ordered := orderTables(tables)
	for _, t := range ordered {
		if !contains(local, t) {
			// 备份里有、当前程序不认识的表（一般是拿新版本的备份往旧版本导）：跳过
			report.Missing = append(report.Missing, t+"（本机不存在，已跳过）")
			continue
		}
		n, err := insertTable(tx, t, colsOf[t], dbFiles[t])
		if err != nil {
			return err
		}
		report.Tables[t] = n
	}

	// ③ 自增序列推到备份里的最大值：否则恢复后新建数据会撞主键
	for _, t := range ordered {
		if !contains(local, t) {
			continue
		}
		for _, c := range colsOf[t] {
			if !c.Serial {
				continue
			}
			stmt := `SELECT setval(pg_get_serial_sequence($1, $2),
				COALESCE((SELECT MAX(` + quoteIdent(c.Name) + `) FROM ` + quoteIdent(t) + `), 1))`
			if _, err := tx.Exec(stmt, "public."+t, c.Name); err != nil {
				return fmt.Errorf("校准表 %s 的序列失败: %w", t, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交失败: %w", err)
	}
	return nil
}

// insertTable 把一份 NDJSON 插入某张表，返回插入行数。
// cols 由调用方在事务外查好传进来（原因见 restoreDB 顶部关于自死锁的说明）。
func insertTable(tx *sql.Tx, table string, cols []columnInfo, zf *zip.File) (int64, error) {
	rc, err := zf.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	dec := json.NewDecoder(bufio.NewReaderSize(rc, 64*1024))
	var inserted int64
	batch := make([]map[string]any, 0, insertBatchRows)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := insertBatch(tx, table, cols, batch); err != nil {
			return err
		}
		inserted += int64(len(batch))
		batch = batch[:0]
		return nil
	}

	for {
		var row map[string]any
		if err := dec.Decode(&row); err == io.EOF {
			break
		} else if err != nil {
			return 0, fmt.Errorf("解析表 %s 的数据失败: %w", table, err)
		}
		batch = append(batch, row)
		if len(batch) >= insertBatchRows {
			if err := flush(); err != nil {
				return 0, err
			}
		}
	}
	if err := flush(); err != nil {
		return 0, err
	}
	return inserted, nil
}

// insertBatch 拼一条多行 INSERT。
// 只使用「备份行里确实有这个键」的列：备份来自旧版本、缺了后来新增的列时，
// 那些列会走数据库默认值，而不是硬塞一个 NULL 进去。
func insertBatch(tx *sql.Tx, table string, cols []columnInfo, rows []map[string]any) error {
	use := make([]columnInfo, 0, len(cols))
	for _, c := range cols {
		if _, ok := rows[0][c.Name]; ok {
			use = append(use, c)
		}
	}
	if len(use) == 0 {
		return fmt.Errorf("表 %s 的备份数据里没有任何可用列", table)
	}

	names := make([]string, len(use))
	for i, c := range use {
		names[i] = c.Name
	}

	args := make([]any, 0, len(rows)*len(use))
	var sb strings.Builder
	sb.WriteString(`INSERT INTO ` + quoteIdent(table) + ` (` + strings.Join(quoteIdentAll(names), ", ") + `) VALUES `)
	for r, row := range rows {
		if r > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(")
		for i, c := range use {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("$" + strconv.Itoa(len(args)+1))
			args = append(args, decodeValue(row[c.Name], c))
		}
		sb.WriteString(")")
	}

	if _, err := tx.Exec(sb.String(), args...); err != nil {
		return fmt.Errorf("写入表 %s 失败: %w", table, err)
	}
	return nil
}

// decodeValue 把 JSON 里的值还原成数据库能接受的参数。
// 时间保持字符串（PG 会按列类型解析），bytea 用 base64 还原成二进制。
func decodeValue(v any, c columnInfo) any {
	if v == nil {
		return nil
	}
	if c.DataType == "bytea" {
		if s, ok := v.(string); ok {
			if b, err := base64.StdEncoding.DecodeString(s); err == nil {
				return b
			}
		}
		return nil
	}
	return v
}

// ------------------------------------------------------------
// 文件解包 / 搬运
// ------------------------------------------------------------

// extractZip 把归档里某个前缀下的条目解到 dst 目录。
//
// 安全要点：条目名完全来自上传的文件，必须挡住「zip slip」——
// 形如 ../../x 或 C:\Windows\x 的名字一旦被直接拼接，就会写到目标目录之外。
func extractZip(files []*zip.File, prefix, dst string) error {
	if len(files) == 0 {
		return nil
	}
	cleanDst := filepath.Clean(dst)
	for _, f := range files {
		rel := filepath.FromSlash(strings.TrimPrefix(f.Name, prefix+"/"))
		if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
			return fmt.Errorf("备份里有非法的文件路径：%s", f.Name)
		}
		target := filepath.Join(cleanDst, rel)
		// 双保险：Join 之后再确认一次确实落在 dst 里面
		if !strings.HasPrefix(target, cleanDst+string(os.PathSeparator)) {
			return fmt.Errorf("备份里的路径越界：%s", f.Name)
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeFileFromZip(f, target); err != nil {
			return fmt.Errorf("解压 %s 失败: %w", f.Name, err)
		}
	}
	return nil
}

// writeFileFromZip 把 zip 里的一个文件写到 target（先写临时文件再原子替换）。
func writeFileFromZip(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	tmp, err := os.CreateTemp(filepath.Dir(target), ".restore-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后这里是空操作

	if _, err := io.Copy(tmp, rc); err != nil {
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
	return replaceFileAtomic(tmpName, target)
}

// moveTree 把暂存目录里的文件搬到目标目录（逐文件原子替换），返回搬了几个。
// 不删除目标目录里多出来的旧文件：多出来的孤儿文件不影响使用，误删才是事故。
func moveTree(src, dst string) (int, error) {
	if _, err := os.Stat(src); err != nil {
		return 0, nil
	}
	n := 0
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := replaceFileAtomic(path, target); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		n++
		return nil
	})
	return n, err
}

// ------------------------------------------------------------
// 小工具
// ------------------------------------------------------------

// quoteIdent 给标识符加双引号并转义内部的双引号。
// 表名 / 列名来自 information_schema（不是用户输入），但拼 SQL 时仍然照规矩加引号：
// settings 表就有个叫 key 的列，不加引号迟早踩到保留字。
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func quoteIdentAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = quoteIdent(n)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// absOrSelf 取绝对路径的父目录，失败就退回当前目录（用于找暂存目录的位置）。
func absOrSelf(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return filepath.Dir(a)
	}
	return "."
}

// cleanStaleStages 清掉上一次被强杀留下的暂存目录。
// 恢复是「先把文件解到暂存目录、最后再搬」，进程如果是被 kill -9 掉的，
// defer 不会执行，那些 .restore-* 就会一直挂在磁盘上。开新一轮之前顺手扫一遍。
func cleanStaleStages(base string) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), ".restore-") {
			continue
		}
		_ = os.RemoveAll(filepath.Join(base, e.Name()))
	}
}
