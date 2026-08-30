package main

// ============================================================
// db.go —— 数据库模块（MySQL）
// 职责：解析连接配置、连接 MySQL、自动建库建表、维护全局连接池。
//
// 设计说明：
//   - 使用 database/sql 标准库 + github.com/go-sql-driver/mysql 驱动。
//   - 后端启动时自动执行 CREATE DATABASE / CREATE TABLE IF NOT EXISTS，
//     所以「不需要手动建表」，连上 MySQL 就能跑。
//   - 全局变量 db 是唯一的数据库句柄，所有 handler 都直接使用它。
// ============================================================

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL 驱动（空导入：注册驱动名为 "mysql"）
)

// db 全局数据库连接池（database/sql 自带连接池管理，可直接并发使用）
var db *sql.DB

// DBConfig MySQL 连接配置，全部来自环境变量（main.go 中组装）。
// 对应环境变量：DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME
type DBConfig struct {
	Host     string // MySQL 地址，默认 127.0.0.1
	Port     string // MySQL 端口，默认 3306
	User     string // 用户名，默认 root
	Password string // 密码，默认空
	Name     string // 数据库名，默认 blog
}

// dsn 构造 go-sql-driver 的连接串（DSN）。
// includeDB=false 时不带库名 —— 用于「先连上服务器再创建数据库」的场景。
// 参数说明：
//   - charset=utf8mb4：全量 UTF-8，保证中文/emoji 正确存取
//   - parseTime=false：DATETIME 列按原始字符串读取（本项目的日期只是展示用，不需要转 time.Time）
func (c DBConfig) dsn(includeDB bool) string {
	name := ""
	if includeDB {
		name = c.Name
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=false",
		c.User, c.Password, c.Host, c.Port, name)
}

// initDB 初始化数据库，分三步（每步都幂等，重启服务不会出错）：
//   1. 不带库名连接 MySQL 服务器，CREATE DATABASE IF NOT EXISTS 建库（utf8mb4）
//   2. 连接目标库，测试连通（Ping），并配置连接池
//   3. 逐条执行 CREATE TABLE IF NOT EXISTS 建表（不用 multiStatements，更安全）
func initDB(cfg DBConfig) error {
	// ---------- ① 确保数据库存在 ----------
	admin, err := sql.Open("mysql", cfg.dsn(false))
	if err != nil {
		return fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	defer admin.Close()
	admin.SetMaxOpenConns(1) // 建库阶段一个连接就够
	if _, err := admin.Exec(
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", cfg.Name),
	); err != nil {
		return fmt.Errorf("创建数据库失败: %w", err)
	}

	// ---------- ② 连接目标数据库 ----------
	db, err = sql.Open("mysql", cfg.dsn(true))
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}
	// 连接池参数：最多 10 个连接、5 个空闲、单连接最长 3 分钟（防止 MySQL 空闲断开）
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)
	if err := db.Ping(); err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// ---------- ③ 建表（schema 与 docs 在线文档中的「数据库设计」一致） ----------
	schema := []string{
		// users 表：管理员账号
		`CREATE TABLE IF NOT EXISTS users (
			id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, -- 自增主键
			username      VARCHAR(64) NOT NULL,                    -- 用户名（唯一）
			password_hash VARCHAR(255) NOT NULL,                   -- bcrypt 加密后的密码
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, -- 创建时间
			PRIMARY KEY (id),
			UNIQUE KEY uk_username (username) -- 用户名唯一约束
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// posts 表：博客文章
		`CREATE TABLE IF NOT EXISTS posts (
			id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, -- 自增主键
			title      VARCHAR(255) NOT NULL,                   -- 标题
			summary    TEXT NOT NULL,                           -- 摘要（列表页展示）
			cover      TEXT NOT NULL,                           -- 封面图 URL（可空字符串）
			content    LONGTEXT NOT NULL,                       -- 正文（wangEditor 产出的 HTML）
			status     VARCHAR(16) NOT NULL DEFAULT 'draft',    -- 状态：draft 草稿 / published 已发布
			likes      INT NOT NULL DEFAULT 0,                  -- 点赞数
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, -- 创建时间
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, -- 更新时间（更新时手动刷 NOW()）
			PRIMARY KEY (id),
			KEY idx_status_created (status, created_at) -- 列表查询常用条件，建索引加速
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		// settings 表：键值对设置（目前只存 JWT 密钥）
		"CREATE TABLE IF NOT EXISTS settings (" +
			"`key` VARCHAR(64) NOT NULL, " + // 注意：key 是 MySQL 保留字，必须反引号
			"value TEXT NOT NULL, " + // 值
			"PRIMARY KEY (`key`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("初始化表结构失败: %w", err)
		}
	}
	return nil
}

// getenv 读取环境变量；未设置或为空时返回默认值 def。
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
