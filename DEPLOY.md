# 部署指南

这套站点是「**一个 Go 二进制**提供全部服务」的结构：API、图片托管、前端页面托管都在同一个进程里，
数据库用 PostgreSQL，Redis 可选（只影响桃信的在线状态 / 正在输入）。

```
浏览器 ──► Nginx（可选，反代 + HTTPS）──► blog-server :8080
                                              ├─ /api/*    JSON 接口
                                              ├─ /uploads/* 用户上传的图片
                                              ├─ /pic/*     public 里的静态图
                                              └─ 其它路径 → frontend/dist/index.html（SPA 回退）
                                                    │
                                            PostgreSQL ── Redis(可选)
```

---

## ⚡ 方式 A：Docker 一键启动（推荐）

整套环境（PostgreSQL + Redis + 博客）都在仓库里定义好了，服务器上只需要 Docker。

### 最快路径：`blog.sh`

项目根目录有一个 `blog.sh`，安装和更新都只有一行：

```bash
# 第一次：生成密码、构建、启动、等健康
bash blog.sh install

# 以后每次发版
bash blog.sh update            # 只重建 app，数据库/Redis 不动
bash blog.sh update --pull     # 目录是 git 仓库时，先拉代码再重建
```

它会打印随机生成的站长密码（**只打印一次**），并把密码写进 `.env`。

其它命令：

| 命令 | 作用 |
| --- | --- |
| `bash blog.sh status` | 三个容器状态 + 健康 |
| `bash blog.sh logs` | 跟踪应用日志 |
| `bash blog.sh update` | 构建 → 健康检查 → 切换，失败**自动回滚** |
| `bash blog.sh rollback` | 手动回滚上一版 |
| `bash blog.sh passwd 新密码` | 在容器里重置站长密码 |
| `bash blog.sh backup` | 导出数据库 + 上传文件到 `backups/` |
| `bash blog.sh down` | 停止（数据卷保留） |

**关于"热更新"**：更新时先构建新镜像（此时线上照旧服务），构建完成才替换 app 容器，
所以停机窗口 = 容器重启的 **1~3 秒**；数据库和 Redis 全程不重启。
新版本起不来会自动退回上一个镜像，不会变成一堆 502。

如果想要**零停机**（连那 1~3 秒也没有），需要蓝绿双容器 + Nginx upstream 切换，见文末「零停机更新」。

### 想做成真正的"一行命令"

`blog.sh` 默认用**它自己所在目录**的源码。如果想让它连源码一起搞定，用 `BLOG_REPO` 指定仓库地址，
它会自动 `git clone` 到 `BLOG_DIR`（已存在就 `git pull`）：

```bash
# 安装（自动拉代码 + 生成密码 + 构建 + 启动）
BLOG_REPO=https://github.com/你/blog.git BLOG_DIR=/opt/blog \
  bash -o pipefail -c 'curl -fsSL https://example.com/blog.sh | bash -s -- install'

# 更新（拉代码 + 重建 + 健康检查 + 失败回滚）
BLOG_REPO=https://github.com/你/blog.git BLOG_DIR=/opt/blog \
  bash -o pipefail -c 'curl -fsSL https://example.com/blog.sh | bash -s -- update'
```

也可以把它当成两个独立步骤，更直观：

```bash
git clone https://github.com/你/blog.git /opt/blog
cd /opt/blog && bash blog.sh install
```

> 用 `curl | bash` 时，脚本拿不到自己的路径，所以**必须**给 `BLOG_DIR`（或先 `cd` 到项目目录），
> 否则它不知道源码该放哪。

### 1. 准备配置

```bash
cd /opt/blog            # 或任意目录（git clone / 上传均可）
cp .env.example .env
vi .env                 # 改 DB_PASSWORD、ADMIN_PASSWORD
```

`.env` 就两个密码要改：

```ini
DB_NAME=blog
DB_USER=blog
DB_PASSWORD=openssl rand -base64 15 生成一个
ADMIN_USER=admin
ADMIN_PASSWORD=你的站长密码
```

### 2. 一键启动

```bash
docker compose up -d --build
```

第一次会构建镜像（约 2~4 分钟），之后启动只要几秒。

### 3. 验证

```bash
docker compose ps                 # 三个服务都应是 healthy / running
docker compose logs -f app        # 看到 "successfully started" 和 "PostgreSQL connected"
curl -I http://127.0.0.1:8080/    # 200
```

浏览器打开 `http://服务器IP:8080`（默认只绑了 `127.0.0.1`，外网请走 Nginx 反代，见第 7 节）。

### 4. 日常运维

```bash
docker compose restart app      # 改完 .env 后重启
docker compose logs -f app      # 看日志
docker compose down             # 停止（数据卷保留）
docker compose down -v          # ⚠️ 停止并删除所有数据
docker compose up -d --build app   # 更新代码后重建
```

