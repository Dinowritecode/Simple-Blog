<template>
  <section>
    <h2><span class="no">11</span>管理接口详解</h2>
    <div class="box tip">以下所有接口的请求头都必须带 <code>Authorization: Bearer &lt;token&gt;</code>，否则返回 401。</div>

    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag auth">需登录</span>/api/admin/posts?page=1&amp;pageSize=10</div>
      <div class="desc">全部文章（<strong>含草稿</strong>），按更新时间倒序。参数与响应格式同公开列表。</div>
    </div>

    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag auth">需登录</span>/api/admin/posts/:id</div>
      <div class="desc">文章详情（含草稿），编辑页加载数据时用。</div>
    </div>

    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag auth">需登录</span>/api/admin/posts</div>
      <div class="desc">新建文章</div>
      <h4>请求体</h4>
      <pre><code>{
  "title": "文章标题（必填）",
  "summary": "摘要，可空",
  "cover": "封面图URL，可空",
  "content": "&lt;p&gt;正文 HTML&lt;/p&gt;，可空",
  "status": "draft"    <span class="comment">// draft 或 published</span>
}</code></pre>
      <h4>响应 200：创建成功的完整文章（含新 ID、时间戳）</h4>
      <h4>错误</h4>
      <ul><li>400：JSON 格式错误 / 标题为空 / status 非法</li></ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag put">PUT</span><span class="tag auth">需登录</span>/api/admin/posts/:id</div>
      <div class="desc">更新文章（<strong>部分字段更新</strong>）：请求体里哪个字段非空就更新哪个，空字符串表示「保持原值」。因此「一键发布」只需传 <code>{"status": "published"}</code>。无论改了什么，updated_at 都会刷新。</div>
      <h4>响应 200</h4>
      <pre><code>{ "ok": true, "id": 2 }</code></pre>
      <h4>错误</h4>
      <ul><li>400：JSON 格式错误 / status 非法 / 所有字段都为空；404：文章不存在</li></ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag del">DELETE</span><span class="tag auth">需登录</span>/api/admin/posts/:id</div>
      <div class="desc">删除文章（物理删除，不可恢复）</div>
      <h4>响应 200</h4>
      <pre><code>{ "ok": true }</code></pre>
      <h4>错误</h4>
      <ul><li>404：文章不存在</li></ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag auth">需登录</span>/api/admin/upload</div>
      <div class="desc">上传图片：<code>multipart/form-data</code>，文件字段名必须是 <code>file</code>；仅支持 jpg/jpeg/png/gif/webp，≤10MB</div>
      <h4>响应 200</h4>
      <pre><code>{ "url": "/uploads/3f8a1c2bd4e5f607.jpg" }</code></pre>
      <p>拿到的 url 可直接用于封面字段或插进正文（<code>&lt;img src="/uploads/xxx.jpg"&gt;</code>）。</p>
    </div>

    <h3>用 curl 快速试一遍（Windows PowerShell）</h3>
    <pre><code><span class="comment"># 1. 登录拿 token</span>
$r = Invoke-RestMethod -Uri http://localhost:8080/api/auth/login -Method Post `
  -ContentType 'application/json' -Body '{"username":"admin","password":"admin123"}'
$token = $r.token

<span class="comment"># 2. 列文章（公开）</span>
curl.exe http://localhost:8080/api/posts

<span class="comment"># 3. 新建文章（带 token）</span>
curl.exe -X POST -H "Authorization: Bearer $token" -H "Content-Type: application/json" `
  -d '{"title":"测试","content":"&lt;p&gt;你好&lt;/p&gt;","status":"published"}' `
  http://localhost:8080/api/admin/posts</code></pre>
  </section>
</template>
