# ============================================================
# 多阶段构建：前端 → 后端 → 运行时
#
#   docker build -t rin-eyce/blog .
#   docker compose up -d          # 见 docker-compose.yml
#
# 最终镜像只含一个静态二进制 + 前端产物，镜像很小，也不需要 Node / Go。
# ============================================================

# ---------- ① 构建前端 ----------
FROM node:20-alpine AS web
WORKDIR /src/frontend
# 先只拷依赖清单，利用层缓存：改代码不会重新装依赖
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---------- ② 构建后端 ----------
FROM golang:1.25-alpine AS api
WORKDIR /src/backend
# 纯 Go 依赖（pgx），关掉 CGO 才能得到静态二进制
ENV CGO_ENABLED=0 GOOS=linux
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN go build -trimpath -ldflags="-s -w" -o /out/blog-server .

# ---------- ③ 运行时 ----------
FROM alpine:3.20

# ca-certificates 访问外链图片要用；tzdata 让 TZ=Asia/Shanghai 生效
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 blog

WORKDIR /app
COPY --from=api /out/blog-server /app/blog-server
COPY --from=web /src/frontend/dist /app/dist

# 这两个目录会被 docker 卷覆盖；这里先建好并给对属主，
# 容器首次创建卷时 Docker 会把这份属主信息复制过去，非 root 用户才能写入
RUN mkdir -p /app/uploads /app/data/posts \
    && chown -R blog:blog /app

ENV PORT=8080 \
    STATIC_DIR=/app/dist \
    UPLOAD_DIR=/app/uploads \
    POST_CONTENT_DIR=/app/data/posts \
    TZ=Asia/Shanghai

USER blog
EXPOSE 8080

# 健康检查打的是公开接口，不需要登录
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/stats >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/blog-server"]
