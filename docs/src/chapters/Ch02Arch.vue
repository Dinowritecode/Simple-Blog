<template>
  <section>
    <h2><span class="no">02</span>系统架构与原理</h2>

    <h3>整体架构</h3>
    <div class="box">
      <pre class="arch">
                  ┌──────────────────────────────┐
                  │     浏览器（前端页面）           │
                  │   Vue 3 单页应用（SPA）         │
                  └──────────────┬───────────────┘
               ①页面/静态资源      │  ② /api/** JSON 请求
                  │               ▼
                  │      ┌────────────────┐
                  ▼      │  Go 后端（Gin） │
       frontend/dist      │   端口 :8080    │
        （页面文件）        └───┬────────┬───┘
                          │        │
                    ③读写数据 │        │ ④图片文件
                          ▼        ▼
                     ┌────────┐  ┌──────────┐
                     │ MySQL  │  │ uploads/ │
                     │ 三张表  │  │ 图片目录  │
                     └────────┘  └──────────┘
</pre>
      <p><strong>核心概念：前端和后端「合并在一个服务里」。</strong>Go 后端不止提供 API，还负责把构建好的前端页面（<code>frontend/dist</code>）直接发给浏览器。所以只需启动一个 Go 进程，网站就完整可用了（不需要单独部署前端）。</p>
    </div>

    <h3>一次完整的请求流程（以「打开一篇文章」为例）</h3>
    <ol>
      <li>浏览器访问 <code>http://localhost:8080/blog/2</code></li>
      <li>Go 后端发现磁盘上没有 <code>blog/2</code> 这个文件 → 返回 <code>index.html</code>（这叫 <strong>SPA 回退</strong>，见下）</li>
      <li>浏览器加载 index.html → 下载 JS/CSS → Vue 启动，前端路由识别出「要显示文章详情页」</li>
      <li>详情页组件向 <code>/api/posts/2</code> 发请求</li>
      <li>Go 后端查 MySQL：<code>SELECT ... FROM posts WHERE id=2 AND status='published'</code></li>
      <li>后端把文章数据以 JSON 返回 → Vue 渲染出页面</li>
    </ol>

    <h3>关键原理一：SPA 回退（为什么 /blog/2 也能打开）</h3>
    <p>前端用了 vue-router 的 <strong>history 模式</strong>，地址是「假路径」——服务器上并不存在 <code>/blog/2</code> 这个文件。所以后端做了兜底（见 <code>backend/static.go</code>）：<strong>除 <code>/api</code> 外的所有路径，如果磁盘上没有对应文件，一律返回 <code>index.html</code></strong>，让前端路由决定显示什么。这样刷新页面、直接输网址都不会 404。</p>

    <h3>关键原理二：登录与 JWT 认证</h3>
    <div class="box">
      <p>JWT（JSON Web Token）是一串「带签名的字符串」，流程如下：</p>
      <ol>
        <li>管理员在登录页输入账号密码 → <code>POST /api/auth/login</code></li>
        <li>后端查 MySQL 里的 <code>password_hash</code>，用 <strong>bcrypt</strong> 比对密码（数据库里从不存明文）</li>
        <li>密码正确 → 后端用密钥签发 JWT（内含用户名，7 天有效）返回给前端</li>
        <li>前端把 token 存进 <code>localStorage</code>，之后每个管理接口请求都带上 <code>Authorization: Bearer &lt;token&gt;</code></li>
        <li>后端中间件验签（<code>backend/auth.go</code> 的 <code>authMiddleware</code>），签名有效才放行</li>
      </ol>
      <p class="comment">JWT 的特点：服务端不用存会话，token 本身就是凭证，验签即验证身份。密钥存 MySQL 的 settings 表，重启服务后旧 token 依然有效。</p>
    </div>

    <h3>关键原理三：点赞</h3>
    <p>点击点赞 → <code>POST /api/posts/:id/like</code> → 后端执行 <code>UPDATE posts SET likes = likes + 1</code>（数据库原子自增，并发安全）→ 返回最新数量。前端用 <code>localStorage</code> 记录「本浏览器已点赞的文章 ID」，同一浏览器只能点一次。这是简单方案：不做 IP 防刷、不记录谁点的赞。</p>

    <h3>关键原理四：图片上传</h3>
    <p>编辑器点「上传图片」→ 前端把图片通过 <code>POST /api/admin/upload</code>（multipart 表单，字段名 <code>file</code>）发给后端 → 后端校验类型（仅图片）和大小（≤10MB）→ 用随机文件名存到 <code>backend/uploads/</code> → 返回 <code>{"url": "/uploads/xxx.jpg"}</code> → 前端把 URL 插进文章正文。图片通过 <code>/uploads</code> 静态路由直接访问。</p>
  </section>
</template>
