<template>
  <section>
    <h2><span class="no">11</span>管理接口详解</h2>
    <div class="box warn">以下所有接口的请求头都必须带 <code>Authorization: Bearer &lt;token&gt;</code>，且账号必须是<strong>管理员</strong>（role=admin），否则返回 401（未登录）/ 403（无权限）。</div>

    <h3>文章接口（/api/admin/posts）</h3>
    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag auth">需管理员</span>/api/admin/posts?page=1&amp;pageSize=10</div>
      <div class="desc">全部文章（<strong>含草稿</strong>），按更新时间倒序。参数与响应格式同公开列表。</div>
    </div>

    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag auth">需管理员</span>/api/admin/posts/:id</div>
      <div class="desc">文章详情（含草稿），编辑页加载数据时用。</div>
    </div>

    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag auth">需管理员</span>/api/admin/posts</div>
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
      <div class="path"><span class="tag put">PUT</span><span class="tag auth">需管理员</span>/api/admin/posts/:id</div>
      <div class="desc">更新文章（<strong>部分字段更新</strong>）：请求体里哪个字段非空就更新哪个，空字符串表示「保持原值」。因此「一键发布」只需传 <code>{"status": "published"}</code>。无论改了什么，updated_at 都会刷新。</div>
      <h4>响应 200</h4>
      <pre><code>{ "ok": true, "id": 2 }</code></pre>
      <h4>错误</h4>
      <ul><li>400：JSON 格式错误 / status 非法 / 所有字段都为空；404：文章不存在</li></ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag del">DELETE</span><span class="tag auth">需管理员</span>/api/admin/posts/:id</div>
      <div class="desc">删除文章（物理删除）。<strong>级联清理</strong>：该文章的评论、点赞记录与相关通知一并删除。</div>
      <h4>错误</h4>
      <ul><li>404：文章不存在</li></ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag auth">需管理员</span>/api/admin/upload</div>
      <div class="desc">上传图片（正文 / 封面）：<code>multipart/form-data</code>，文件字段名必须是 <code>file</code>；仅支持 jpg/jpeg/png/gif/webp，≤10MB</div>
      <h4>响应 200</h4>
      <pre><code>{ "url": "/uploads/3f8a1c2bd4e5f607.jpg" }</code></pre>
      <p>拿到的 url 可直接用于封面字段或插进正文（<code>&lt;img src="/uploads/xxx.jpg"&gt;</code>）。</p>
    </div>

    <h3>评论管理接口（/api/admin/comments）</h3>
    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag auth">需管理员</span>/api/admin/comments?page=1&amp;pageSize=10</div>
      <div class="desc">全部评论（含回复），按时间倒序。可选 <code>?postId=</code> 只看某篇文章。每项含评论内容、作者（昵称/头像/角色）、所属文章标题、<code>parentId</code>（0=顶级评论）等。</div>
    </div>
    <div class="api">
      <div class="path"><span class="tag del">DELETE</span><span class="tag auth">需管理员</span>/api/admin/comments/:id</div>
      <div class="desc">删除评论。<strong>顶级评论会连同其下所有回复一起删除</strong>，并清理相关通知。评论发布后直接公开展示（无需审核），发现违规在这里删即可。</div>
    </div>

    <h3>用户管理接口（/api/admin/users）</h3>
    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag auth">需管理员</span>/api/admin/users?page=1&amp;pageSize=10</div>
      <div class="desc">全部注册用户（含管理员），带评论数。可选 <code>?keyword=</code> 按邮箱 / 昵称模糊搜索。</div>
    </div>
    <div class="api">
      <div class="path"><span class="tag del">DELETE</span><span class="tag auth">需管理员</span>/api/admin/users/:id</div>
      <div class="desc">删除用户。<strong>级联清理</strong>该用户的评论与通知。不能删除管理员账号，也不能删除自己。</div>
    </div>

    <h3>用 curl 快速试一遍（Windows PowerShell）</h3>
    <pre><code><span class="comment"># 1. 管理员登录拿 token（账号填 admin 或邮箱均可）</span>
$r = Invoke-RestMethod -Uri http://localhost:8080/api/auth/login -Method Post `
  -ContentType 'application/json' -Body '{"account":"admin","password":"admin123"}'
$token = $r.token

<span class="comment"># 2. 列评论 / 用户</span>
curl.exe -H "Authorization: Bearer $token" http://localhost:8080/api/admin/comments
curl.exe -H "Authorization: Bearer $token" http://localhost:8080/api/admin/users

<span class="comment"># 3. 新建文章（带 token）</span>
curl.exe -X POST -H "Authorization: Bearer $token" -H "Content-Type: application/json" `
  -d '{"title":"测试","content":"&lt;p&gt;你好&lt;/p&gt;","status":"published"}' `
  http://localhost:8080/api/admin/posts</code></pre>
  </section>
</template>
