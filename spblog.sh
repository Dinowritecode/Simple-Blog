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
SPBLOG_VERSION="2026.11.20-1708"

# ---------- 可配置项 ----------
ROOT="${BLOG_DIR:-/opt/blog}"                 # 站点目录（只放 compose 文件）
# 留空表示「跟随已有部署」：resolve_image 会从正在运行的容器反查，
# 所以 install --image 阿里云/... 之后，直接 spblog update 也用同一个仓库。
IMAGE_REPO="${IMAGE_REPO:-}"
IMAGE_TAG="${IMAGE_TAG:-}"
APP_PORT="${APP_PORT:-127.0.0.1:8080}"        # 只监听本机，前面挂 Nginx
APP_NAME=blog-app
PG_NAME=blog-postgres
ADMIN_LOGIN_NAME="${ADMIN_USER:-admin}"   # 站长登录名（后端用 ADMIN_USER 建号，默认 admin）
REDIS_NAME=blog-redis
COMPOSE_FILE="$ROOT/docker-compose.yml"

# ---------- 颜色 ----------
C_OK=$'\033[32m'; C_WARN=$'\033[33m'; C_ERR=$'\033[31m'; C_DIM=$'\033[2m'; C_END=$'\033[0m'
# 进度条配色：已完成 = 蔚蓝（#00AAEE，与站点主题色一致），未完成 = 白
C_BAR=$'\033[38;2;0;170;238m'
C_BAR_EMPTY=$'\033[38;2;255;255;255m'
# 交互提问用浅绿（92 = 亮绿），和普通的 ▸ 灰阶、✔ 绿区分开
C_ASK=$'\033[92m'
info() { printf '%s\n' "${C_DIM}▸${C_END} $*"; }
ok()   { printf '%s\n' "${C_OK}✔${C_END} $*"; }
warn() { printf '%s\n' "${C_WARN}!${C_END} $*"; }
die()  { printf '%s\n' "${C_ERR}✘${C_END} $*" >&2; exit 1; }

# ---------- 交互提问 ----------
# 规则：只在真终端里问（curl | bash、重定向、CI 里自动跳过，不会卡住），
# 想完全跳过问询：SPBLOG_NO_ASKS=1（或 install 时带 --yes）
can_ask() {
  [ "${SPBLOG_NO_ASKS:-0}" = "1" ] && return 1
  [ "${SPBLOG_FORCE_ASKS:-0}" = "1" ] && return 0   # 调试/测试用：即使没有 TTY 也照常提问
  [ -t 0 ]
}

# 打印一段浅绿色的提问（不带换行，光标停在问题后面等输入）
ask() { printf '  %s%s%s ' "$C_ASK" "$1" "$C_END"; }

# 安装前的确认：y / Y / yes 才算同意，其它一律取消
confirm_install() {
  [ "${SPBLOG_YES:-0}" = "1" ] && return 0
  can_ask || return 0
  local ans=""
  ask "是否安装 Simple-blog (y)？"
  read -r ans || ans=""
  [ -t 0 ] || printf "\n"
  case "$ans" in
    y|Y|yes|YES|是) return 0 ;;
    *)              return 1 ;;
  esac
}

