#!/usr/bin/env bash
# ============================================================
# blog.sh —— 一行命令安装 / 更新
#
#   安装（第一次）：
#     bash blog.sh install
#
#   更新（以后每次发版）：
#     bash blog.sh update
#
#   其它：
#     bash blog.sh status          看服务状态
#     bash blog.sh logs            跟日志
#     bash blog.sh restart         重启应用
#     bash blog.sh rollback        回滚到上一个镜像
#     bash blog.sh passwd 新密码    重置站长密码
#     bash blog.sh backup          备份数据库 + 图片 + 正文
#     bash blog.sh down            停止（数据保留）
#
# 源码从哪来：
#   · 默认用「脚本所在目录」的代码（所以要先 cd 到项目里）
#   · 也可以让脚本自己 clone：设 BLOG_REPO 和 BLOG_DIR
#       BLOG_REPO=https://github.com/你/blog.git BLOG_DIR=/opt/blog \
#         bash blog.sh install
#     已经 clone 过的话，update --pull 会 git pull
#
# 设计要点：
#   · update 是「先构建、再健康检查、失败自动回滚」——
#     构建失败或新版本起不来，线上仍然是旧版本，不会变成一堆 502
#   · 整个更新过程中数据库 / Redis 容器不动，只有 app 换一次
#   · 停机窗口 = app 容器重启的时间（1~3 秒）
# ============================================================
set -euo pipefail

# 脚本所在目录（curl | bash 时为空，退回当前目录）
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || pwd)"
# 项目根目录：优先 BLOG_DIR（配合 BLOG_REPO 自动克隆时用）
ROOT="${BLOG_DIR:-$SELF_DIR}"
REPO="${BLOG_REPO:-}"

IMAGE="rin-eyce/blog"
APP_NAME="blog-app"
PG_NAME="blog-postgres"
ENV_FILE="$ROOT/.env"
COMPOSE_FILE="$ROOT/docker-compose.yml"

# ---------- 颜色 ----------
C_OK=$'\033[32m'; C_WARN=$'\033[33m'; C_ERR=$'\033[31m'; C_DIM=$'\033[2m'; C_END=$'\033[0m'
info() { printf '%s\n' "${C_DIM}▸${C_END} $*"; }
ok()   { printf '%s\n' "${C_OK}✔${C_END} $*"; }
warn() { printf '%s\n' "${C_WARN}!${C_END} $*"; }
die()  { printf '%s\n' "${C_ERR}✘${C_END} $*" >&2; exit 1; }

# ---------- 源码就位 ----------
# 已经有代码就用；没有且给了 BLOG_REPO，就自动 clone。
ensure_source() {
  if [ -f "$COMPOSE_FILE" ]; then
    cd "$ROOT"
    return
  fi
  [ -n "$REPO" ] || die "没有找到源码（$COMPOSE_FILE）。
两种办法：
  ① 先 cd 到项目目录，再执行 bash blog.sh <命令>
  ② 让脚本自己拉：BLOG_REPO=<git 地址> BLOG_DIR=<目录> bash blog.sh <命令>"
  command -v git >/dev/null 2>&1 || die "没装 git：apt install -y git"
  info "克隆源码：$REPO → $ROOT"
  mkdir -p "$(dirname "$ROOT")"
  if [ -d "$ROOT/.git" ]; then
    git -C "$ROOT" pull --ff-only
  else
    git clone --depth 1 "$REPO" "$ROOT"
  fi
  [ -f "$COMPOSE_FILE" ] || die "克隆完成但没找到 docker-compose.yml，检查仓库结构"
  cd "$ROOT"
}

# ---------- 依赖检查 ----------
need_docker() {
  ensure_source
  command -v docker >/dev/null 2>&1 || die "没装 Docker，先执行：curl -fsSL https://get.docker.com | bash"
  if docker compose version >/dev/null 2>&1; then
    DC="docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    DC="docker-compose"
  else
    die "缺少 docker compose 插件（Docker 20.10+ 自带，或 apt install docker-compose-plugin）"
  fi
  [ -f "$COMPOSE_FILE" ] || die "当前目录没有 docker-compose.yml（应在项目根目录执行）"
}

