<#
.SYNOPSIS
    本地测试环境一键启停（Windows）

.DESCRIPTION
    拉起三样东西：
      1. PostgreSQL   .tools\pg 里的便携版，端口 5432
      2. 后端         .tools\blog-server-dev.exe，端口 8080（顺带托管 frontend\dist）
      3. 前端 dev 服务 Vite，端口 5173（热更新，/api 代理到 8080）

.EXAMPLE
    .\dev.ps1 start     启动「正式构建」模式：只开数据库 + 后端，访问 http://127.0.0.1:8080
    .\dev.ps1 dev       启动「开发」模式：额外开 Vite，访问 http://127.0.0.1:5173（改代码自动刷新）
    .\dev.ps1 stop      停止全部
    .\dev.ps1 status    看端口与进程
    .\dev.ps1 logs      跟踪后端日志
    .\dev.ps1 build     重新构建前端（改完代码想让 8080 生效时用）
#>
param(
    [Parameter(Position = 0)]
    [ValidateSet('start', 'dev', 'stop', 'restart', 'status', 'logs', 'build', 'help')]
    [string]$Command = 'start'
)

$ErrorActionPreference = 'Stop'
$Root = $PSScriptRoot
$Tools = Join-Path $Root '.tools'
$LogDir = Join-Path $Tools 'logs'
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

$PgExe = Join-Path $Tools 'pg\pgsql\bin\postgres.exe'
$PgData = Join-Path $Tools 'pg\data'
$ServerExe = Join-Path $Tools 'blog-server-dev.exe'
$ViteJs = Join-Path $Root 'frontend\node_modules\vite\bin\vite.js'
$BaseUrl = 'http://127.0.0.1:8080'
$DevUrl = 'http://127.0.0.1:5173'

function Say($msg, $color = 'Cyan') { Write-Host $msg -ForegroundColor $color }
function Ok($msg) { Write-Host "  OK  $msg" -ForegroundColor Green }
function Warn($msg) { Write-Host "  !!  $msg" -ForegroundColor Yellow }
function Fail($msg) { Write-Host "  XX  $msg" -ForegroundColor Red }

# 端口是否在监听
function Test-Port([int]$Port) {
    try {
        $c = New-Object System.Net.Sockets.TcpClient
        $task = $c.ConnectAsync('127.0.0.1', $Port)
        $done = $task.Wait(400)
        $alive = $done -and $c.Connected
        $c.Close()
        return $alive
    } catch {
        return $false
    }
}

