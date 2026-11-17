#!/usr/bin/env bash
# ============================================================
# spblog —— 一行部署 / 一行更新（拉镜像，不需要源码、不需要 .env）
#
#   安装（第一次）：
#     spblog install
#
#   更新：
#     spblog update              # 拉新镜像 → 健康检查 → 切换（失败自动回滚）
#     spblog update --tag sha-abc1234   # 回退/切到某个具体版本
#
#   其它：
#     spblog status | logs | restart | down
#     spblog passwd <新密码>
#     spblog backup
#     spblog rollback
#
# 为什么不需要 .env：
#   数据库密码和站长密码首次安装时自动生成并写进容器自身的环境变量，
#   之后每次执行都从 `docker inspect` 读回来喂给 compose。
# 为什么不需要源码：
#   docker-compose.yml 由脚本内嵌生成，镜像里已包含前端产物和二进制。
# ============================================================
set -Eeuo pipefail
trap 'code=$?; [ "$code" = "0" ] || printf "\n✘ 脚本在第 %s 行失败（退出码 %s）：%s\n" "$LINENO" "$code" "$BASH_COMMAND" >&2' ERR

# ---------- 版本 ----------
# 改脚本时记得更新它：服务器上 `spblog version` 一眼就能看出「下载到的到底是不是新版」
SPBLOG_VERSION="2026-11-17.2"

# ---------- 可配置项 ----------
ROOT="${BLOG_DIR:-/opt/blog}"                 # 站点目录（只放 compose 文件）
# 留空表示「跟随已有部署」：resolve_image 会从正在运行的容器反查，
# 所以 install --image 阿里云/... 之后，直接 spblog update 也用同一个仓库。
IMAGE_REPO="${IMAGE_REPO:-}"
IMAGE_TAG="${IMAGE_TAG:-}"
APP_PORT="${APP_PORT:-127.0.0.1:8080}"        # 只监听本机，前面挂 Nginx
APP_NAME=blog-app
PG_NAME=blog-postgres
REDIS_NAME=blog-redis
COMPOSE_FILE="$ROOT/docker-compose.yml"

# ---------- 颜色 ----------
C_OK=$'\033[32m'; C_WARN=$'\033[33m'; C_ERR=$'\033[31m'; C_DIM=$'\033[2m'; C_END=$'\033[0m'
info() { printf '%s\n' "${C_DIM}▸${C_END} $*"; }
ok()   { printf '%s\n' "${C_OK}✔${C_END} $*"; }
warn() { printf '%s\n' "${C_WARN}!${C_END} $*"; }
die()  { printf '%s\n' "${C_ERR}✘${C_END} $*" >&2; exit 1; }

# ---------- 依赖 ----------
need_docker() {
  command -v docker >/dev/null 2>&1 || die "没装 Docker，先执行：curl -fsSL https://get.docker.com | bash"
  if docker compose version >/dev/null 2>&1; then
    DC="docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    DC="docker-compose"
  else
    die "缺少 docker compose 插件（apt install docker-compose-plugin）"
  fi
}

# ---------- 镜像地址 ----------
split_image() {
  local ref="$1" last="${1##*/}"
  if [ "$last" != "${last%%:*}" ]; then
    printf '%s %s' "${ref%:*}" "${last##*:}"
  else
    printf '%s %s' "$ref" "latest"
  fi
}

resolve_image() {
  if [ -z "$IMAGE_REPO" ]; then
    local cur repo tag
    cur="$(docker inspect -f '{{.Config.Image}}' "$APP_NAME" 2>/dev/null || true)"
    if [ -n "$cur" ]; then
      read -r repo tag <<<"$(split_image "$cur")"
      IMAGE_REPO="$repo"
      [ -z "$IMAGE_TAG" ] && IMAGE_TAG="$tag"
      info "沿用已有部署的镜像：${IMAGE_REPO}:${IMAGE_TAG}"
    else
      IMAGE_REPO="crpi-jlqf5sfrp7vsp54s.cn-hangzhou.personal.cr.aliyuncs.com/rineyce/spblog"
    fi
  fi
  [ -n "$IMAGE_TAG" ] || IMAGE_TAG="latest"
}

registry_host() {
  local h="${IMAGE_REPO%%/*}"
  case "$h" in
    *.*|*:*) printf '%s' "$h" ;;   # 形如 xxx.aliyuncs.com / registry:5000
    *)       printf '' ;;          # 否则是 Docker Hub 的用户名
  esac
}

