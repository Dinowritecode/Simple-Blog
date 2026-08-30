package main

// ============================================================
// auth.go —— 认证模块
// 职责：管理员账号初始化、登录（账密校验）、JWT 签发与校验中间件。
//
// 认证原理（通俗版）：
//   1. 管理员第一次启动时自动创建（users 表为空时），密码用 bcrypt 加密存储，
//      数据库里永远不保存明文密码。
//   2. 登录成功 → 后端签发一个 JWT（JSON Web Token，一串带签名的字符串）
//      返回给前端，有效期 7 天。
//   3. 之后前端每次调用「管理接口」都在请求头带上
//      Authorization: Bearer <token>，后端用同一个密钥验签，验过才放行。
// ============================================================

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// jwtSecret JWT 签名密钥（进程内缓存，首次使用时从环境变量或数据库读取/生成）。
var jwtSecret []byte

// ensureAdmin 初始化管理员账号。
// 逻辑：如果 users 表里一个用户都没有，就用启动参数（ADMIN_USER / ADMIN_PASSWORD）
// 创建一个管理员；如果已有用户，什么都不做。
// 这样保证：① 首次启动一定有账号可登录；② 不会覆盖你后来修改过的账号。
func ensureAdmin(username, password string) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // 已有用户，跳过
	}
	// bcrypt 加密密码（自带随机盐，同样的密码每次加密结果都不同，更安全）
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, string(hash))
	return err
}

// getJWTSecret 获取 JWT 签名密钥，优先级：
//   1. 环境变量 JWT_SECRET（适合多实例部署，保证各实例密钥一致）
//   2. settings 表中持久化的随机密钥（首次启动自动生成 64 位十六进制随机串）
// 把密钥存进数据库的好处：重启服务后密钥不变，已签发的 token 不会失效。
func getJWTSecret() ([]byte, error) {
	if jwtSecret != nil {
		return jwtSecret, nil // 进程内已有缓存，直接返回
	}
	// ① 环境变量优先
	if s := os.Getenv("JWT_SECRET"); s != "" {
		jwtSecret = []byte(s)
		return jwtSecret, nil
	}
	// ② 查数据库里是否已有密钥
	var stored string
	err := db.QueryRow("SELECT value FROM settings WHERE `key` = 'jwt_secret'").Scan(&stored)
	if err == nil && stored != "" {
		jwtSecret = []byte(stored)
		return jwtSecret, nil
	}
	// ③ 都没有 → 生成一个随机密钥并存库（注意：`key` 是 MySQL 保留字，必须加反引号）
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	secret := hex.EncodeToString(buf)
	if _, err := db.Exec(
		"INSERT INTO settings (`key`, value) VALUES ('jwt_secret', ?) ON DUPLICATE KEY UPDATE value = ?",
		secret, secret,
	); err != nil {
		return nil, err
	}
	jwtSecret = []byte(secret)
	return jwtSecret, nil
}

// handleLogin 登录接口：POST /api/auth/login
// 请求体: {"username": "...", "password": "..."}
// 响应:   {"token": "<JWT>", "username": "..."}
// 流程：
//   1. 解析请求体，校验非空
//   2. 按用户名查库取 password_hash（查不到 = 用户不存在）
//   3. bcrypt 比对密码（比对失败 = 密码错误，和用户不存在返回同样的提示，防爆破枚举）
//   4. 签发 JWT（HS256 签名，7 天有效期），返回给前端
func handleLogin(c *gin.Context) {
	// ① 解析请求体
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}

	// ②③ 查库 + 比对密码（两者失败返回同一提示）
	var hash string
	err := db.QueryRow(`SELECT password_hash FROM users WHERE username = ?`, req.Username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// ④ 签发 JWT
	secret, err := getJWTSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务端错误"})
		return
	}
	claims := jwt.MapClaims{
		"sub": req.Username,                                  // 主题：用户名
		"iat": time.Now().Unix(),                             // 签发时间
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),     // 过期时间：7 天后
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签发令牌失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString, "username": req.Username})
}

// authMiddleware JWT 校验中间件（挂在 /api/admin 整个分组上）。
// 逻辑：请求头必须带 "Authorization: Bearer <token>"，验签通过才放行，否则直接 401。
// 验签要点：强制要求 HS256 签名算法（防止攻击者用 none 等算法伪造 token）。
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ① 取请求头
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		// ② 拿密钥
		secret, err := getJWTSecret()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "服务端错误"})
			return
		}
		// ③ 解析并验签
		token, err := jwt.Parse(strings.TrimPrefix(auth, "Bearer "), func(t *jwt.Token) (any, error) {
			// 只接受 HMAC 签名（HS256），其它一律拒绝
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("非法的签名方式")
			}
			return secret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			return
		}
		// ④ 把用户名放进上下文，后续 handler 可通过 c.GetString("username") 读取
		if sub, ok := token.Claims.(jwt.MapClaims)["sub"].(string); ok {
			c.Set("username", sub)
		}
		c.Next() // 放行到真正的接口处理函数
	}
}