# 等端口起来（最多 $Timeout 秒）
function Wait-Port([int]$Port, [int]$Timeout = 30) {
    for ($i = 0; $i -lt $Timeout; $i++) {
        if (Test-Port $Port) { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

# 找到监听某端口的进程并结束
function Stop-Port([int]$Port, [string]$Name) {
    $conns = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    if (-not $conns) { Write-Host "  --  $Name : $Port 未在运行"; return }
    $pids = $conns | Select-Object -ExpandProperty OwningProcess -Unique
    foreach ($procId in $pids) {
        if ($procId -and $procId -gt 0) {
            Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
        }
    }
    Ok "$Name 已停止 (端口 $Port)"
}

function Start-DataBase {
    if (Test-Port 5432) { Ok 'PostgreSQL 已在运行'; return }
    Say '  启动 PostgreSQL ...'
    if (-not (Test-Path $PgExe)) { Fail "找不到 $PgExe"; return }
    Start-Process -FilePath $PgExe `
        -ArgumentList @('-D', $PgData, '-p', '5432') `
        -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogDir 'postgres.out.log') `
        -RedirectStandardError (Join-Path $LogDir 'postgres.err.log')

    # 端口通了不等于能用：上次没正常退出时 PostgreSQL 要先做崩溃恢复，
    # 这期间连接会返回 "the database system is starting up"，所以用 pg_isready 判定
    $isReady = Join-Path $Tools 'pg\pgsql\bin\pg_isready.exe'
    for ($i = 0; $i -lt 120; $i++) {
        if (Test-Path $isReady) {
            & $isReady -h 127.0.0.1 -p 5432 -q 2>$null | Out-Null
            if ($LASTEXITCODE -eq 0) { Ok 'PostgreSQL 就绪 (5432)'; return }
        } elseif (Test-Port 5432) {
            Ok 'PostgreSQL 就绪 (5432)'
            return
        }
        Start-Sleep -Milliseconds 500
    }
    Fail 'PostgreSQL 60 秒内未就绪，看 .tools\logs\postgres.err.log'
}

function Start-Backend {
    if (Test-Port 8080) { Ok '后端已在运行'; return }
    Say '  启动后端 ...'
    if (-not (Test-Path $ServerExe)) {
        Fail "找不到 $ServerExe（先 go build -o .tools\blog-server-dev.exe .）"
        return
    }
    # 环境变量：子进程会继承
    $env:PORT = '8080'
    $env:STATIC_DIR = Join-Path $Root 'frontend\dist'
    $env:UPLOAD_DIR = Join-Path $Root 'uploads'
    $env:POST_CONTENT_DIR = Join-Path $Root 'data\posts'
    $env:TZ = 'Asia/Shanghai'
    # 本地没装 Redis 就别设 REDIS_ADDR：不设只会「在线状态显示离线」，聊天照常
    Remove-Item Env:REDIS_ADDR -ErrorAction SilentlyContinue

    Start-Process -FilePath $ServerExe `
        -WorkingDirectory (Join-Path $Root 'backend') `
        -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogDir 'backend.out.log') `
        -RedirectStandardError (Join-Path $LogDir 'backend.err.log')

    # 数据库刚恢复完时后端可能抢跑失败，等不到就再拉一次
    if (Wait-Port 8080 25) { Ok '后端就绪 (8080)'; return }
    Say '  首次未起来，重试一次 ...'
    Start-Sleep -Seconds 2
    Start-Process -FilePath $ServerExe `
        -WorkingDirectory (Join-Path $Root 'backend') `
        -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogDir 'backend.out.log') `
        -RedirectStandardError (Join-Path $LogDir 'backend.err.log')
    if (Wait-Port 8080 25) { Ok '后端就绪 (8080)' } else { Fail '后端启动失败，看 .tools\logs\backend.err.log' }
}

function Start-Vite {
    if (Test-Port 5173) { Ok 'Vite 已在运行'; return }
    Say '  启动 Vite dev server ...'
    if (-not (Test-Path $ViteJs)) { Fail "找不到 $ViteJs（先在 frontend 里 npm install）"; return }
    $node = (Get-Command node -ErrorAction SilentlyContinue).Source
    if (-not $node) { Fail '找不到 node，检查 PATH'; return }
    Start-Process -FilePath $node `
        -ArgumentList @($ViteJs, '--host', '127.0.0.1', '--port', '5173') `
        -WorkingDirectory (Join-Path $Root 'frontend') `
        -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogDir 'vite.out.log') `
        -RedirectStandardError (Join-Path $LogDir 'vite.err.log')
    if (Wait-Port 5173 40) { Ok 'Vite 就绪 (5173)' } else { Fail 'Vite 启动超时，看 .tools\logs\vite.err.log' }
}

function Show-Urls([bool]$withVite) {
    Write-Host ''
    if ($withVite) {
        Say "  前端（热更新）： $DevUrl" 'Green'
        Say "  后端直连：      $BaseUrl"
    } else {
        Say "  站点：          $BaseUrl" 'Green'
    }
    Say "  后台：          $BaseUrl/admin/login"
    Say "  日志：          .tools\logs\   （.\dev.ps1 logs 跟后端）"
    Say "  账号：          admin / Admin@103253" 'DarkGray'
    Write-Host ''
}

function Cmd-Status {
    Say '端口状态：'
    @(
        @{ Port = 5432; Name = 'PostgreSQL' },
        @{ Port = 8080; Name = '后端' },
        @{ Port = 5173; Name = 'Vite' }
    ) | ForEach-Object {
        if (Test-Port $_.Port) { Ok ("{0,-12} {1}  运行中" -f $_.Name, $_.Port) }
        else { Write-Host ("  --  {0,-12} {1}  未运行" -f $_.Name, $_.Port) -ForegroundColor DarkGray }
    }
}

function Cmd-Stop {
    Say '停止本地服务 ...'
    Stop-Port 5173 'Vite'
    Stop-Port 8080 '后端'
    Stop-Port 5432 'PostgreSQL'
    Write-Host ''
    Ok '已全部停止'
}

function Cmd-Build {
    Say '构建前端（vite build）...'
    $node = (Get-Command node -ErrorAction SilentlyContinue).Source
    Push-Location (Join-Path $Root 'frontend')
    try {
        & $node (Join-Path $Root 'frontend\node_modules\vite\bin\vite.js') build
        if ($LASTEXITCODE -eq 0) { Ok '构建完成，刷新 http://127.0.0.1:8080 即可' }
        else { Fail '构建失败' }
    } finally { Pop-Location }
}

switch ($Command) {
    'start' {
        Say '启动本地环境（数据库 + 后端）' 'Green'
        Start-DataBase
        Start-Backend
        Show-Urls $false
    }
    'dev' {
        Say '启动本地开发环境（数据库 + 后端 + Vite）' 'Green'
        Start-DataBase
        Start-Backend
        Start-Vite
        Show-Urls $true
    }
    'restart' {
        Cmd-Stop
        Start-Sleep -Seconds 2
        Start-DataBase
        Start-Backend
        Show-Urls $false
    }
    'stop' { Cmd-Stop }
    'status' { Cmd-Status }
    'build' { Cmd-Build }
    'logs' {
        $f = Join-Path $LogDir 'backend.out.log'
        if (Test-Path $f) { Get-Content $f -Tail 40 -Wait } else { Warn "还没有日志：$f" }
    }
    default {
        Get-Help $PSCommandPath -Detailed
    }
}