# ---------- 私有仓库登录（可选） ----------
# 仓库公开就不用管；私有时二选一：
#   ① 服务器上先手动 docker login 一次（之后 install/update 都不用再带密码）
#   ② 临时传入：ACR_USER=xxx ACR_PASSWORD=xxx spblog install
ensure_login() {
  [ -n "${ACR_USER:-}" ] && [ -n "${ACR_PASSWORD:-}" ] || return 0
  local host
  host="$(registry_host)"
  if [ -n "$host" ]; then
    info "登录镜像仓库 $host"
    printf '%s' "$ACR_PASSWORD" | docker login "$host" -u "$ACR_USER" --password-stdin \
      || die "镜像仓库登录失败（检查账号密码，或仓库是否私有）"
  else
    info "登录 Docker Hub"
    printf '%s' "$ACR_PASSWORD" | docker login -u "$ACR_USER" --password-stdin \
      || die "Docker Hub 登录失败"
  fi
}

# ---------- 生成 compose（服务器上不需要仓库） ----------
write_compose() {
  mkdir -p "$ROOT"
  cat > "$COMPOSE_FILE" <<YAML
services:
  postgres:
    image: postgres:16-alpine
    container_name: $PG_NAME
    restart: unless-stopped
    environment:
      POSTGRES_DB: blog
      POSTGRES_USER: blog
      POSTGRES_PASSWORD: \${DB_PASSWORD:?missing}
      TZ: Asia/Shanghai
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -h 127.0.0.1 -U blog -d blog"]
      interval: 5s
      timeout: 3s
      retries: 20

  redis:
    image: redis:7-alpine
    container_name: $REDIS_NAME
    restart: unless-stopped
    command: ["redis-server", "--appendonly", "yes"]
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 20

  app:
    image: ${IMAGE_REPO}:${IMAGE_TAG}
    container_name: $APP_NAME
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      PORT: 8080
      STATIC_DIR: /app/dist
      UPLOAD_DIR: /app/uploads
      POST_CONTENT_DIR: /app/data/posts
      DB_HOST: postgres
      DB_PORT: 5432
      DB_USER: blog
      DB_PASSWORD: \${DB_PASSWORD:?missing}
      DB_NAME: blog
      DB_SSLMODE: disable
      ADMIN_USER: admin
      ADMIN_PASSWORD: \${ADMIN_PASSWORD:?missing}
      REDIS_ADDR: redis:6379
      TZ: Asia/Shanghai
    volumes:
      - uploads:/app/uploads
      - posts:/app/data/posts
    # 容器内的 DNS：很多 Ubuntu 主机的 /etc/resolv.conf 只写了 127.0.0.53
    # （systemd-resolved 的本地 stub），容器里访问不到，就会出现
    # "lookup postgres on 127.0.0.53:53: connection refused"。
    # 显式指定公共 DNS 之后，服务名（postgres / redis）仍然由 Docker 内嵌 DNS
    # （127.0.0.11）解析，外网域名则转发到下面这两个。
    dns:
      - 223.5.5.5
      - 119.29.29.29
    ports:
      - "${APP_PORT}:8080"
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8080/api/stats >/dev/null 2>&1 || exit 1"]
      interval: 10s
      timeout: 3s
      start_period: 10s
      retries: 6

volumes:
  pgdata:
  redisdata:
  uploads:
  posts:
YAML
  ok "已生成 $COMPOSE_FILE"
}

ensure_compose() {  # $1=force 时强制重新生成（install 用）
  if [ "${1:-}" = "force" ] || [ ! -f "$COMPOSE_FILE" ]; then
    write_compose
  fi
  cd "$ROOT"
}

# ---------- 密码：生成一次，之后从容器里读回 ----------
inspect_env() {
  docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$1" 2>/dev/null \
    | sed -n "s/^$2=//p" | head -1 || true
}

# 20 位字母数字。刻意避开 `| head -c`：head 读够就退出会让上游收到 SIGPIPE，
# 在 pipefail 下整条管道返回 141，赋值失败 + set -e 会把脚本静默杀掉。
gen_pass() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32 | tr -dc 'A-Za-z0-9' | cut -c1-20
  else
    local s=""
    while [ ${#s} -lt 40 ]; do
      s="$s$(od -An -tx1 -N32 /dev/urandom | tr -d ' \n')"
    done
    printf '%s' "$s" | cut -c1-20
  fi
}

load_secrets() {
  DB_PASSWORD="$(inspect_env "$PG_NAME" POSTGRES_PASSWORD)"
  ADMIN_PASSWORD="$(inspect_env "$APP_NAME" ADMIN_PASSWORD)"
  : "${DB_PASSWORD:=}"
  : "${ADMIN_PASSWORD:=}"
}

# ---------- 健康检查 ----------
wait_healthy() {
  local name="$1" timeout="${2:-180}" i=0 st
  while [ "$i" -lt "$timeout" ]; do
    st="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$name" 2>/dev/null || echo missing)"
    case "$st" in
      healthy)   return 0 ;;
      unhealthy) return 1 ;;
      running)   return 0 ;;
    esac
    sleep 1
    i=$((i + 1))
  done
  return 1
}

