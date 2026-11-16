package main

// ============================================================
// console.go —— 后端命令控制台
// 职责：让运行中的后端可以直接在终端里输入命令，执行运维操作。
//
// 两种使用方式：
//   1. 交互模式：go run . 启动后，在终端输入命令（输入 help 查看帮助）
//   2. 命令行模式：go run . <命令> [参数]，执行一次后自动退出
//
// 可用命令：
//   help                       查看帮助
//   info                       查看服务 / 数据库 / 账户信息
//   changepass                 交互式修改管理员密码
//   changepass <新密码>          直接修改管理员密码
//   changepass <用户名> <新密码>  修改指定用户的密码
//   exit | quit                退出后端
//
// 设计说明：
//   - 控制台跑在独立 goroutine 里，不阻塞 HTTP 服务；
//   - 后台运行（stdin 不可用）时控制台自动静默退出，不影响服务；
//   - 密码修改走与登录相同的 bcrypt 加密，保证一致性。
// ============================================================

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

)

// startConsole 启动交互式命令控制台（goroutine）。
// 在终端里执行 go run . 后即可输入命令；stdin 关闭时（如后台运行）自动退出。
func startConsole() {
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("========================================")
		fmt.Println("  凛冬博客后端控制台已就绪")
		fmt.Println("  输入 help 查看可用命令，exit 退出")
		fmt.Println("========================================")
		for {
			fmt.Print("> ")
			if !scanner.Scan() {
				return // stdin 已关闭（后台运行/服务模式），控制台静默退出
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if shouldExit, _ := runCommand(scanner, fields[0], fields[1:]); shouldExit {
				os.Exit(0) // 用户输入 exit/quit
			}
		}
	}()
}

// runCommandOnce 命令行模式入口：go run . <命令> [参数]，执行一次后退出。
// 返回进程退出码（0 成功 / 1 未知命令或失败）。
func runCommandOnce(name string, args []string) int {
	_, success := runCommand(nil, name, args)
	if success {
		return 0
	}
	return 1
}

// runCommand 执行单条命令。
// 返回 (shouldExit, success)：
//   shouldExit —— true 表示程序应退出（用户输入 exit/quit）；
//   success    —— 命令是否成功执行（用于命令行模式的退出码）。
func runCommand(scanner *bufio.Scanner, name string, args []string) (shouldExit, success bool) {
	switch name {
	case "help", "h", "?":
		cmdHelp()
	case "info", "i":
		cmdInfo()
	case "changepass", "cp", "password":
		cmdChangePass(scanner, args)
	case "backup", "dump":
		cmdBackup(args)
	case "restore", "import":
		cmdRestore(scanner, args)
	case "chatpartition", "chat-upgrade":
		cmdChatPartition()
	case "exit", "quit", "q":
		fmt.Println("正在退出后端...")
		return true, true
	default:
		fmt.Printf("未知命令: %s（输入 help 查看帮助）\n", name)
		return false, false
	}
	return false, true
}

// cmdHelp 打印命令帮助。
func cmdHelp() {
	fmt.Println("可用命令：")
	fmt.Println("  help                        显示本帮助")
	fmt.Println("  info                        查看服务 / 数据库 / 账户信息")
	fmt.Println("  chatpartition               把空的消息表升级为按月分区（非空则拒绝）")
	fmt.Println("  changepass                  交互式修改管理员密码（提示输入，密码明文回显）")
	fmt.Println("  changepass <新密码>          直接修改管理员密码（至少 6 位）")
	fmt.Println("  changepass <用户名> <新密码>  修改指定用户的密码")
	fmt.Println("  backup [文件] [parts]       导出备份 zip（parts 默认 db,posts,uploads）")
	fmt.Println("  restore <文件> [REPLACE]    从备份恢复（会清空并整体替换现有数据）")
	fmt.Println("  exit | quit                退出后端")
}