# ---------- 启动字符画 ----------
# 用 base64(gzip(ANSI)) 内嵌：脚本仍是纯文本，不怕编码/换行被破坏，体积也只有 1.2 KB。
# 终端太窄（<120 列）时自动换成小横幅，避免 120 列的画面被折成乱码。
# 强制显示大图：SPBLOG_BANNER=always   完全关闭：SPBLOG_NO_BANNER=1
BANNER_B64='H4sICHhK/GoCA2xvZ29fYW5zaV8xMjAudHh0ALVWUYrjMAz93yvMT8CfBjEzS2FQEfgGOsCcoehD94eV5DjJUjmzC6mgaWNH0nvSs9JleZm9ff/+un/eP77uH/7j9v6At+/3x69XBAbIH6WLUuZZSVXX5c/bbXwelYgwDCaOejmsY34qmCVFqhVnPmBon32I8ZVAkae1xVo10CLDBO51NUyxKZy3XqTRtJomgOvQ5fgIz/AJY1WtBDOAFa4EOEmCOMPH2t1MlOQgMxn/fUoSjdAwTzOdAQdDOlTtpLREkGGmeR/QWWSHDjHzKgdqz1617VZtiLUmPzGD0hr+PBksoGbMMHlY7WE7oNha6ZVFTnqw8XjeswCCsJq5izgZPVj15f3OonAwdu2GYS6s0koyrVBgRppGgbbIfnZp3CgNGskMNHfZnbB4ObwoIhaShaNJDkkklmxbvGPArbDEvqYsbIeSfJpOI7UEtISPY7DEazJf2m5OaAiHlRbYyhqnc+EQmV2lq6hvV1/yn1JtGsd+As07lqRLz4pnt5eTp9FdWOLJDpmfRmcqLYYulo4VtrZAj7estdp4yWqpRiQngdNjZGl0FDLOXXVKtlRcJ7yRSKAPIBwc1P1W2bvbGtEPXynerDLI0RrcLOVQczlljfD00QkKDLJGDtku1BGWCGcckvnEm8WRYJ/mNK62on712Pt4tiX7AhmONT8STOmMSaTk5gPFQ5qMhr7ZZgbUbhLT7l/fctlcjNDnr5T/fi1u1RO69BU8+7/9B4g3H5njCwAA'

show_banner() {
  # $1=compact 时不额外留空行（部署流程用，避免字符画和第一条提示之间空一大块）
  local compact="${1:-}"
  [ "${SPBLOG_NO_BANNER:-0}" = "1" ] && return 0
  [ "${SPBLOG_BANNER_DONE:-0}" = "1" ] && return 0
  SPBLOG_BANNER_DONE=1
  local cols="${COLUMNS:-}"
  [ -n "$cols" ] || cols="$(tput cols 2>/dev/null || echo 0)"
  case "$cols" in ''|*[!0-9]*) cols=0 ;; esac

  if [ "${SPBLOG_BANNER:-auto}" = "always" ] || [ "$cols" -eq 0 ] || [ "$cols" -ge 120 ]; then
    printf '%s' "$BANNER_B64" | base64 -d | gzip -dc
  else
    printf '\n'
    printf '  ─────────────────────────────────────\n'
    printf '   spblog · 凛冬 RinEyce\n'
    printf '   一行部署 / 一行更新\n'
    printf '  ─────────────────────────────────────\n'
  fi

  # 字符画下方两行标识（蔚蓝，左对齐、不居中）：
  #   第 1 行：Simple-Blog@版本    RinEyce©2026   （中间空三格）
  #   第 2 行：一句话介绍
  # 版本号由 git pre-commit 钩子自动写入（见 .githooks/pre-commit），提交时无需手改
  local indent=""
  [ "${cols:-0}" -lt 120 ] 2>/dev/null && indent="  "   # 紧凑横幅时缩进 2 格，和方框对齐
  show_tagline "$indent"

  [ "$compact" = "compact" ] || printf '\n'
}

# 两行标识：全部用蔚蓝
TAGLINE_DESC="A fast, modern, and multifunctional personal blog system"

show_tagline() {
  local indent="${1:-}"
  # 版本号每次现取（自更新后可能已变）
  printf '%s%s%s   %s%s\n' "$indent" "$C_BAR" "Simple-Blog@${SPBLOG_VERSION}" "RinEyce©2026" "$C_END"
  printf '%s%s%s%s\n' "$indent" "$C_BAR" "$TAGLINE_DESC" "$C_END"
}