### 5. 数据在哪

| 内容 | Docker 卷 | 容器内路径 |
| --- | --- | --- |
| 数据库 | `pgdata` | PostgreSQL 数据目录 |
| 上传图片 | `uploads` | `/app/uploads` |
| 文章正文 | `posts` | `/app/data/posts` |
| Redis AOF | `redisdata` | `/data` |

备份：`docker run --rm -v blog_pgdata:/data -v $PWD:/backup alpine tar czf /backup/pgdata.tgz -C /data .`

或在后台「备份」页导出 zip（数据库 + 正文 + 上传文件，最省事）。

### 6. 从「裸机部署」迁移过来

如果之前已经用 systemd 跑过一段时间，把旧库导进容器：

```bash
# 旧库导出
pg_dump -h 127.0.0.1 -U blog blog > old.sql

# 导入容器（注意容器名，compose 里叫 blog-postgres）
docker exec -i blog-postgres psql -U blog -d blog < old.sql

# 旧的上传文件和正文
docker cp uploads/.      blog-app:/app/uploads/
docker cp data/posts/.   blog-app:/app/data/posts/
docker compose restart app
```

### 7. 前面挂 Nginx 反代

compose 默认只把 8080 绑在 `127.0.0.1`，公网访问要走反代：

```nginx
server {
    listen 80;
    server_name 你的域名;
    client_max_body_size 12m;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_buffering off;        # ★ 桃信 SSE 必须关缓冲
        proxy_read_timeout 3600s;
    }
}
```

在 1Panel 里就是：**网站 → 创建 → 反向代理 → 代理地址 `http://127.0.0.1:8080`**（1Panel 自身也有「编排」入口，可以直接粘贴 `docker-compose.yml`）。

### 8. 常见问题

| 现象 | 原因 |
| --- | --- |
| `app` 一直 restarting | `.env` 里 `DB_PASSWORD` 和 postgres 容器不一致，或没填 |
| 打开是 502 | 反代指向的端口错，或 `app` 没 healthy |
| 上传图片 500 | 卷权限问题：`docker compose down && docker compose up -d`（重建卷） |
| 中文时间差 8 小时 | 检查 compose 里三个服务都有 `TZ: Asia/Shanghai`（已内置） |

---

## 方式 B：裸机部署（不用 Docker）

需要 Go / Node 编译，PostgreSQL 自己装。

---

## 0. 需要准备什么

| 依赖 | 版本 | 用途 | 是否必须 |
| --- | --- | --- | --- |
| Go | 1.25+ | 编译后端 | 构建时需要 |
| Node.js | 20+ | 构建前端 | 构建时需要 |
| PostgreSQL | 14+ | 全部业务数据 | **必须** |
| Redis | 6+ | 桃信在线状态 / 输入中 / 限流 | 可选 |
| Nginx | 任意 | 反代、HTTPS、压缩 | 可选但推荐 |

---

## 1. 构建

```bash
# ① 前端：产出 frontend/dist
cd frontend
npm ci            # 或 npm install
npm run build

# ② 后端：产出单文件二进制
cd ../backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../blog-server .
```

Windows 上编译 Linux 版本就把 `GOOS=linux` 换成 `windows`（或留空编译本机版本）。

> 前端 dist 是**纯静态**的，不需要 Node 常驻运行；Node 只在构建时用一次。

---

## 2. 部署目录

```
/opt/blog/
├── blog-server          # 编译出来的二进制
├── frontend/dist/       # 前端产物（STATIC_DIR 指向这里）
├── uploads/             # 上传的图片（运行时要可写）
├── data/posts/          # 文章正文 HTML（运行时要可写）
└── blog.env             # 环境变量（见下一节）
```

`uploads/` 和 `data/posts/` 必须可写；`frontend/dist` 只读即可。

---

## 3. 环境变量

全部有默认值，最小配置也能跑。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 监听端口 |
| `STATIC_DIR` | `../frontend/dist` | 前端产物目录（相对启动目录） |
| `UPLOAD_DIR` | `uploads` | 图片上传目录 |
| `POST_CONTENT_DIR` | `../data/posts` | 文章正文文件目录 |
| `DB_HOST` | `127.0.0.1` | PostgreSQL 地址 |
| `DB_PORT` | `5432` | |
| `DB_USER` | `postgres` | |
| `DB_PASSWORD` | 空 | |
| `DB_NAME` | `blog` | 库名 |
| `DB_SSLMODE` | `disable` | 走公网数据库时改 `require` |
| `ADMIN_USER` | `admin` | 站长账号名 |
| `ADMIN_PASSWORD` | 空 | **不设则首次启动随机生成并打印在日志里** |
| `REDIS_ADDR` | 空 | 例 `127.0.0.1:6379`；不设 = 关闭实时层（聊天照常，只显示离线） |

