<template>
  <section>
    <h2><span class="no">13</span>参与开发指南</h2>

    <h3>如何新增一个前端页面</h3>
    <ol>
      <li>在 <code>frontend/src/views/</code> 新建 <code>XxxView.vue</code>（模板 + 脚本 + 样式写在一个文件里）</li>
      <li>在 <code>frontend/src/router/index.js</code> 的路由表里加一条：<code>{ path: '/xxx', name: 'xxx', component: () =&gt; import('../views/XxxView.vue') }</code></li>
      <li>需要登录的后台页面：加 <code>meta: { requiresAuth: true }</code>，并放进 <code>/admin</code> 的子路由</li>
    </ol>

    <h3>如何新增一个后端接口</h3>
    <ol>
      <li>在 <code>backend/posts.go</code>（或新文件）写处理函数：<code>func handleXxx(c *gin.Context) { ... }</code></li>
      <li>在 <code>backend/main.go</code> 的路由注册处加一行，例如：<code>api.GET("/xxx", handleXxx)</code></li>
      <li>需要登录就放在 <code>admin := api.Group("/admin", authMiddleware())</code> 分组内</li>
      <li>涉及数据库就用全局 <code>db</code>（database/sql 连接池），参数一律用 <code>?</code> 占位符防注入</li>
    </ol>

    <h3>调试技巧</h3>
    <ul>
      <li>前端调试：<code>npm run dev</code> 后浏览器 F12 → Network 面板看请求与响应</li>
      <li>后端调试：后端终端会打印每个请求的日志（Gin Logger）；也可以临时在 handler 里加 <code>log.Println(...)</code></li>
      <li>后端控制台：<code>go run .</code> 后在终端输入 <code>help</code> 查看命令——<code>info</code> 查看数据库/服务信息、<code>changepass</code> 修改管理员密码；也可以 <code>go run . info</code> 一次性执行（见第 7 节）</li>
      <li>数据库调试：按第 5 节连 MySQL，直接看数据对不对</li>
      <li>接口调试：用 curl（上文有示例）或 Postman/Apifox 导入接口</li>
    </ul>

    <h3>安全注意事项（给贡献者）</h3>
    <ul>
      <li>所有用户输入必须走参数化查询（<code>?</code> 占位符），不要拼 SQL 字符串</li>
      <li>密码永远 bcrypt 存储，不存明文</li>
      <li>管理接口必须挂在 authMiddleware 分组里</li>
      <li>上传文件必须校验类型与大小（见 upload.go）</li>
      <li>文章正文是 HTML，目前只在后台编辑（管理员自己写），未来若开放评论/投稿需加 XSS 过滤（如 DOMPurify）</li>
    </ul>
  </section>
</template>