// cmdChatPartition 查看聊天消息表的分区状态。
//
// 换到 PostgreSQL 后，建分区是 `CREATE TABLE ... PARTITION OF`（幂等、可随时执行），
// 不再需要「清空表才能升级」这种 MySQL 时代的手工迁移，所以这里只做状态展示，
// 并顺手补齐未来几个月的分区。
func cmdChatPartition() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("正在检查聊天消息表分区…")
	// 校验当前确实是分区表
	var isPartitioned bool
	err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_partitioned_table WHERE partrelid = 'messages'::regclass)`).Scan(&isPartitioned)
	if err != nil {
		fmt.Printf("❌ 检查失败: %v\n", err)
		return
	}
	if !isPartitioned {
		fmt.Println("⚠️  messages 不是分区表（可能还是旧结构）。")
	} else {
		fmt.Println("✅ messages 是分区表。")
	}
	// 顺手补齐未来分区（幂等）
	if err := chatStore.Maintain(ctx); err != nil {
		fmt.Printf("（补齐分区失败: %v）\n", err)
	}
	cmdChatPartitionInfo()
}

// cmdChatPartitionInfo 打印当前分区列表，方便确认维护任务是否正常工作。
func cmdChatPartitionInfo() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// PostgreSQL 的分区是真实子表（messages_pYYYYMM），查 pg_inherits 即可
	rows, err := q.QueryContext(ctx,
		`SELECT c.relname FROM pg_class c
		 JOIN pg_inherits i ON c.oid = i.inhrelid
		 WHERE i.inhparent = 'messages'::regclass
		 ORDER BY c.relname`)
	if err != nil {
		fmt.Printf("（读取分区列表失败: %v）\n", err)
		return
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err == nil {
			names = append(names, n)
		}
	}
	fmt.Printf("当前分区（%d 个）: %s\n", len(names), strings.Join(names, ", "))
	fmt.Println("提示：启动与每日维护会自动补齐未来 6 个月的分区。")
}

// cmdInfo 查看服务 / 数据库 / 账户信息。
// 数据库密码来自环境变量 DB_PASSWORD（默认空），此处按配置原样展示，方便本地排查。
func cmdInfo() {
	fmt.Println("========== 服务信息 ==========")
	fmt.Printf("HTTP 监听端口: %s\n", port)
	fmt.Printf("上传目录     : %s\n", uploadDir)

	fmt.Println("========== 数据库信息 ==========")
	fmt.Println("类型         : PostgreSQL")
	fmt.Printf("地址         : %s:%s\n", dbCfg.Host, dbCfg.Port)
	fmt.Printf("用户名       : %s\n", dbCfg.User)
	if dbCfg.Password == "" {
		fmt.Println("密码         : (空)")
	} else {
		fmt.Printf("密码         : %s\n", dbCfg.Password)
	}
	fmt.Printf("数据库名     : %s\n", dbCfg.Name)
	if err := db.Ping(); err != nil {
		fmt.Printf("连接状态     : 失败（%v）\n", err)
	} else {
		fmt.Println("连接状态     : 正常")
	}

	fmt.Println("========== 账户信息 ==========")
	var u string
	if err := q.QueryRow(`SELECT username FROM users WHERE role = 'admin' ORDER BY id LIMIT 1`).Scan(&u); err == nil {
		fmt.Printf("管理员账号   : %s\n", u)
	} else {
		fmt.Println("管理员账号   : (尚无管理员，首次启动会自动创建)")
	}
	var userCount int64
	_ = q.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'user'`).Scan(&userCount)
	fmt.Printf("注册用户     : %d\n", userCount)
	var secret string
	if err := q.QueryRow("SELECT value FROM settings WHERE `key` = 'jwt_secret'").Scan(&secret); err == nil && secret != "" {
		fmt.Printf("JWT 密钥     : 已配置（%d 位随机串，存于 settings 表）\n", len(secret))
	} else {
		fmt.Println("JWT 密钥     : 未配置（首次登录时自动生成）")
	}

	fmt.Println("========== 内容统计 ==========")
	var total, published int
	q.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&total)
	q.QueryRow(`SELECT COUNT(*) FROM posts WHERE status = 'published'`).Scan(&published)
	fmt.Printf("文章总数     : %d（已发布 %d / 草稿 %d）\n", total, published, total-published)
}

// cmdChangePass 修改用户密码。
// 参数规则：
//   无参数   —— 交互模式：提示输入新密码（两次确认）
//   1 个参数 —— 修改管理员（ADMIN_USER，默认 admin）的密码
//   2 个参数 —— 修改指定用户的密码
func cmdChangePass(scanner *bufio.Scanner, args []string) {
	username := adminUser
	newPass := ""

	switch len(args) {
	case 0:
		if scanner == nil {
			fmt.Println("用法: changepass <新密码>  或  changepass <用户名> <新密码>")
			return
		}
		fmt.Print("请输入新密码（至少 6 位，明文回显）: ")
		if !scanner.Scan() {
			fmt.Println("\n输入中断，已取消。")
			return
		}
		newPass = strings.TrimSpace(scanner.Text())
		fmt.Print("请再次输入确认: ")
		if !scanner.Scan() {
			fmt.Println("\n输入中断，已取消。")
			return
		}
		if confirm := strings.TrimSpace(scanner.Text()); confirm != newPass {
			fmt.Println("两次输入不一致，已取消。")
			return
		}
	case 1:
		newPass = args[0]
	case 2:
		username = args[0]
		newPass = args[1]
	default:
		fmt.Println("用法: changepass [用户名] [新密码]")
		return
	}

	if len(newPass) < 6 {
		fmt.Println("密码太短：至少需要 6 位。")
		return
	}

	// 与 ensureAdmin / handleLogin 相同的 bcrypt 加密方式
	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("加密失败: %v\n", err)
		return
	}
	res, err := q.Exec(`UPDATE users SET password_hash = ? WHERE username = ?`, string(hash), username)
	if err != nil {
		fmt.Printf("修改失败: %v\n", err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fmt.Printf("用户不存在: %s\n", username)
		return
	}
	fmt.Printf("✅ 密码修改成功（用户: %s）。请用新密码登录。\n", username)
}