示例 `blog.env`：

```ini
PORT=8080
STATIC_DIR=./frontend/dist
UPLOAD_DIR=./uploads
POST_CONTENT_DIR=./data/posts
DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=blog
DB_PASSWORD=换成你的密码
DB_NAME=blog
ADMIN_USER=admin
ADMIN_PASSWORD=换成强密码
REDIS_ADDR=127.0.0.1:6379
TZ=Asia/Shanghai
```

> **`TZ` 很重要**：热力图按天分组、签到按天判定都依赖本地时区。
> 容器/服务器默认 UTC 时，北京时间 00:00–08:00 的活动会被算到前一天。

---

## 4. 建库

```sql
CREATE DATABASE blog ENCODING 'UTF8';
CREATE USER blog WITH PASSWORD '换成你的密码';
GRANT ALL PRIVILEGES ON DATABASE blog TO blog;
```

表结构**不用手动建**：启动时会自动执行 `CREATE TABLE IF NOT EXISTS` 等幂等迁移，
老库升级也走同一套逻辑。

---

## 5. 首次启动

```bash
cd /opt/blog
set -a; . ./blog.env; set +a
./blog-server
```

看到这两行就说明起来了：

```
The backend service has been successfully started: http://localhost:8080
PostgreSQL connected: blog@127.0.0.1:5432/blog
```

站长账号：
- 设了 `ADMIN_PASSWORD` → 用它登录；
- 没设 → 首次启动会随机生成 12 位密码并打一行日志：
  `首次部署：已自动创建站长账号 admin，初始密码为 xxxxxxxx（请尽快登录后台修改）`。
  **这行只打印一次**，请立刻保存。

后台登录地址：`/admin/login`。

忘了密码可以改：

```bash
./blog-server changepass 新密码          # 改站长自己的
./blog-server changepass 用户名 新密码    # 改指定用户
```

---

## 6. 常驻运行

### Linux（systemd）

`/etc/systemd/system/blog.service`：

```ini
[Unit]
Description=RinEyce Blog
After=network.target postgresql.service

[Service]
Type=simple
WorkingDirectory=/opt/blog
EnvironmentFile=/opt/blog/blog.env
ExecStart=/opt/blog/blog-server
Restart=always
RestartSec=3
# 安全加固（可选）
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now blog
sudo journalctl -u blog -f
```

### Windows

最简单的两种：

```powershell
# ① 计划任务（开机启动）
schtasks /create /tn "BlogServer" /tr "D:\website\.tools\blog-server-dev.exe" /sc onstart /ru SYSTEM

# ② 或用 nssm 注册成服务
nssm install BlogServer D:\website\.tools\blog-server-dev.exe
nssm set BlogServer AppDirectory D:\website
nssm start BlogServer
```

---

## 7. Nginx 反代 + HTTPS

```nginx
server {
    listen 80;
    server_name example.com;

    # 上传接口限 10MB，这里留余量
    client_max_body_size 12m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # ★ 桃信用 SSE 长连接，必须关掉缓冲，否则消息会被攒着不发
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 3600s;
    }
}
```

HTTPS 用 certbot：

```bash
sudo certbot --nginx -d example.com
```

---

## 8. 备份 / 恢复 / 升级

### 备份（三种都行）

1. **后台「备份」页**：导出 zip（数据库 + 正文 + 上传文件），一键下载；
2. **命令行**：`./blog-server backup /path/out.zip` / `./blog-server restore /path/out.zip REPLACE`；
3. **pg_dump**：`pg_dump -U blog blog > blog.sql`（记得同时备份 `uploads/` 与 `data/posts/`）。

建议加个每日定时：

```bash
# crontab -e
0 4 * * * cd /opt/blog && ./blog-server backup /backup/blog-$(date +\%F).zip
```

### 升级

```bash
# 本地构建好新的 dist 和二进制，上传覆盖，然后重启
sudo systemctl restart blog
```

数据库表结构会自动迁移，不用手动执行 SQL。

---

## 9. 常见坑

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| 页面能开，但接口全 500 | 连不上数据库 | 看日志里的 `PostgreSQL connected` 有没有出现 |
| 桃信能发消息但对方收不到 | Nginx 缓冲了 SSE | `proxy_buffering off` |
| 上传图片报 413 | Nginx 默认 1MB | `client_max_body_size 12m` |
| 所有页面 404 | `STATIC_DIR` 指错或 dist 没上传 | 确认 `index.html` 在 `STATIC_DIR` 里 |
| 图片上传后 404 | `UPLOAD_DIR` 不可写 / 路径不对 | 给运行用户写权限 |
| 热力图 / 签到日期错一天 | 服务器是 UTC | 设 `TZ=Asia/Shanghai` |
| 在线状态一直离线 | 没配 Redis | 设 `REDIS_ADDR`，或忽略（聊天功能不受影响） |
