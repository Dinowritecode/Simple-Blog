package main

// ============================================================
// siteinfo.go —— 后台「总览」右侧的网站简略信息 + 备忘录
//
//   GET /api/admin/site-info  {name, version, os, ip, startedAt, uptime, memo}
//   PUT /api/admin/memo       {memo}   保存备忘录（存 settings，重启不丢）
//
// 各字段来源：
//   名称    settings.site_name → hero_nickname → 默认「凛冬RinEyece」
//   版本    环境变量 APP_VERSION（compose 里由镜像 tag 传入），没有就是 dev
//   系统    Linux 读 /etc/os-release 的 PRETTY_NAME，其它用 GOOS/GOARCH
//   IP      第一张网卡上的非回环 IPv4
//   启动    进程启动时刻（包级变量），运行时长由它算
// ============================================================

import (
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 进程启动时间：main 包初始化时就记下来
var processStartedAt = time.Now()

const settingAdminMemo = "admin_memo"

// siteName 站点名称：优先后台设置，其次首屏昵称，最后兜底
func siteName() string {
	if v := strings.TrimSpace(getSetting("site_name")); v != "" {
		return v
	}
	if v := strings.TrimSpace(getSetting("hero_nickname")); v != "" {
		return strings.ReplaceAll(v, "|", "") // 首屏昵称用 | 分隔两段颜色，这里拼回一个整体
	}
	return "凛冬RinEyece"
}

// appVersion 当前镜像版本：compose 把镜像 tag 作为 APP_VERSION 传进来
func appVersion() string {
	if v := strings.TrimSpace(os.Getenv("APP_VERSION")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("IMAGE_TAG")); v != "" {
		return v
	}
	return "dev"
}

// osInfo 形如「Ubuntu 22.04.3 LTS · linux/amd64」
func osInfo() string {
	name := runtime.GOOS
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				name = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
				break
			}
		}
	}
	if name == "windows" {
		name = "Windows"
	}
	return name + " · " + runtime.GOOS + "/" + runtime.GOARCH
}

// localIP 取第一张网卡上的非回环 IPv4（拿不到就 127.0.0.1）
func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	// 先找内网地址（192.168 / 10. / 172.16-31），再退而求其次找任意公网地址，
	// 跳过 169.254.x.x 这种「网线没插好」的链路本地地址 —— 直接取第一个会拿到它。
	var fallback string
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		v4 := ipnet.IP.To4()
		if v4 == nil || v4.IsLinkLocalUnicast() {
			continue
		}
		if v4[0] == 10 || (v4[0] == 192 && v4[1] == 168) || (v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31) {
			return v4.String()
		}
		if fallback == "" {
			fallback = v4.String()
		}
	}
	if fallback != "" {
		return fallback
	}
	return "127.0.0.1"
}

// humanUptime 把秒数说成人话：3 天 4 小时 12 分
func humanUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, itoa(days)+" 天")
	}
	if hours > 0 || days > 0 {
		parts = append(parts, itoa(hours)+" 小时")
	}
	parts = append(parts, itoa(mins)+" 分")
	return strings.Join(parts, " ")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// handleAdminSiteInfo 网站简略信息 + 备忘录。GET /api/admin/site-info
func handleAdminSiteInfo(c *gin.Context) {
	up := time.Since(processStartedAt)
	c.JSON(http.StatusOK, gin.H{
		"name":         siteName(),
		"version":      appVersion(),
		"os":           osInfo(),
		"ip":           localIP(),
		"startedAt":    processStartedAt.Format("2006-01-02 15:04:05"),
		"uptime":       humanUptime(up),
		"uptimeSec":    int64(up.Seconds()),
		"memo":         getSetting(settingAdminMemo),
		"storage":      storageHint(),
		"currentTime":  time.Now().Format("2006-01-02 15:04:05"),
		"processCount": runtime.NumGoroutine(),
	})
}

// storageHint 数据存放位置（容器里一眼知道数据在哪个卷）
func storageHint() string {
	if v := strings.TrimSpace(os.Getenv("UPLOAD_DIR")); v != "" {
		return v
	}
	return "uploads"
}

// handleAdminSaveMemo 保存备忘录。PUT /api/admin/memo
func handleAdminSaveMemo(c *gin.Context) {
	var req struct {
		Memo string `json:"memo"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	memo := strings.TrimSpace(req.Memo)
	if len([]rune(memo)) > 5000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "备忘录最多 5000 字"})
		return
	}
	if err := setSetting(settingAdminMemo, memo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "memo": memo})
}
