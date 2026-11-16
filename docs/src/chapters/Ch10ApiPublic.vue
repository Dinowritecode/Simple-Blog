<template>
  <section>
    <h2><span class="no">10</span>公开接口详解</h2>

    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag pub">公开</span>/api/auth/login</div>
      <div class="desc">账号密码登录：账号填<strong>邮箱或用户名</strong>均可。成功后返回 JWT（后续所有需登录的接口都要带它）与用户信息</div>
      <h4>请求体</h4>
      <pre><code>{ "account": "admin", "password": "admin123" }</code></pre>
      <h4>响应 200</h4>
      <pre><code>{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": { "id": 1, "email": "admin@localhost", "username": "admin",
            "nickname": "", "avatar": "", "role": "admin" }
}</code></pre>
      <h4>错误</h4>
      <ul>
        <li>400：缺账号或密码</li>
        <li>401：账号或密码错误（用户不存在与密码错误提示一致，防枚举）</li>
      </ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag pub">公开</span>/api/auth/register</div>
      <div class="desc">注册：邮箱 + 密码（至少 6 位），暂未开通邮箱验证。成功后<strong>自动登录</strong>（直接返回 token + user，昵称默认取邮箱前缀）</div>
      <h4>请求体</h4>
      <pre><code>{ "email": "me@example.com", "password": "123456" }</code></pre>
      <h4>错误</h4>
      <ul>
        <li>400：邮箱格式不对 / 密码过短；409：该邮箱已被注册</li>
      </ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag pub">公开</span>/api/posts?page=1&amp;pageSize=10</div>
      <div class="desc">已发布文章分页列表（不含正文），按创建时间倒序</div>
      <h4>查询参数</h4>
      <table>
        <tr><th>参数</th><th>默认</th><th>说明</th></tr>
        <tr><td>page</td><td>1</td><td>页码，从 1 开始</td></tr>
        <tr><td>pageSize</td><td>10</td><td>每页条数，最大 50</td></tr>
      </table>
      <h4>响应 200</h4>
      <pre><code>{
  "items": [ { "id": 2, "title": "Rust 所有权小记", "summary": "…", "cover": "",
               "status": "published", "likes": 5, "createdAt": "…", "updatedAt": "…" } ],
  "total": 1, "page": 1, "pageSize": 10
}</code></pre>
    </div>

    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag pub">公开</span>/api/posts/:slug</div>
      <div class="desc">文章详情（含正文 HTML）。<strong>slug 是每篇文章创建时生成的随机短码</strong>（如 <code>Ksj2JayzYnDt</code>，非递增 id、非标题拼音），公开链接为 <code>/blog/&lt;slug&gt;</code>。草稿对外不可见，访问草稿返回 404。额外返回 <code>liked</code>：当前浏览器（Cookie 访客）是否已赞</div>
      <h4>响应 200</h4>
      <pre><code>{
  "id": 2, "title": "Rust 所有权小记", "summary": "…", "cover": "",
  "content": "&lt;h2&gt;引言&lt;/h2&gt;…", "status": "published",
  "likes": 5, "liked": true, "createdAt": "…", "updatedAt": "…"
}</code></pre>
      <h4>错误</h4>
      <ul><li>400：ID 不是正整数；404：文章不存在或是草稿</li></ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag pub">公开</span>/api/posts/:slug/like</div>
      <div class="desc">点赞 / 取消点赞。<strong>无需登录</strong>：同一浏览器（Cookie）对同一文章只能点一次，再次点击即取消。返回最新状态与数量</div>
      <h4>响应 200</h4>
      <pre><code>{ "liked": true, "likes": 6 }</code></pre>
      <h4>错误</h4>
      <ul><li>400：ID 非法；404：文章不存在或未发布</li></ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag pub">公开</span>/api/posts/:slug/comments</div>
      <div class="desc">文章评论列表（公开浏览）：返回<strong>顶级评论数组</strong>，每条含 <code>replies</code>（挂在它下面的回复），两级结构。作者信息含昵称 / 头像 / 角色</div>
      <h4>响应 200</h4>
      <pre><code>{
  "items": [
    {
      "id": 3, "postId": 2, "parentId": 0, "replyTo": null,
      "author": { "id": 5, "nickname": "小明", "avatar": "/uploads/avatars/u5_xx.png", "role": "user" },
      "content": "写得很棒！", "createdAt": "…", "replies": [
        { "id": 4, "postId": 2, "parentId": 3,
          "replyTo": { "id": 5, "nickname": "小明", "avatar": "", "role": "user" },
          "author": { "id": 1, "nickname": "admin", "avatar": "", "role": "admin" },
          "content": "谢谢支持～", "createdAt": "…", "replies": [] }
      ] }
  ],
  "total": 2
}</code></pre>
    </div>
    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag pub">公开</span>/api/posts/:slug/comments</div>
      <div class="desc">发表评论 / 回复（<strong>需登录</strong>，带 <code>Authorization: Bearer &lt;token&gt;</code>；普通用户与站长均可）。回复某人的评论时，对方会收到站内通知</div>
      <h4>请求体</h4>
      <pre><code>{ "content": "写得很棒！", "parentId": 3 }  <span class="comment">// parentId 省略/0 = 顶级评论；填评论 ID = 回复它</span></code></pre>
      <h4>响应 200</h4>
      <pre><code>{ "ok": true, "id": 5, "parentId": 3 }</code></pre>
      <h4>错误</h4>
      <ul><li>400：内容为空 / 超过 2000 字 / 要回复的评论不存在；401：未登录；404：文章不存在或未发布</li></ul>
    </div>

    <h3>登录用户接口（个人中心，均需带 token）</h3>
    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag auth">需登录</span>/api/user/me</div>
      <div class="desc">当前登录用户信息（id / email / nickname / avatar / role…），App 启动时用于刷新登录态。</div>
    </div>
    <div class="api">
      <div class="path"><span class="tag put">PUT</span><span class="tag auth">需登录</span>/api/user/profile</div>
      <div class="desc">修改昵称：<code>{"nickname": "新昵称"}</code>（1~30 字）。返回更新后的用户。</div>
    </div>
    <div class="api">
      <div class="path"><span class="tag put">PUT</span><span class="tag auth">需登录</span>/api/user/password</div>
      <div class="desc">修改密码（<strong>需提供原密码</strong>）：<code>{"oldPassword":"…","newPassword":"…（至少6位）"}</code>。原密码错误返回 400。</div>
    </div>
    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag auth">需登录</span>/api/user/avatar</div>
      <div class="desc">上传头像：<code>multipart/form-data</code>，字段名 <code>avatar</code>；仅图片、<strong>≤5MB</strong>（前端会先裁剪成正方形再上传）。响应 <code>{"avatar":"/uploads/avatars/u1_xx.jpg"}</code>。</div>
    </div>
    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag auth">需登录</span>/api/user/notifications?page=1</div>
      <div class="desc">我的「回复通知」分页列表；同时返回 <code>unreadCount</code>（铃铛角标用）。每项含触发者 actor、关联文章 postId、评论 commentId 与展示文本。</div>
    </div>
    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag auth">需登录</span>/api/user/notifications/read</div>
      <div class="desc">标记通知已读：<code>{"ids":[1,2]}</code> 标记指定；传空数组 / 不传 body 则全部标记已读。</div>
    </div>
  </section>
</template>