# ---------- 端口占用预检 ----------
# 常见情况：服务器上还留着旧的 systemd 部署（blog-server 占着 8080），
# 这时 docker 会报 "address already in use"，而且提示很不直观。
preflight_port() {
  local port="${APP_PORT##*:}" holder
  command -v ss >/dev/null 2>&1 || return 0
  holder="$(ss -lntp 2>/dev/null | grep -E "[:.]${port}[[:space:]]" | head -1 || true)"
  [ -n "$holder" ] || return 0
  case "$holder" in
    *docker*|*containerd*) return 0 ;;   # docker 自己占的（上一轮部署）不算冲突
  esac
  warn "端口 ${port} 已被其它进程占用："
  printf '%s\n' "     $holder"
  warn "如果是旧的 systemd 部署，先停掉再重试："
  printf '%s\n' "     systemctl stop blog && systemctl disable blog"
  printf '\n'
}

# ---------- 安装 ----------
cmd_install() {
  need_docker
  info "spblog v${SPBLOG_VERSION}"
  resolve_image
  ensure_login
  ensure_compose force
  load_secrets

  local fresh_admin=0
  [ -n "$DB_PASSWORD" ]    || { DB_PASSWORD="$(gen_pass)"; info "已生成数据库密码（随容器保存，无需记录）"; }
  [ -n "$ADMIN_PASSWORD" ] || { ADMIN_PASSWORD="$(gen_pass | cut -c1-16)"; fresh_admin=1; }
  export DB_PASSWORD ADMIN_PASSWORD

  info "拉取镜像 ${IMAGE_REPO}:${IMAGE_TAG}"
  $DC pull app || die "拉取失败。
   · 镜像是否已由 CI 推送？去 ACR 控制台看标签列表
   · 仓库是私有的？先登录一次：docker login $(registry_host || echo docker.io)
     或者把 ACR 仓库类型改成「公开」，之后就不用登录了"

  preflight_port
  info "启动数据库 / Redis / 应用"
  $DC up -d

  info "等待服务健康（首次要初始化数据库，稍慢）"
  if wait_healthy "$APP_NAME" 240; then
    ok "部署完成"
    printf '\n  访问地址：http://<服务器IP>:8080  （或经 Nginx 反代的域名）\n'
    printf '  后台入口：/admin/login\n\n'
    if [ "$fresh_admin" = "1" ]; then
      printf '  %s\n' "========== 站长账号（只显示这一次） =========="
      printf '   %s\n' "账号：admin"
      printf '   %s\n' "密码：$ADMIN_PASSWORD"
      printf '  %s\n\n' "=============================================="
      printf '  %s\n\n' "改密码：spblog passwd 新密码"
    else
      printf '  站长账号已存在（admin），密码未变。忘记可执行：spblog passwd 新密码\n\n'
    fi
  else
    warn "健康检查没过，最近日志："
    $DC logs --tail 60 app || true
    die "部署未完成"
  fi
}

# ---------- 更新 ----------
cmd_update() {
  need_docker
  load_secrets
  resolve_image
  ensure_login
  ensure_compose
  [ -n "$DB_PASSWORD" ] || die "没找到已有部署（$PG_NAME 容器不存在），请先执行 install"
  export DB_PASSWORD ADMIN_PASSWORD

  local rollback_id=""
  rollback_id="$(docker inspect -f '{{.Image}}' "$APP_NAME" 2>/dev/null || true)"

  info "拉取镜像 ${IMAGE_REPO}:${IMAGE_TAG}"
  $DC pull app || die "拉取失败，线上未改动。
   私有仓库请先登录：docker login $(registry_host || echo docker.io)"

  info "替换应用容器"
  $DC up -d --no-deps app

  info "等待新版本健康（最多 180 秒）"
  if wait_healthy "$APP_NAME" 180; then
    ok "更新完成：${IMAGE_REPO}:${IMAGE_TAG}"
    $DC ps
    return 0
  fi

  warn "新版本没起来，回滚"
  if [ -n "$rollback_id" ]; then
    docker tag "$rollback_id" "${IMAGE_REPO}:${IMAGE_TAG}" >/dev/null 2>&1 || true
    $DC up -d --no-deps app
    wait_healthy "$APP_NAME" 180 && { ok "已回滚到上一个镜像"; return 0; }
  fi
  die "更新失败：$DC logs --tail 80 app"
}

