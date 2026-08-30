# 凛冬RinEyce 的个人主页与博客

一个前后端分离的个人网站：**Vue 3 + Vite** 前端，**Go** 后端，**MySQL** 存储，支持博客发布与管理、点赞、分享、富文本编辑（wangEditor）与图片上传。

## 技术栈

| 层 | 技术 |
|---|---|
| 前端 | Vue 3 · Vite 6 · vue-router 4 · wangEditor 5 |
| 后端 | Go · Gin · JWT · bcrypt |
| 存储 | **MySQL 8**（`github.com/go-sql-driver/mysql`） |

## 环境要求

| 工具 | 版本 | 检查命令 |
|---|---|---|
| Go | **1.25+**（依赖 quic-go 等要求较新版本） | `go version` |
| Node.js | **18+**（推荐 20+） | `node --version` |
| MySQL | **8.0+**（本地安装或用 Docker） | `mysql --version` / `docker compose version` |

首次使用需要联网下载依赖（npm / Go modules），之后可离线运行。

## 目录结构

```
myblog
├── frontend/             # Vue 3 前端
│   ├── src/
│   │   ├── views/        # 首页 / 博客列表 / 文章详情 / 后台登录 / 后台布局 / 文章管理 / 编辑器
│   │   ├── components/   # 公共组件（导航、文章卡片、首页区块等）
│   │   ├── router/       # 路由（含登录守卫）
│   │   ├── api/          # 请求封装（自动携带 JWT）
│   │   └── composables/  # 主题 / 打字机 / 登录状态
│   └── dist/             # 前端构建产物（由后端静态托管，勿手改）
├── backend/              # Go 后端
│   ├── main.go           # 入口与路由
│   ├── auth.go           # 登录 / JWT / 管理员初始化
│   ├── posts.go          # 文章 CRUD / 点赞 / 分页
│   ├── upload.go         # 图片上传
│   ├── static.go         # 前端静态托管 + SPA 回退
│   ├── db.go             # MySQL 连接 / 自动建库建表
│   └── uploads/          # 上传的图片
├── docs/                 # 📖 独立文档站（Vue 3 + Vite 项目，与主项目解耦）
│   ├── src/chapters/     # 13 个章节，每章一个独立 .vue 组件
│   ├── src/App.vue       # 文档布局（侧边导航 + 内容区）
│   ├── src/router/       # 章节路由（hash 模式，无刷新切换）
│   ├── src/style.css     # 文档全局样式（改样式主要改这里）
│   └── dist/             # 构建产物（双击 dist/index.html 即可阅读）
├── docker-compose.yml    # 一键启动 MySQL 8
└── pic/                  # 原始头像素材
```

---

## 一、首次准备（只做一次）

### 1. 启动 MySQL

**方式 1：Docker（推荐）**

```bash
docker compose up -d
# 启动后连接信息：127.0.0.1:3306，库 blog，用户 blog / 密码 blog123456
```

**方式 2：本地安装 MySQL 8**

安装后创建数据库与用户：

```sql
CREATE DATABASE IF NOT EXISTS blog CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'blog'@'localhost' IDENTIFIED BY 'blog123456';
GRANT ALL PRIVILEGES ON blog.* TO 'blog'@'localhost';
FLUSH PRIVILEGES;
```

> 后端启动时会**自动建库建表**（`CREATE DATABASE/TABLE IF NOT EXISTS`），手动建库只是为了权限分配。

### 2. 安装项目依赖

```bash
# 前端依赖
cd frontend
npm install

# 后端依赖
cd ../backend
go build -o blog-server.exe .
```

---

## 二、方式 A：一键预览（最快）

```bash
cd backend
go run .
```

后端启动时会自动连接 MySQL 并建库建表，看到日志 `凛冬博客后端已启动: http://localhost:8080` 后：

| 入口 | 地址 |
|---|---|
| 网站首页 | http://localhost:8080 |
| 博客列表 | http://localhost:8080/blog |
| **后台管理** | **http://localhost:8080/admin/login** |

> ⚠️ 此方式依赖 `frontend/dist` 构建产物；改过前端代码需重新 `npm run build`。

---

## 三、方式 B：开发调试模式（改代码实时生效）

开 **两个终端**：

### 终端 1：Go 后端（:8080）

```bash
cd backend
go run .
```

### 终端 2：Vite 开发服务器（:5173，热更新）

```bash
cd frontend
npm run dev
```

**所有页面从 5173 访问**：首页 http://localhost:5173 ，后台 **http://localhost:5173/admin/login**。

Vite 已把 `/api` 与 `/uploads` 代理到 `http://localhost:8080`（见 `frontend/vite.config.js`），前后端互通且无跨域问题。改前端热更新；改后端需重启 `go run .`。发布前执行 `npm run build` 更新 `dist/`。

---

## 四、进入后台（两种模式通用）