# ---------- 自更新 ----------
# `spblog update` 只换容器镜像，**脚本本身不会跟着变** —— 脚本是服务器上的一个文件，
# 不在镜像里。所以这里加一步：执行 install / update 前先看看远端脚本的版本号，
# 不一致就把自己替换掉并重新执行（exec），这样以后改脚本只要 push，服务器不用手动下载。
#
# 关掉：SPBLOG_NO_SELF_UPDATE=1    换下载源：SPBLOG_URL=https://…
SPBLOG_SELF_URL="${SPBLOG_URL:-https://raw.githubusercontent.com/Dinowritecode/Simple-Blog/main/spblog.sh}"
SPBLOG_MIRROR="https://ghfast.top/"

self_update() {
  [ "${SPBLOG_NO_SELF_UPDATE:-0}" = "1" ] && return 0
  # 只对「磁盘上的一个真实文件」做自更新；curl | bash 时 $0 是 bash，不能覆盖
  [ -f "$0" ] && [ -w "$0" ] || return 0
  command -v curl >/dev/null 2>&1 || return 0

  local tmp newver self
  tmp="$(mktemp 2>/dev/null || echo "/tmp/spblog.$$")"

  if ! curl -fsSL --connect-timeout 4 --max-time 10 "$SPBLOG_SELF_URL" -o "$tmp" 2>/dev/null; then
    # 直连不通（国内访问 raw.githubusercontent 经常超时）就走镜像再试一次
    if ! curl -fsSL --connect-timeout 4 --max-time 14 "${SPBLOG_MIRROR}${SPBLOG_SELF_URL}" -o "$tmp" 2>/dev/null; then
      rm -f "$tmp"
      return 0   # 拿不到就算了，不影响本次部署
    fi
  fi

  newver="$(tr -d '\r' < "$tmp" | sed -n 's/^SPBLOG_VERSION="\(.*\)"/\1/p' | head -1)"
  if [ -z "$newver" ]; then
    rm -f "$tmp"
    return 0   # 下载到的不是脚本（镜像返回了错误页之类），忽略
  fi
  if [ "$newver" = "$SPBLOG_VERSION" ]; then
    rm -f "$tmp"
    return 0   # 已经是最新
  fi

  self="$0"
  info "发现脚本新版本：$SPBLOG_VERSION → $newver"
  cp "$self" "${self}.bak" 2>/dev/null || true
  if cat "$tmp" > "$self" 2>/dev/null; then
    chmod +x "$self" 2>/dev/null || true
    rm -f "$tmp"
    ok "脚本已更新（旧版备份在 ${self}.bak），重新执行 spblog $*"
    printf '\n'
    exec "$self" "$@"
  fi
  rm -f "$tmp"
  warn "脚本无法写入（$self），跳过自更新；请手动下载覆盖"
  return 0
}

# ---------- dry-run：本地调试整个部署流程 ----------
# 打开后 docker 命令只打印不执行，其余逻辑（生成 compose、密码、进度条、
# 健康等待、回滚判断）全部照常跑 —— 不需要服务器、不需要 Docker。
#   spblog debug            跑 install + update 两段
#   spblog debug install    只跑 install
#   spblog debug update     只跑 update
install_dry_run_stub() {
  docker() {
    # 查询类命令（inspect / image inspect）只返回假数据、**不打印** ——
    # 它们的 stdout 会被 $(...) 读进变量（健康检查就是这样），
    # 一旦打印出来，变量里就不是 healthy 而是日志文本，循环会一直转。
    case "$1" in
      inspect)
        case "$*" in
          *".State.Health"*)
            # SPBLOG_DRY_FAIL=1 可模拟「新版本起不来」，用来调试回滚分支
            if [ "${SPBLOG_DRY_FAIL:-0}" = "1" ]; then echo unhealthy; else echo healthy; fi ;;
          *".Image"*) echo "${IMAGE_REPO:-crpi-x/rineyce/spblog}:${IMAGE_TAG:-latest}" ;;
          *".Config.Env"*)
            printf 'POSTGRES_PASSWORD=dry-run-db-pass\nADMIN_PASSWORD=dry-run-admin\n' ;;
        esac
        return 0 ;;
      image) return 0 ;;
    esac

    # 调试用：想看具体执行了哪些 docker 命令就设 SPBLOG_DEBUG_CMD=1
    # 默认不打印，保证 dry-run 的输出和服务器上**一模一样**
    [ "${SPBLOG_DEBUG_CMD:-0}" = "1" ] && printf '  \033[2m[dry-run]\033[0m docker %s\n' "$*" >&2
    case "$1" in
      compose)
        case "$2" in
          ps) printf 'NAME         STATUS\n%-12s Up (healthy)\n' "${APP_NAME:-blog-app}" ;;
        esac ;;
    esac
    return 0
  }
}

