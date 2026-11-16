# 存储架构（PostgreSQL 单库）

本站已完成从 MySQL/MariaDB 到 **PostgreSQL** 的整体迁移：用户、文章索引、评论、
通知、点赞、好友、会话与消息全部在同一个 PostgreSQL 数据库里，代码中不再有
任何 MySQL 专用语法。文章正文另外放在文件系统上。

```
                    ┌──────────────────────────────┐
   浏览器 ──► Go 后端 │  PostgreSQL  blog 库          │
                    │   users / posts / comments    │
                    │   notifications / post_likes  │
                    │   friends / settings          │
                    │   conversations               │
                    │   conversation_members        │
                    │   messages（按月分区）         │
                    └──────────────────────────────┘
                              │
                              ├── data/posts/       文章正文（HTML 文件）
                              └── uploads/         用户上传的图片
```

---

## 一、PostgreSQL（唯一数据库）

### 表结构

| 表 | 用途 |
|---|---|
| `users` | 账号（管理员 + 注册用户），含 9 位桃信 ID 与等级 |
| `posts` | 文章**索引**（标题/摘要/封面/分类/标签/状态/计数/时间）；`content` 列为 NULL 时正文在文件里 |
| `comments` | 评论（两级：`parent_id` + `reply_to_user_id`） |
| `notifications` | 站内通知（`is_read` 为 BOOLEAN） |
| `post_likes` | 点赞去重（`post_id + visitor` 主键） |
| `friends` | 好友关系（`small_id < big_id` 唯一约束） |
| `settings` | 键值设置（`key` 列名在 PG 里不是保留字） |
| `conversations` | 一对一会话（`CHECK (user_a < user_b)`） |
| `conversation_members` | 会话成员 + 已读水位 `last_read_seq` |
| `messages` | 私信正文，**按月 RANGE 分区**（`messages_pYYYYMM`，另有 `messages_default` 兜底） |

### 驱动与占位符

- 驱动：`github.com/jackc/pgx/v5/stdlib`，仍然走标准库 `database/sql`
- SQL 里照旧写 `?`，由 `backend/db.go` 的 `pgQuery()` 统一翻译成 `$1/$2…`
  （`backend/pgdb.go` 的 `q` 包装了 `Query/QueryRow/Exec` 及事务版 `txq`）
- `INSERT` 用 `RETURNING id` 取自增主键（PG 驱动不支持 `LastInsertId`）
- upsert 用 `ON CONFLICT ... DO UPDATE SET x = EXCLUDED.x`

### 环境变量

```powershell
$env:DB_HOST    = "127.0.0.1"
$env:DB_PORT    = "5432"
$env:DB_USER    = "postgres"
$env:DB_PASSWORD= ""
$env:DB_NAME    = "blog"
$env:DB_SSLMODE = "disable"
```

聊天存储与主库同库（`chatstore` 的 PG 实现），也支持单独指定：
`CHAT_PG_DSN`（完整连接串）或 `CHAT_PG_HOST/PORT/USER/PASSWORD/DB`。

---

## 二、文章正文的文件存储

| 存哪 | 存什么 |
|---|---|
| `posts` 表 | 列表 / 筛选 / 分页用的轻量列 |
| `data/posts/` | 正文 HTML，路径 `<年>/<月>/<id>.html` |

文件首部带元数据，让正文文件自描述：

```
<!--post:{"id":4,"slug":"testcard02","title":"Git 使用教程",...,"v":1}-->
<p>正文 HTML</p>
```

- **写入是原子的**：临时文件 → `Sync` → rename，不会出现写一半的坏文件
- **读取策略**：`posts.content` 为 NULL（或空串）时去文件读；否则用库里的值
  —— 所以迁移可分批进行，文件临时缺失时文章仍能打开
- 环境变量 `POST_CONTENT_DIR`，默认 `./data/posts`（相对后端工作目录）
- 首次启动会把库里已有的正文自动搬到文件（幂等）

---

## 三、本机启动

```powershell
# 1) PostgreSQL（聊天 + 全部业务数据）
D:\website\.tools\pg\pgsql\bin\postgres.exe -D "D:\website\.tools\pg\data" -p 5432

# 2) Redis（在线状态 / 正在输入 / 限流，可选；不启用聊天照常）
#    默认连 127.0.0.1:6379，用 REDIS_ADDR 指定

# 3) 后端
$env:REDIS_ADDR       = "127.0.0.1:6379"
$env:PORT             = "8080"
$env:STATIC_DIR       = "D:\website\frontend\dist"
$env:UPLOAD_DIR       = "D:\website\uploads"
$env:POST_CONTENT_DIR = "D:\website\data\posts"
D:\website\.tools\blog-server-dev.exe

# 4) 前端开发服务器（热更新）：frontend 目录下 npm run dev
```

> - PostgreSQL 客户端连接：`psql -h 127.0.0.1 -p 5432 -U postgres -d blog`
> - `pg_ctl start` 在本机可能报 `could not create restricted token: error code 87`
>   （Windows 已知权限问题），直接用 `postgres.exe` 启动即可。

---

## 四、从 MariaDB 迁移历史数据（已完成，留作记录）

`backend/cmd/migrateall` 是一次性迁移工具，把 MariaDB 里的数据整体搬进 PostgreSQL：

```powershell
cd backend
go run ./cmd/migrateall
```

- 幂等：全部 `ON CONFLICT DO NOTHING`，可重复执行
- **保留原 id**：`users`/`posts`/`comments` 等主键原样搬运，
  表间引用关系（`post_id`/`user_id`/`parent_id`）不用重建；
  搬完自动把每个表的 `BIGSERIAL` 序列推到 `MAX(id)`
- 字段改名对应：`conversation_members.last_read_id` → `last_read_seq`
- `posts.content` 不搬（正文已在文件系统）

迁移之后 MariaDB 已不再是运行依赖，可以停掉。
