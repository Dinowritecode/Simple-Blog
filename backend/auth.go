package main

// ============================================================
// auth.go —— 认证与授权模块
// 职责：
//   - 账户初始化（首次启动自动创建管理员，email/username/role 全字段）
//   - 登录（邮箱或用户名 + 密码，bcrypt 校验）与注册（邮箱 + 密码）
//   - JWT 签发与两个鉴权中间件：
//       authMiddleware   —— 任何已登录用户（普通用户 / 管理员）
//       adminMiddleware  —— 仅管理员
//
// 认证原理（通俗版）：
//   1. 登录 / 注册成功后，后端签发 JWT（HS256），sub = 用户 ID，有效期 7 天。
//   2. 前端每次请求管理/个人接口都带 Authorization: Bearer <token>。
//   3. 中间件验签后，再从数据库加载用户（id/role…）挂到请求上下文，
//      这样用户被删除 / 角色变化能即时生效，无需等 token 过期。
// ============================================================

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// jwtSecret JWT 签名密钥（进程内缓存，首次使用时从环境变量或数据库读取/生成）。
var jwtSecret []byte

// ensureAdmin 初始化管理员账号。
// 逻辑：如果不存在任何 role='admin' 的账号，就按 ADMIN_USER/ADMIN_PASSWORD 创建一个。
// 已存在则不动 —— 保证不会覆盖你后来修改过的密码。
func ensureAdmin(username, password string) error {
	var count int
	if err := q.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	// 老库可能残留同用户名但缺角色的行：先尝试升级它，再考虑新建
	res, err := q.Exec(
		`UPDATE users SET role = 'admin', email = COALESCE(email, ?), password_hash = ? WHERE username = ?`,
		strings.ToLower(username)+"@localhost", string(hash), username,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	// 桃信 ID：管理员同样需要 9 位 ID（否则别人搜不到、也无法私信）
	uid, err := genUserID()
	if err != nil {
		return err
	}
	_, err = q.Exec(
		`INSERT INTO users (username, email, password_hash, nickname, level, user_id, role)
		 VALUES (?, ?, ?, ?, 1, ?, 'admin')`,
		username, strings.ToLower(username)+"@localhost", string(hash), username, uid,
	)
	return err
}

// randomPassword 生成随机初始密码（首次部署创建站长账号用，仅打印一次）。
// 字符集去掉了易混淆的 0O1lI，长度 n 默认 >= 8。
func randomPassword(n int) string {
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	if n < 8 {
		n = 8
	}
	buf := make([]byte, n)
	// 保证每位都来自 charset 且避免简单连续（无特殊要求，够用即可）
	randBytes := make([]byte, n)
	if _, err := rand.Read(randBytes); err != nil {
		return "changeme123456" // 极端兜底
	}
	for i := 0; i < n; i++ {
		buf[i] = chars[int(randBytes[i])%len(chars)]
	}
	return string(buf)
}

// getJWTSecret 获取 JWT 签名密钥，优先级：
//  1. 环境变量 JWT_SECRET（适合多实例部署）
//  2. settings 表中持久化的随机密钥（首次启动自动生成 64 位十六进制随机串）
func getJWTSecret() ([]byte, error) {
	if jwtSecret != nil {
		return jwtSecret, nil
	}
	if s := os.Getenv("JWT_SECRET"); s != "" {
		jwtSecret = []byte(s)
		return jwtSecret, nil
	}
	var stored string
	err := q.QueryRow(`SELECT value FROM settings WHERE "key" = 'jwt_secret'`).Scan(&stored)
	if err == nil && stored != "" {
		jwtSecret = []byte(stored)
		return jwtSecret, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	secret := hex.EncodeToString(buf)
	// PostgreSQL 的 upsert 写法：ON CONFLICT (主键) DO UPDATE
	if _, err := q.Exec(
		`INSERT INTO settings ("key", value) VALUES ('jwt_secret', ?)
		 ON CONFLICT ("key") DO UPDATE SET value = EXCLUDED.value`,
		secret,
	); err != nil {
		return nil, err
	}
	jwtSecret = []byte(secret)
	return jwtSecret, nil
}

// signToken 用用户 ID 签发 JWT（7 天有效期）。
func signToken(userID int64) (string, error) {
	secret, err := getJWTSecret()
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10), // 主题：用户 ID（字符串，避免跨语言精度问题）
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// userByAccount 按「邮箱或用户名」查用户（密码比对放外面做）。
// username 列对普通用户为 NULL，故用 NullString 扫描后归一为字符串。
func userByAccount(account string) (*User, string, error) {
	var u User
	var hash string
	var username, email sql.NullString
	err := q.QueryRow(
		`SELECT id, COALESCE(user_id,''), email, username, nickname, avatar, COALESCE(level,1), COALESCE(points,0), role, created_at, password_hash
		 FROM users WHERE email = ? OR username = ? LIMIT 1`,
		strings.ToLower(strings.TrimSpace(account)), strings.TrimSpace(account),
	).Scan(&u.ID, &u.UserID, &email, &username, &u.Nickname, &u.Avatar, &u.Level, &u.Points, &u.Role, &u.CreatedAt, &hash)
	if err != nil {
		return nil, "", err
	}
	u.Email, u.Username = email.String, username.String
	return &u, hash, nil
}

// userByID 按 ID 查用户（中间件刷新用户状态用）。
func userByID(id int64) (*User, error) {
	var u User
	var username, email sql.NullString
	err := q.QueryRow(
		`SELECT id, COALESCE(user_id,''), email, username, nickname, avatar, COALESCE(cover,''), COALESCE(level,1), COALESCE(points,0), role, created_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.UserID, &email, &username, &u.Nickname, &u.Avatar, &u.Cover, &u.Level, &u.Points, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.Email, u.Username = email.String, username.String
	return &u, nil
}

// userBriefByID 按 ID 取「简版用户信息」（桃信/评论/通知里嵌套用，含 9 位 ID 与等级）。
func userBriefByID(id int64) (*UserBrief, error) {
	var b UserBrief
	var nickname, avatar, role, uid sql.NullString
	var level sql.NullInt64
	err := q.QueryRow(
		`SELECT id, user_id, nickname, avatar, level, role FROM users WHERE id = ?`, id,
	).Scan(&b.ID, &uid, &nickname, &avatar, &level, &role)
	if err != nil {
		return nil, err
	}
	b.UserID, b.Nickname, b.Avatar, b.Role = uid.String, nickname.String, avatar.String, role.String
	b.Level = int(level.Int64)
	if b.Level < 1 {
		b.Level = 1
	}
	return &b, nil
}

// authUser 从请求上下文取当前登录用户（中间件挂载的）。
func authUser(c *gin.Context) *User {
	if v, ok := c.Get("user"); ok {
		if u, ok := v.(*User); ok {
			return u
		}
	}
	return nil
}

// ------------------------------------------------------------
// 接口：登录 / 注册（公开）
// ------------------------------------------------------------

// emailRe 极简邮箱格式校验（用于注册）
var emailRe = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)

// handleLogin 登录接口：POST /api/auth/login
// 请求体: {"account": "邮箱或用户名", "password": "..."}
// 响应:   {"token": "<JWT>", "user": {…}}
func handleLogin(c *gin.Context) {
	var req struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Account) == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入账号和密码"})
		return
	}
	u, hash, err := userByAccount(req.Account)
	if errors.Is(err, sql.ErrNoRows) || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		// 用户不存在与密码错误返回同一提示，防止枚举账号
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号或密码错误"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务端错误"})
		return
	}
	token, err := signToken(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签发令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": u})
}

// handleRegister 注册接口：POST /api/auth/register
// 请求体: {"email": "...", "password": "..."}
// 规则：邮箱合法且未被注册、密码至少 6 位。成功后自动登录（直接返回 token）。
// 目前不做邮箱验证（个人博客，注册即用）。
func handleRegister(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !emailRe.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式不正确"})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少需要 6 位"})
		return
	}
	// 昵称默认取邮箱前缀，作为展示名兜底
	nick := strings.SplitN(email, "@", 2)[0]
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务端错误"})
		return
	}
	// 桃信 ID：注册时分配一个没被占用的 9 位数字
	uid, err := genUserID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务端错误"})
		return
	}
	// PostgreSQL 用 RETURNING 拿自增主键（驱动不支持 LastInsertId）
	id, err := insertReturningID(
		`INSERT INTO users (username, email, password_hash, nickname, avatar, level, user_id, role)
		 VALUES (NULL, ?, ?, ?, '', 1, ?, 'user')
		 RETURNING id`,
		email, string(hash), nick, uid,
	)
	if err != nil {
		// 23505 = unique_violation（邮箱已被注册）
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被注册"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}
	u, err := userByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}
	token, err := signToken(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签发令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": u})
}

// ------------------------------------------------------------
// 鉴权中间件
// ------------------------------------------------------------

// parseAndLoadUser 公共步骤：验签 → 取 sub（用户 ID）→ 从数据库加载用户。
// 失败时直接写 401 响应并返回 nil。
func parseAndLoadUser(c *gin.Context) *User {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return nil
	}
	u, err := userFromToken(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return nil
	}
	c.Set("user", u)
	c.Set("userId", u.ID)
	return u
}

// userFromToken 校验 JWT 并加载对应用户（不涉及 HTTP 响应，方便 SSE 等场景复用）。
// 返回的错误文案已经是「可直接展示给用户」的中文。
func userFromToken(tokenStr string) (*User, error) {
	secret, err := getJWTSecret()
	if err != nil {
		return nil, errors.New("服务端错误")
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		// 只接受 HMAC 签名（HS256），防止算法混淆攻击
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非法的签名方式")
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("登录已过期，请重新登录")
	}
	sub, _ := token.Claims.(jwt.MapClaims)["sub"].(string)
	id, err := strconv.ParseInt(sub, 10, 64)
	if err != nil || id < 1 {
		return nil, errors.New("登录已过期，请重新登录")
	}
	u, err := userByID(id)
	if err != nil {
		// 查不到用户（可能已被删除）→ 视为未登录
		return nil, errors.New("账号不存在或已被删除")
	}
	return u, nil
}

// authMiddleware 登录校验：任何已登录用户可访问（普通用户 / 管理员）。
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if parseAndLoadUser(c) != nil {
			c.Next()
		}
	}
}

// adminMiddleware 管理员校验：在 authMiddleware 基础上要求 role='admin'。
func adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := parseAndLoadUser(c)
		if u == nil {
			return
		}
		if u.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Next()
	}
}
