<template>
  <section>
    <h2><span class="no">10</span>公开接口详解</h2>

    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag pub">公开</span>/api/auth/login</div>
      <div class="desc">账号密码登录，成功后返回 JWT（后续所有管理接口都要用它）</div>
      <h4>请求体</h4>
      <pre><code>{ "username": "admin", "password": "admin123" }</code></pre>
      <h4>响应 200</h4>
      <pre><code>{ "token": "eyJhbGciOiJIUzI1NiIs...", "username": "admin" }</code></pre>
      <h4>错误</h4>
      <ul>
        <li>400：请求体缺用户名或密码</li>
        <li>401：用户名或密码错误（两者返回同样提示，防枚举）</li>
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
  "items": [
    {
      "id": 2,
      "title": "Rust 所有权小记",
      "summary": "借用一个简单例子…",
      "cover": "",
      "status": "published",
      "likes": 5,
      "createdAt": "2026-08-27 10:10:52",
      "updatedAt": "2026-08-27 10:10:52"
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 10
}</code></pre>
    </div>

    <div class="api">
      <div class="path"><span class="tag get">GET</span><span class="tag pub">公开</span>/api/posts/:id</div>
      <div class="desc">文章详情（含正文 HTML）。草稿对外不可见，访问草稿返回 404</div>
      <h4>响应 200</h4>
      <pre><code>{
  "id": 2,
  "title": "Rust 所有权小记",
  "summary": "…",
  "cover": "",
  "content": "&lt;h2&gt;引言&lt;/h2&gt;&lt;p&gt;所有权是…&lt;/p&gt;",
  "status": "published",
  "likes": 5,
  "createdAt": "2026-08-27 10:10:52",
  "updatedAt": "2026-08-27 10:10:52"
}</code></pre>
      <h4>错误</h4>
      <ul><li>400：ID 不是正整数；404：文章不存在或是草稿</li></ul>
    </div>

    <div class="api">
      <div class="path"><span class="tag post">POST</span><span class="tag pub">公开</span>/api/posts/:id/like</div>
      <div class="desc">点赞：数据库原子自增，返回最新点赞数</div>
      <h4>响应 200</h4>
      <pre><code>{ "likes": 6 }</code></pre>
      <h4>错误</h4>
      <ul><li>400：ID 非法；404：文章不存在</li></ul>
    </div>
  </section>
</template>