cmd_debug() {
  local what="${1:-all}"
  shift || true
  export SPBLOG_DRY_RUN=1
  export SPBLOG_NO_SELF_UPDATE=1
  export SPBLOG_NO_VERSION_CHECK=1
  # 默认写到当前目录下的 .dryrun，不去碰 /opt/blog
  if [ -z "${BLOG_DIR:-}" ]; then
    BLOG_DIR="$PWD/.dryrun"
  fi
  ROOT="$BLOG_DIR"
  COMPOSE_FILE="$ROOT/docker-compose.yml"
  install_dry_run_stub
  mkdir -p "$ROOT"

  # 刻意什么都不额外打印：dry-run 的输出要和服务器上完全一致
  # （想看命令就 SPBLOG_DEBUG_CMD=1）
  case "$what" in
    install)  cmd_install ;;
    update)   cmd_update ;;
    passwd)   cmd_passwd "${1:-new-password-123}" ;;
    rollback) cmd_rollback ;;
    version)  printf 'spblog %s\n' "$SPBLOG_VERSION" ;;
    backup)   cmd_backup ;;
    status)   cmd_status ;;
    *)        cmd_install; cmd_update ;;
  esac
}

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
      # 当前镜像版本：后台「总览」右侧展示用，取的就是部署时的 tag
      APP_VERSION: \${IMAGE_TAG:-latest}
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

# ---------- 进度条 ----------
# 完成部分蔚蓝（#00AAEE，与站点主题色一致），未完成部分白色。
# 只在真终端里画：输出被重定向/管道时自动关掉，避免日志里一堆 \r。
# 强制开启：SPBLOG_FORCE_PROGRESS=1   关闭：SPBLOG_NO_PROGRESS=1
BAR_W=40            # 细进度条长度（pip 风格是「一条细线」，不是粗块）
BAR_PCT=0
BAR_START=0
BAR_DRAWN=0         # 当前是否已经画出一行进度条（用于「提示信息上移到条上方」）

bar_on() {
  [ "${SPBLOG_NO_PROGRESS:-0}" = "1" ] && return 1
  [ "${SPBLOG_FORCE_PROGRESS:-0}" = "1" ] && return 0
  [ -t 1 ]
}

# 重复输出一个字符 N 次（纯 bash，不依赖 seq）
_rep() {
  local n="$1" ch="$2" s=""
  while [ "$n" -gt 0 ]; do s="$s$ch"; n=$((n - 1)); done
  printf '%s' "$s"
}

# 秒 → mm:ss
_fmt_time() {
  local s="$1"
  printf '%d:%02d' $(( s / 60 )) $(( s % 60 ))
}

# 只重绘进度条本身（不改变状态）
bar_render() {
  local fill empty elapsed
  fill=$(( BAR_PCT * BAR_W / 100 ))
  [ "$fill" -gt "$BAR_W" ] && fill=$BAR_W
  empty=$(( BAR_W - fill ))
  elapsed=$( _fmt_time $(( $(date +%s) - BAR_START )) )
  # pip 风格：4 空格缩进 + 一条细线（完成段蔚蓝、未完成段白）+ 百分比 + 已用时间
  printf '\r    %s%s%s%s%s%s %3d%%  %s\033[K' \
    "$C_BAR" "$(_rep "$fill" '━')" "$C_END" \
    "$C_BAR_EMPTY" "$(_rep "$empty" '━')" "$C_END" \
    "$BAR_PCT" "$elapsed"
  BAR_DRAWN=1
}