# ---------- 等容器健康 ----------
# 参数：容器名 超时秒数；成功 0，失败 1
wait_healthy() {
  local name="$1" timeout="${2:-120}" i=0 st
  while [ "$i" -lt "$timeout" ]; do
    st="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$name" 2>/dev/null || echo missing)"
    case "$st" in
      healthy)   return 0 ;;
      unhealthy) return 1 ;;
      running)   # 容器没配 healthcheck 时，running 就当通过
                 return 0 ;;
    esac
    sleep 1
    i=$((i + 1))
  done
  return 1
}

# ---------- 首次安装 ----------
cmd_install() {
  need_docker

  if [ ! -f "$ENV_FILE" ]; then
    info "生成 .env（随机密码）"
    local dbpass adminpass
    dbpass="$(openssl rand -base64 32 2>/dev/null | tr -dc 'A-Za-z0-9' | cut -c1-20)"
    adminpass="$(openssl rand -base64 32 2>/dev/null | tr -dc 'A-Za-z0-9' | cut -c1-16)"
    cat > "$ENV_FILE" <<EOF
DB_NAME=blog
DB_USER=blog
DB_PASSWORD=$dbpass
ADMIN_USER=admin
ADMIN_PASSWORD=$adminpass
EOF
    chmod 600 "$ENV_FILE"
    ok "已生成 $ENV_FILE"
    printf '\n'
    printf '  %s\n' "==================== 请立即保存 ===================="
    printf '   %s  %s\n' "站长账号：" "admin"
    printf '   %s  %s\n' "站长密码：" "$adminpass"
    printf '   %s\n' "===================================================="
    printf '\n'
  else
    info ".env 已存在，沿用其中的密码"
  fi

  info "构建并启动（首次约 2~4 分钟）"
  $DC up -d --build

  info "等待服务健康"
  if wait_healthy "$APP_NAME" 180; then
    ok "启动成功"
    printf '\n  访问地址：http://127.0.0.1:8080  （或经 Nginx / 1Panel 反代的域名）\n'
    printf '  后台入口：http://你的域名/admin/login\n'
    printf '  查看日志：bash blog.sh logs\n'
    printf '  以后更新：bash blog.sh update\n\n'
  else
    warn "健康检查没过，打印最近日志："
    $DC logs --tail 40 app || true
    die "安装未完成，请把上面的报错贴出来"
  fi
}

# ---------- 更新 ----------
# 可选 --pull：如果当前目录是 git 仓库，先拉最新代码
cmd_update() {
  need_docker
  local do_pull=0
  [ "${1:-}" = "--pull" ] && do_pull=1

  if [ "$do_pull" = "1" ]; then
    if [ -d "$ROOT/.git" ]; then
      info "拉取最新代码"
      git -C "$ROOT" fetch --all --quiet
      git -C "$ROOT" reset --hard '@{u}' --quiet || git -C "$ROOT" reset --hard --quiet
    else
      warn "当前目录不是 git 仓库，跳过拉代码"
    fi
  fi

  # 把当前线上镜像打上 previous 标签，方便回滚
  if docker image inspect "${IMAGE}:latest" >/dev/null 2>&1; then
    docker tag "${IMAGE}:latest" "${IMAGE}:previous"
    info "已把当前版本标记为 previous（可用于 rollback）"
  fi

  info "构建新镜像（构建期间线上仍是旧版本）"
  if ! $DC build app; then
    die "构建失败，线上未改动"
  fi

  info "替换应用容器"
  $DC up -d --no-deps app

  info "等待新版本健康（最多 120 秒）"
  if wait_healthy "$APP_NAME" 120; then
    ok "更新完成"
    $DC ps
  else
    warn "新版本没起来，自动回滚"
    if docker image inspect "${IMAGE}:previous" >/dev/null 2>&1; then
      docker tag "${IMAGE}:previous" "${IMAGE}:latest"
      $DC up -d --no-deps app
      if wait_healthy "$APP_NAME" 120; then
        ok "已回滚到上一个版本"
      else
        die "回滚也失败了，请手动检查：$DC logs --tail 80 app"
      fi
    fi
    die "更新失败（已回滚）"
  fi
}

