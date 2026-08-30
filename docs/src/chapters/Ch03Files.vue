<template>
  <section>
    <h2><span class="no">03</span>目录结构与文件作用</h2>
    <pre><code>myblog/
├── <span class="comment"># ========== 前端（Vue 3） ==========</span>
├── frontend/
│   ├── index.html            <span class="comment"># 页面入口：一个挂载点 + 加载 /src/main.js</span>
│   ├── vite.config.js        <span class="comment"># 构建配置 + 开发代理（/api /uploads → :8080）</span>
│   ├── package.json          <span class="comment"># 依赖清单与脚本（dev/build/preview）</span>
│   ├── public/               <span class="comment"># 静态资源，原样复制到构建产物（含头像）</span>
│   └── src/
│       ├── main.js           <span class="comment"># 前端入口：创建 Vue 应用，挂载路由与全局指令</span>
│       ├── App.vue           <span class="comment"># 根组件：导航栏 + 页面出口(router-view) + 页脚</span>
│       ├── style.css         <span class="comment"># 全局样式：主题变量、按钮、文章排版</span>
│       ├── router/index.js   <span class="comment"># 路由表 + 登录守卫（未登录跳 /admin/login）</span>
│       ├── api/index.js      <span class="comment"># 统一请求封装：自动带 JWT、解析错误</span>
│       ├── directives/reveal.js   <span class="comment"># v-reveal 滚动出现动画指令</span>
│       ├── composables/      <span class="comment"># 可复用逻辑：主题/打字机/登录状态</span>
│       ├── components/       <span class="comment"># 导航栏、文章卡片、首页各区块组件</span>
│       └── views/            <span class="comment"># 页面：首页/博客列表/文章详情/后台登录/后台布局/文章管理/编辑器</span>
│
├── <span class="comment"># ========== 后端（Go） ==========</span>
├── backend/
│   ├── main.go       <span class="comment"># 入口：读配置 → 初始化数据库 → 注册路由 → 启动服务</span>
│   ├── db.go         <span class="comment"># MySQL 连接、自动建库建表、连接池</span>
│   ├── auth.go       <span class="comment"># 登录、JWT 签发与校验中间件、管理员初始化</span>
│   ├── posts.go      <span class="comment"># 文章增删改查、分页、点赞</span>
│   ├── upload.go     <span class="comment"># 图片上传（校验类型/大小，随机文件名）</span>
│   ├── static.go     <span class="comment"># 前端页面托管 + SPA 回退</span>
│   ├── models.go     <span class="comment"># 数据结构定义（JSON 字段名在这里定）</span>
│   ├── uploads/      <span class="comment"># 上传的图片存放目录</span>
│   └── blog.db       <span class="comment"># 旧版 SQLite 数据库（已弃用，可删除）</span>
│
├── <span class="comment"># ========== 其它 ==========</span>
├── docs/             <span class="comment"># 独立文档项目（本目录，与主项目解耦）</span>
├── docker-compose.yml <span class="comment"># 一条命令启动 MySQL 8</span>
├── README.md         <span class="comment"># 根目录快速指引（指向 docs/）</span>
└── pic/              <span class="comment"># 原始头像素材</span></code></pre>

    <h3>后端文件调用关系（按启动顺序）</h3>
    <p><code>main.go</code> → 调 <code>db.go</code> 的 <code>initDB</code>（连库建表）→ 调 <code>auth.go</code> 的 <code>ensureAdmin</code>（建管理员）→ 注册路由（处理器函数都在 <code>auth.go</code> / <code>posts.go</code> / <code>upload.go</code>）→ <code>static.go</code> 托管前端页面 → 监听端口。</p>
  </section>
</template>