bar_draw() {
  BAR_PCT="$1"
  [ "$BAR_PCT" -lt 0 ] && BAR_PCT=0
  [ "$BAR_PCT" -gt 100 ] && BAR_PCT=100
  bar_render
}

# 进度条画满并换行
bar_done() {
  bar_on || return 0
  BAR_PCT=100
  bar_render
  printf '\n'
  BAR_DRAWN=0
}

# 在进度条**上方**输出一行提示：先把条擦掉 → 打日志 → 再画回来
bar_msg() {
  if bar_on && [ "$BAR_DRAWN" = "1" ]; then
    printf '\r\033[K'
    info "$*"
    bar_render
  else
    info "$*"
  fi
}

# run_with_bar "标签" 命令…  —— 命令在后台跑，前台按时间推进进度
# 命令输出先落临时文件：成功就丢弃，失败才打出来，避免和进度条互相刷新
run_with_bar() {
  local label="$1"; shift
  [ "${SPBLOG_DEBUG_CMD:-0}" = "1" ] && info "  \$ $*"
  if ! bar_on; then
    info "$label"
    "$@"
    return $?
  fi

  local log pid rc t=0
  log="$(mktemp 2>/dev/null || echo "/tmp/spblog.cmd.$$")"

  info "$label"
  BAR_START="$(date +%s)"
  BAR_PCT=0
  "$@" >"$log" 2>&1 &
  pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    # 越接近完成推进越慢（90% 封顶），等真正结束再补到 100%
    BAR_PCT=$(( 90 * t / (t + 5) ))
    [ "$BAR_PCT" -gt 90 ] && BAR_PCT=90
    bar_render
    sleep 0.2
    t=$((t + 1))
  done
  wait "$pid"; rc=$?

  if [ "$rc" -eq 0 ]; then
    bar_done
    rm -f "$log"
  else
    printf '\n'
    BAR_DRAWN=0
    sed 's/^/    /' "$log" | tail -30
    rm -f "$log"
  fi
  return $rc
}

# 跑一条命令并把输出藏起来（成功只报一句完成，失败时把日志打出来）
# 用于「启动容器」这类不需要进度条、但输出又很吵的步骤
run_quiet() {
  local label="$1"; shift
  [ "${SPBLOG_DEBUG_CMD:-0}" = "1" ] && info "  \$ $*"
  local log rc
  log="$(mktemp 2>/dev/null || echo "/tmp/spblog.q.$$")"
  if "$@" >"$log" 2>&1; then
    rm -f "$log"
    ok "$label"
    return 0
  fi
  rc=$?
  sed 's/^/    /' "$log" | tail -30
  rm -f "$log"
  return $rc
}

# ---------- 健康检查 ----------
# 不再画进度条：静默轮询，成功时报一句耗时。
# $1=容器名 $2=超时秒 $3=成功提示里用的名字（可选）
wait_healthy() {
  local name="$1" timeout="${2:-180}" label="${3:-服务}" i=0 st start
  start="$(date +%s)"
  while [ "$i" -lt "$timeout" ]; do
    st="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$name" 2>/dev/null || echo missing)"
    case "$st" in
      healthy|running)
        ok "${label}已就绪（耗时 $( _fmt_time $(( $(date +%s) - start )) )）"
        return 0 ;;
      unhealthy)
        return 1 ;;
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

