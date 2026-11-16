# ============================================================
# deploy.ps1 —— 一条命令把站点发到 Linux 服务器
#
#   用法（在 D:\website 下）：
#     .\deploy.ps1 -Server root@1.2.3.4
#     .\deploy.ps1 -Server root@1.2.3.4 -RemoteDir /srv/blog -Service blog
#
#   它会做三件事：
#     1) 构建前端（frontend/dist）
#     2) 交叉编译后端（linux/amd64 单文件）
#     3) 传到服务器 → 原子替换 → 重启 systemd 服务
#
#   前提：服务器上已经建好数据库、装好二进制所在目录和 systemd 服务
#        （第一次部署见 DEPLOY.md 的 4~6 节）
# ============================================================
param(
  [Parameter(Mandatory = $true)]
  [string]$Server,                       # 例：root@1.2.3.4

  [string]$RemoteDir = "/opt/blog",      # 服务器上的站点根目录
  [string]$Service = "blog",             # systemd 服务名

  [switch]$SkipBuild,                    # 只上传，不重新构建
  [switch]$NoRestart                     # 上传但不重启
)

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$stage = Join-Path $root ".deploy"

function Step($n, $text) { Write-Host "[$n] $text" -ForegroundColor Cyan }

# ---------- 1) 构建 ----------
if (-not $SkipBuild) {
  Step "1/3" "构建前端（vite build）"
  Push-Location (Join-Path $root "frontend")
  try {
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
  } finally { Pop-Location }

  Step "2/3" "交叉编译后端（linux/amd64）"
  New-Item -ItemType Directory -Force -Path $stage | Out-Null
  # pgx 是纯 Go，关掉 CGO 才能跨平台编译
  $env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"
  Push-Location (Join-Path $root "backend")
  try {
    go build -o (Join-Path $stage "blog-server") .
    if ($LASTEXITCODE -ne 0) { throw "后端构建失败" }
  } finally {
    Pop-Location
    Remove-Item Env:CGO_ENABLED, Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
  }
}

if (-not (Test-Path (Join-Path $stage "blog-server"))) {
  throw "没有找到构建产物，先去掉 -SkipBuild 跑一次"
}

# ---------- 2) 上传 ----------
Step "3/3" "上传到 $Server"
ssh $Server "mkdir -p /tmp/blog-upload" | Out-Null
scp (Join-Path $stage "blog-server") "${Server}:/tmp/blog-upload/blog-server"
scp -r (Join-Path $root "frontend\dist") "${Server}:/tmp/blog-upload/dist"

# ---------- 3) 原子替换 + 重启 ----------
# 先把新文件放到 .new，再 mv 覆盖：进程崩在中间也不会留下半个文件
$remote = @"
set -e
mkdir -p '$RemoteDir/uploads' '$RemoteDir/data/posts'
install -m 755 /tmp/blog-upload/blog-server '$RemoteDir/blog-server.new'
rm -rf '$RemoteDir/dist.new'
cp -r /tmp/blog-upload/dist '$RemoteDir/dist.new'
mv -f '$RemoteDir/blog-server.new' '$RemoteDir/blog-server'
if [ -d '$RemoteDir/dist' ]; then rm -rf '$RemoteDir/dist.old'; mv '$RemoteDir/dist' '$RemoteDir/dist.old'; fi
mv '$RemoteDir/dist.new' '$RemoteDir/dist'
rm -rf /tmp/blog-upload
"@

if ($NoRestart) {
  $remote += "`necho '已上传，未重启（-NoRestart）'"
} else {
  $remote += "`nsystemctl restart '$Service' && sleep 1 && systemctl is-active '$Service'"
}

$remote | ssh $Server "bash -s"

Write-Host "`n完成：https://你的域名  （服务：$Service，目录：$RemoteDir）" -ForegroundColor Green