# ---------- 回滚 ----------
cmd_rollback() {
  need_docker
  docker image inspect "${IMAGE}:previous" >/dev/null 2>&1 \
    || die "没有 previous 镜像可回滚（只保留最近一个版本）"
  info "把 previous 提升为 latest 并重启"
  docker tag "${IMAGE}:previous" "${IMAGE}:latest"
  $DC up -d --no-deps app
  wait_healthy "$APP_NAME" 120 && ok "回滚完成" || die "回滚后仍不健康，看日志：$DC logs --tail 80 app"
}

# ---------- 重置站长密码 ----------
cmd_passwd() {
  need_docker
  local pass="${1:-}"
  [ -n "$pass" ] || die "用法：bash blog.sh passwd 新密码（至少 6 位）"
  info "在容器内重置 admin 的密码"
  docker exec "$APP_NAME" /app/blog-server changepass "$pass"
  ok "已更新，用新密码登录 /admin/login"
}

# ---------- 备份 ----------
cmd_backup() {
  need_docker
  local dir="$ROOT/backups" stamp
  stamp="$(date +%Y%m%d-%H%M%S)"
  mkdir -p "$dir"

  # 从 .env 读库名和用户名
  # shellcheck disable=SC1090
  local db_name db_user
  db_name="$(grep -E '^DB_NAME=' "$ENV_FILE" | cut -d= -f2- || echo blog)"
  db_user="$(grep -E '^DB_USER=' "$ENV_FILE" | cut -d= -f2- || echo blog)"

  info "导出数据库"
  docker exec "$PG_NAME" pg_dump -U "$db_user" "$db_name" > "$dir/db-$stamp.sql"
  ok "数据库 → $dir/db-$stamp.sql"

  info "打包上传文件与文章正文"
  local tmp="$dir/.tmp-$stamp"
  mkdir -p "$tmp/uploads" "$tmp/posts"
  docker cp "$APP_NAME:/app/uploads/." "$tmp/uploads/" >/dev/null 2>&1 || true
  docker cp "$APP_NAME:/app/data/posts/." "$tmp/posts/" >/dev/null 2>&1 || true
  tar czf "$dir/files-$stamp.tgz" -C "$tmp" .
  rm -rf "$tmp"
  ok "上传文件与正文 → $dir/files-$stamp.tgz"
}

# ---------- 其它 ----------
cmd_status()  { need_docker; $DC ps; printf '\n'; docker inspect -f '{{.Name}}  健康：{{if .State.Health}}{{.State.Health.Status}}{{else}}-{{end}}' "$APP_NAME" "$PG_NAME" 2>/dev/null || true; }
cmd_logs()    { need_docker; $DC logs -f --tail 100 app; }
cmd_restart() { need_docker; $DC restart app; ok "已重启"; }
cmd_down()    { need_docker; $DC down; ok "已停止（数据卷保留）"; }

usage() {
  cat <<'EOF'
用法：bash blog.sh <命令>

  install          首次安装：生成 .env、构建、启动、等健康
  update [--pull]  更新：构建新镜像 → 健康检查 → 切换（失败自动回滚）
                   加 --pull 会先 git 拉取最新代码
  rollback         回滚到上一个版本
  passwd <新密码>   重置站长密码
  backup           备份数据库 + 上传文件
  status           查看服务状态
  logs             跟踪应用日志
  restart          重启应用
  down             停止全部服务（数据保留）

源码位置：
  默认用脚本所在目录。也可以让脚本自己拉：
    BLOG_REPO=<git 地址> BLOG_DIR=<目录> bash blog.sh install
EOF
}

case "${1:-}" in
  install)  cmd_install ;;
  update)   shift; cmd_update "${1:-}" ;;
  rollback) cmd_rollback ;;
  passwd)   shift; cmd_passwd "${1:-}" ;;
  backup)   cmd_backup ;;
  status)   cmd_status ;;
  logs)     cmd_logs ;;
  restart)  cmd_restart ;;
  down)     cmd_down ;;
  *)        usage ;;
esac