# ---------- 装完设置站长账号 ----------
# 账号是后端首次启动时按 ADMIN_PASSWORD 自动建的，所以这里不是「重建账号」，而是：
#   ① 要换用户名 → 直接在库里改 users.username（psql 在 postgres 容器里）
#   ② 要换密码   → 调二进制自带的 changepass（与登录同一套 bcrypt）
setup_admin_interactive() {
  can_ask || return 0

  printf '\n'
  info "请完成网站初始设置"
  local user pass pass2 tries=0 valid=0

  ask "站长用户名 [${ADMIN_LOGIN_NAME}]："
  read -r user || user=""
  [ -t 0 ] || printf "\n"
  user="${user:-$ADMIN_LOGIN_NAME}"
  case "$user" in
    ''|*[!A-Za-z0-9_-]*)
      warn "用户名只能用字母、数字、下划线或短横线，已跳过"
      return 0 ;;
  esac

  # 密码不符合规范就**重新问**（最多 3 轮），而不是直接跳过
  while [ "$tries" -lt 3 ]; do
    tries=$((tries + 1))

    ask "站长密码（至少 6 位）："
    read -rs pass || pass=""
    printf '\n'
    if [ -z "$pass" ]; then
      return 0
    fi
    if [ "${#pass}" -lt 6 ]; then
      warn "密码至少 6 位（第 ${tries}/3 次），请重新输入"
      continue
    fi

    ask "确认密码："
    read -rs pass2 || pass2=""
    printf '\n'
    if [ "$pass" != "$pass2" ]; then
      warn "两次输入不一致（第 ${tries}/3 次），请重新设置"
      continue
    fi

    valid=1
    break
  done

  if [ "$valid" != "1" ]; then
    warn "连续 3 次不符合要求，已跳过"
    return 0
  fi

  # ① 换用户名（只改 role='admin' 那一行）
  if [ "$user" != "$ADMIN_LOGIN_NAME" ]; then
    if docker exec "$PG_NAME" psql -U blog -d blog -q -v ON_ERROR_STOP=1 \
         -c "UPDATE users SET username = '$user' WHERE username = '$ADMIN_LOGIN_NAME' AND role = 'admin'" \
         >/dev/null 2>&1; then
      ok "站长用户名已改为 ${user}"
      ADMIN_LOGIN_NAME="$user"
    else
      warn "用户名修改失败（可能已存在同名账号）"
    fi
  fi

  # ② 换密码
  if docker exec "$APP_NAME" /app/blog-server changepass "$ADMIN_LOGIN_NAME" "$pass" >/dev/null 2>&1; then
    ok "站长密码已更新，请用新密码登录 /admin/login"
  else
    warn "密码更新失败，可稍后执行：spblog passwd <新密码>"
  fi
}

# ---------- 安装 ----------
cmd_install() {
  self_update install
  show_banner compact

  # 1) 先问一句要不要装（浅绿提问，y 才继续）
  if ! confirm_install; then
    warn "已取消安装"
    return 0
  fi

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

  if ! run_with_bar "拉取镜像 ${IMAGE_REPO}:${IMAGE_TAG}" $DC pull app; then
    die "拉取失败。"
  fi

  preflight_port
  bar_msg "Starting Redis"
  run_quiet "The container has been successfully started" $DC up -d || die "Container startup failed"

  if wait_healthy "$APP_NAME" 240 "服务"; then
    ok "部署完成"
    printf '\n  访问地址：http://<服务器IP>:8080 \n'
    printf '  后台入口：/admin/login\n\n'
    if [ "$fresh_admin" = "1" ]; then
      printf '   %s\n' "账号：admin"
      printf '   %s\n' "密码：$ADMIN_PASSWORD"
      printf '  %s\n\n' "若要修改密码请执行：spblog passwd <newpassword>"
    else
      printf '  站长账号已存在\n'
    fi
    # 2) 装完问一次：要不要自定义站长用户名 / 密码
    setup_admin_interactive
  else
    warn "Health status check failed"
    $DC logs --tail 60 app || true
    die "部署未完成"
  fi
}