# ---------- 回滚 / 运维 ----------
cmd_rollback() {
  need_docker
  load_secrets
  resolve_image
  ensure_compose
  export DB_PASSWORD ADMIN_PASSWORD
  docker image inspect "${IMAGE_REPO}:previous" >/dev/null 2>&1 \
    || die "没有 previous 镜像。想回到某个提交：spblog update --tag sha-xxxxxxx"
  docker tag "${IMAGE_REPO}:previous" "${IMAGE_REPO}:${IMAGE_TAG}"
  $DC up -d --no-deps app
  wait_healthy "$APP_NAME" 180 && ok "回滚完成" || die "回滚后仍不健康"
}

cmd_passwd() {
  need_docker
  local pass="${1:-}"
  [ -n "$pass" ] || die "用法：spblog passwd 新密码（至少 6 位）"
  docker exec "$APP_NAME" /app/blog-server changepass "$pass"
  ok "已更新，用新密码登录 /admin/login"
}

cmd_backup() {
  need_docker
  local dir="$ROOT/backups" stamp
  stamp="$(date +%Y%m%d-%H%M%S)"
  mkdir -p "$dir"
  info "导出数据库"
  docker exec "$PG_NAME" pg_dump -U blog blog > "$dir/db-$stamp.sql"
  ok "数据库 → $dir/db-$stamp.sql"
  info "打包上传文件与正文"
  local tmp="$dir/.tmp-$stamp"
  mkdir -p "$tmp/uploads" "$tmp/posts"
  docker cp "$APP_NAME:/app/uploads/." "$tmp/uploads/" >/dev/null 2>&1 || true
  docker cp "$APP_NAME:/app/data/posts/." "$tmp/posts/" >/dev/null 2>&1 || true
  tar czf "$dir/files-$stamp.tgz" -C "$tmp" .
  rm -rf "$tmp"
  ok "上传文件与正文 → $dir/files-$stamp.tgz"
}

cmd_status() {
  need_docker
  resolve_image
  ensure_compose
  $DC ps
  printf '\n  镜像：%s:%s\n' "$IMAGE_REPO" "$IMAGE_TAG"
  printf '  配置：%s（由脚本生成，无 .env）\n' "$COMPOSE_FILE"
}

cmd_logs()    { need_docker; resolve_image; ensure_compose; $DC logs -f --tail 100 app; }
cmd_restart() {
  need_docker; load_secrets; resolve_image; ensure_compose; export DB_PASSWORD ADMIN_PASSWORD
  $DC up -d --no-deps app; ok "已重启"
}
cmd_down() {
  need_docker; load_secrets; resolve_image; ensure_compose; export DB_PASSWORD ADMIN_PASSWORD
  $DC down; ok "已停止（数据卷保留）"
}

usage() {
  cat <<'EOF'
用法：spblog <命令>

  install          首次部署：生成密码 → 拉镜像 → 启动（不需要 .env、不需要源码）
  update           更新：拉新镜像 → 健康检查 → 切换（失败自动回滚）
                     --tag sha-xxxxxxx  切到指定版本（可用于回退）
  rollback         回滚到上一个镜像
  passwd <新密码>   重置站长密码
  backup           备份数据库 + 上传文件
  status / logs / restart / down

可覆盖的环境变量：
  BLOG_DIR=/opt/blog        站点目录
  IMAGE_REPO=xxx/yyy        镜像仓库（默认阿里云 ACR: rineyce/spblog）
  IMAGE_TAG=latest          镜像标签（也可填 sha-xxxxxxx 回退到某次提交）
  APP_PORT=127.0.0.1:8080   端口绑定
  ACR_USER / ACR_PASSWORD   私有仓库账号（给了就自动 docker login）

一行部署示例：
  curl -fsSL <脚本地址> -o /usr/local/bin/spblog && chmod +x /usr/local/bin/spblog && spblog install
EOF
}

# ---------- 参数 ----------
CMD="${1:-}"
[ $# -gt 0 ] && shift
ARGS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --image)  IMAGE_REPO="${2:-}"; shift 2 ;;
    --tag)    IMAGE_TAG="${2:-}"; shift 2 ;;
    --dir)    ROOT="${2:-}"; COMPOSE_FILE="$ROOT/docker-compose.yml"; shift 2 ;;
    --port)   APP_PORT="${2:-}"; shift 2 ;;
    *)        ARGS+=("$1"); shift ;;
  esac
done

case "$CMD" in
  install)  cmd_install ;;
  update)   cmd_update ;;
  rollback) cmd_rollback ;;
  passwd)   cmd_passwd "${ARGS[0]:-}" ;;
  backup)   cmd_backup ;;
  status)   cmd_status ;;
  version)  printf 'spblog %s\n' "$SPBLOG_VERSION" ;;
  logs)     cmd_logs ;;
  restart)  cmd_restart ;;
  down)     cmd_down ;;
  *)        usage ;;
esac