1. 打开后台地址：方式 A → `http://localhost:8080/admin/login`；方式 B → `http://localhost:5173/admin/login`
2. 默认管理员：**`admin` / `admin123`**
3. 功能：文章管理（发布/下架/编辑/删除）、写文章（wangEditor 富文本 + 图片上传）、草稿/发布状态切换

> 🔑 **修改/重置管理员密码**：
> - 首次启动通过环境变量指定：`$env:ADMIN_USER="myuser"; $env:ADMIN_PASSWORD="mypass"; go run .`
> - 已初始化后想重置：直接改 MySQL `blog.users` 表的 `password_hash`（需 bcrypt 加密），或删掉该用户行后重启（会自动重建）

---

## 五、在线文档（docs/ 独立文档站）

项目完整原理、部署流程、数据库与后台访问方法、全部 API 说明等，是一个**独立的 Vue 3 + Vite 项目**（放在 `docs/`，与主项目无任何关联）。

- **查看成品**：直接双击 `docs/dist/index.html`（已构建，hash 路由 + 相对路径，离线可用）；也可把 `docs/dist/` 放到任意静态服务器
- **开发修改**（模块化，方便改样式/内容）：
  ```powershell
  cd docs
  npm install          # 首次安装依赖（独立于主项目）
  npm run dev          # 开发：http://localhost:5174，热更新
  npm run build        # 构建：产物输出 docs/dist/
  ```
- **章节结构**：`docs/src/chapters/` 下 Ch01 ~ Ch13 共 13 个独立组件，路由在 `docs/src/router/index.js`（hash 模式，无刷新切换）；全局样式在 `docs/src/style.css`，章节样式写在各组件内

---

## 六、常用环境变量（后端）

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | 监听端口（换端口：`$env:PORT="9000"; go run .`） |
| `DB_HOST` | `127.0.0.1` | MySQL 地址 |
| `DB_PORT` | `3306` | MySQL 端口 |
| `DB_USER` | `root` | MySQL 用户 |
| `DB_PASSWORD` | （空） | MySQL 密码 |
| `DB_NAME` | `blog` | 数据库名（自动创建） |
| `UPLOAD_DIR` | `uploads` | 图片上传目录 |
| `STATIC_DIR` | `../frontend/dist` | 前端构建产物目录 |
| `ADMIN_USER` | `admin` | 初始管理员用户名（仅首次启动生效） |
| `ADMIN_PASSWORD` | `admin123` | 初始管理员密码（仅首次启动生效） |
| `JWT_SECRET` | 自动生成并持久化到 settings 表 | JWT 签名密钥 |

PowerShell 示例（连接 docker-compose 的 MySQL）：

```powershell
$env:DB_USER="blog"; $env:DB_PASSWORD="blog123456"; go run .
```

---

## 七、常见问题排查

| 现象 | 原因与解决 |
|---|---|
| 第一次 `go run` 提示 `go: downloading ...` | 正常现象：Go 首次把依赖下载到本机缓存，一次性。下载慢/失败先设镜像：`$env:GOPROXY="https://goproxy.cn,direct"; go run .` |
| 启动报 `Error 1045: Access denied` | MySQL 账号密码错误，检查 `DB_USER` / `DB_PASSWORD` |
| 启动报 `Error 1049: Unknown database` | 账号没有建库权限；确认 MySQL 里已存在 `blog` 库且账号有权限（见「首次准备」） |
| 启动报 `dial tcp ... connect: connection refused` | MySQL 没启动：`docker compose up -d` 或本地启动 MySQL 服务 |
| 8080 端口被占用 | `$env:PORT="9000"; go run .`（前端代理需同步改 `vite.config.js`） |
| 5173 端口被占用 | `npm run dev -- --port 5174` |
| 前端能开但接口报「网络异常」 | 后端没启动或 MySQL 没连上；看后端终端日志 |
| 8080 打开的页面是旧内容 | 前端改过后没重新 `npm run build` |
| 登录提示「登录已过期」 | JWT 7 天有效，重新登录即可 |

---

## 八、接口一览

**公开接口**
- `GET  /api/posts` — 已发布文章分页列表（`?page=&pageSize=`）
- `GET  /api/posts/:id` — 文章详情
- `POST /api/posts/:id/like` — 点赞

**管理接口（需 `Authorization: Bearer <token>`）**
- `POST /api/auth/login` — 账密登录，返回 JWT（7 天有效）
- `GET  /api/admin/posts` — 全部文章（含草稿）
- `GET  /api/admin/posts/:id` — 文章详情（含草稿）
- `POST /api/admin/posts` — 新建文章
- `PUT  /api/admin/posts/:id` — 更新文章（部分字段）
- `DELETE /api/admin/posts/:id` — 删除文章
- `POST /api/admin/upload` — 上传图片（multipart，字段名 `file`，限 10MB，仅 jpg/png/gif/webp）

## 说明

- 点赞为简单计数（前端按浏览器本地去重），未做 IP 级防刷
- 评论区暂未开发，接口与数据模型已预留扩展空间
- 图片上传未做缩略图/鉴黄，个人使用场景足够