# ---------- 更新 ----------
cmd_update() {
  self_update update
  need_docker
  load_secrets
  resolve_image
  ensure_login
  ensure_compose
  [ -n "$DB_PASSWORD" ] || die "没找到已有部署（$PG_NAME 容器不存在），请先执行 install"
  export DB_PASSWORD ADMIN_PASSWORD

  local rollback_id=""
  rollback_id="$(docker inspect -f '{{.Image}}' "$APP_NAME" 2>/dev/null || true)"

  if ! run_with_bar "拉取镜像 ${IMAGE_REPO}:${IMAGE_TAG}" $DC pull app; then
    die "拉取失败，线上未改动。
   私有仓库请先登录：docker login $(registry_host || echo docker.io)"
  fi

  run_quiet "应用容器已替换" $DC up -d --no-deps app || die "替换容器失败"

  if wait_healthy "$APP_NAME" 180 "新版本"; then
    ok "更新完成：${IMAGE_REPO}:${IMAGE_TAG}"
    $DC ps
    return 0
  fi

  warn "新版本没起来，回滚"
  if [ -n "$rollback_id" ]; then
    docker tag "$rollback_id" "${IMAGE_REPO}:${IMAGE_TAG}" >/dev/null 2>&1 || true
    $DC up -d --no-deps app
    wait_healthy "$APP_NAME" 180 "回滚后服务" && { ok "已回滚到上一个镜像"; return 0; }
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

# ---------- 版本检查（只提示，不自动改） ----------
# `spblog` 时顺手看看远端有没有新版：结果缓存 6 小时，避免每次敲命令都联网等几秒。
# 关掉：SPBLOG_NO_VERSION_CHECK=1
fetch_remote_version() {
  command -v curl >/dev/null 2>&1 || return 0
  local tmp
  tmp="$(mktemp 2>/dev/null || echo "/tmp/spblog.ver.$$")"
  if curl -fsSL --connect-timeout 3 --max-time 6 "$SPBLOG_SELF_URL" -o "$tmp" 2>/dev/null \
     || curl -fsSL --connect-timeout 3 --max-time 8 "${SPBLOG_MIRROR}${SPBLOG_SELF_URL}" -o "$tmp" 2>/dev/null; then
    tr -d '\r' < "$tmp" | sed -n 's/^SPBLOG_VERSION="\(.*\)"/\1/p' | head -1
  fi
  rm -f "$tmp"
}

# 检查远端版本；有更新时**在标识下方**打一行绿色提示。
# 结果缓存 6 小时，避免每次敲 spblog 都联网等几秒。
# 关掉：SPBLOG_NO_VERSION_CHECK=1
check_new_version() {
  [ "${SPBLOG_NO_VERSION_CHECK:-0}" = "1" ] && return 0

  local cache="${TMPDIR:-/tmp}/spblog-latest-version" now remote="" ts ver
  now="$(date +%s)"

  # 缓存格式：<时间戳> <版本号>
  if [ -f "$cache" ]; then
    read -r ts ver < "$cache" 2>/dev/null || true
    ver="${ver%$'\r'}"   # 缓存文件可能是 CRLF（Windows 写出来的），去掉尾部回车
    case "${ts:-}" in ''|*[!0-9]*) ts=0 ;; esac
    if [ -n "${ver:-}" ] && [ $((now - ts)) -lt 21600 ]; then
      remote="$ver"
    fi
  fi

  if [ -z "$remote" ]; then
    remote="$(fetch_remote_version)"
    [ -n "$remote" ] && printf '%s %s\n' "$now" "$remote" > "$cache" 2>/dev/null
  fi

  [ -n "$remote" ] || return 0
  [ "$remote" = "$SPBLOG_VERSION" ] && return 0

  printf '  %s发现新版本(%s)，运行 spblog update 以应用更新%s\n\n' "$C_ASK" "$remote" "$C_END"
}

# ---------- 命令列表 ----------
# 命令名字用「蔚蓝」（与站点主题色 #00AAEE 同一个色值），解释保持普通文字色。
# 输出被重定向 / 管道时不加颜色（避免日志里出现转义码），
# 需要强制上色用 SPBLOG_FORCE_COLOR=1，关掉用 SPBLOG_NO_COLOR=1。
print_commands() {
  local c='' w='' r=''
  if [ "${SPBLOG_FORCE_COLOR:-0}" = "1" ] || { [ -t 1 ] && [ "${SPBLOG_NO_COLOR:-0}" != "1" ]; }; then
    c=$'\033[38;2;0;170;238m'          # 命令名：蔚蓝
    w=$'\033[38;2;255;255;255m'        # 解释：白
    r=$'\033[0m'
  fi

  printf '常见命令\n\n'
  printf '  %sinstall%s              %s首次部署/启动%s\n' "$c" "$r" "$w" "$r"
  printf '  %supdate%s               %s拉取最新版本%s\n' "$c" "$r" "$w" "$r"
  printf '                       %s--tag sha-xxxxxxx%s  %s回退到指定版本%s\n' "$c" "$r" "$w" "$r"
  printf '  %srollback%s             %s回滚到上一个镜像%s\n' "$c" "$r" "$w" "$r"
  printf '  %spasswd%s %s<new_password>%s   %s重置站长密码%s\n' "$c" "$r" "$c" "$r" "$w" "$r"
  printf '  %sbackup%s               %s备份数据库 + 上传文件%s\n' "$c" "$r" "$w" "$r"
  printf '  %sstatus%s / %slogs%s / %srestart%s / %sdown%s\n' "$c" "$r" "$c" "$r" "$c" "$r" "$c" "$r"
  printf '  %sversion%s              %s显示脚本版本%s\n' "$c" "$r" "$w" "$r"
  printf '  %shelp%s                 %s帮助（不带字符画）%s\n' "$c" "$r" "$w" "$r"
  printf '\n'
  # 隐藏命令（照常可用，只是不列出来）：
  #   upgrade  只更新脚本本身（自更新/版本提示会引导用户用 update）
  #   demo     本地预览终端输出
  #   debug    跑一遍完整流程（docker 只打印不执行）
}

usage() {
  show_banner
  check_new_version      # 紧跟标识下方：有新版就在这里提示
  print_commands
}

# help：只打命令列表，不打字符画 —— 终端窄、日志里、复制粘贴时更好用
cmd_help() {
  print_commands
}

# 只检查并更新脚本本身（不动容器）
cmd_upgrade() {
  info "当前版本：spblog ${SPBLOG_VERSION}"
  self_update upgrade
  ok "已是最新版本：spblog ${SPBLOG_VERSION}"
}

# ---------- demo：本地预览终端输出（不需要 Docker / 服务器） ----------
# 改脚本里的字符画、进度条样式、提示文案时，用它快速看效果：
#   bash spblog.sh demo          正常速度
#   bash spblog.sh demo fast     加速（约 2 秒跑完）
cmd_demo() {
  local fast="${1:-}"
  local step=0.06
  [ "$fast" = "fast" ] && step=0.01

  show_banner compact
  info "spblog v${SPBLOG_VERSION}（demo，不执行任何 docker 命令）"

  info "拉取镜像 crpi-jlqf5sfrp7vsp54s.cn-hangzhou.personal.cr.aliyuncs.com/rineyce/spblog:latest"
  BAR_START="$(date +%s)"
  BAR_PCT=0
  bar_render
  local i
  for i in 3 8 14 21 27 34; do
    BAR_PCT=$i; bar_render; sleep "$step"
  done
  bar_msg "已下载 3/7 层"
  for i in 41 48 57 66 74 83 90; do
    BAR_PCT=$i; bar_render; sleep "$step"
  done
  bar_done
  ok "容器已启动"
  ok "服务已就绪（耗时 0:08）"
  ok "部署完成"

  printf '\n'
  info "spblog demo update（更新流程）"
  info "拉取镜像 crpi-…/spblog:latest"
  BAR_START="$(date +%s)"
  for i in 10 25 40 58 72 85 95; do
    BAR_PCT=$i; bar_render; sleep "$step"
  done
  bar_done
  ok "应用容器已替换"
  ok "新版本已就绪（耗时 0:06）"
  ok "更新完成：crpi-…/spblog:latest"
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
  help|-h|--help) cmd_help ;;
  upgrade)  cmd_upgrade ;;
  demo)     cmd_demo "${ARGS[0]:-}" ;;
  debug)    cmd_debug "${ARGS[0]:-}" ;;
  logs)     cmd_logs ;;
  restart)  cmd_restart ;;
  down)     cmd_down ;;
  *)        usage ;;
esac
