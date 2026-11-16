<template>
  <section>
    <h2><span class="no">04</span>数据库设计</h2>
    <p>数据库名默认 <code>blog</code>（utf8mb4 字符集），共 <strong>6 张表</strong>。后端启动时<strong>自动建库建表</strong>，老库会自动 ALTER 迁移到新版结构，无需手动操作。</p>

    <h3>表 1：users（所有账户：管理员 + 注册用户）</h3>
    <table>
      <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      <tr><td>id</td><td>BIGINT UNSIGNED</td><td>自增主键</td></tr>
      <tr><td>username</td><td>VARCHAR(64) NULL</td><td>用户名（仅管理员使用，如 admin；注册用户为 NULL）</td></tr>
      <tr><td>email</td><td>VARCHAR(190) NULL</td><td>邮箱（注册用户登录标识，唯一）</td></tr>
      <tr><td>password_hash</td><td>VARCHAR(255)</td><td>bcrypt 加密后的密码（不是明文！）</td></tr>
      <tr><td>nickname</td><td>VARCHAR(64)</td><td>昵称（评论区展示，默认取邮箱前缀）</td></tr>
      <tr><td>avatar</td><td>VARCHAR(255)</td><td>头像 URL（/uploads/avatars/…）</td></tr>
      <tr><td>role</td><td>VARCHAR(16)</td><td>角色：<code>admin</code> 管理员 / <code>user</code> 普通用户</td></tr>
      <tr><td>created_at</td><td>DATETIME</td><td>创建时间，默认当前时间</td></tr>
    </table>
    <div class="box tip">登录时「账号」填邮箱或用户名均可（后台 admin 用用户名 admin）。普通用户注册只需邮箱 + 密码。</div>

    <h3>表 2：posts（博客文章）</h3>
    <table>
      <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      <tr><td>id</td><td>BIGINT UNSIGNED</td><td>自增主键（后台编辑用，不对外）</td></tr>
      <tr><td>slug</td><td>VARCHAR(32) NULL</td><td>公开链接随机短码（/blog/&lt;slug&gt;，创建时自动生成，唯一；老文章启动时自动回填）</td></tr>
      <tr><td>title</td><td>VARCHAR(255)</td><td>文章标题</td></tr>
      <tr><td>summary</td><td>TEXT</td><td>摘要（列表页展示）</td></tr>
      <tr><td>cover</td><td>TEXT</td><td>封面图 URL（可空）</td></tr>
      <tr><td>content</td><td>LONGTEXT</td><td>正文 HTML（wangEditor 产出）</td></tr>
      <tr><td>status</td><td>VARCHAR(16)</td><td><code>draft</code> 草稿 / <code>published</code> 已发布</td></tr>
      <tr><td>likes</td><td>INT</td><td>点赞数（冗余计数器，与 post_likes 表保持一致）</td></tr>
      <tr><td>created_at</td><td>DATETIME</td><td>创建时间</td></tr>
      <tr><td>updated_at</td><td>DATETIME</td><td>更新时间（每次修改自动刷新）</td></tr>
    </table>

    <h3>表 3：settings（键值对配置）</h3>
    <table>
      <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      <tr><td>key</td><td>VARCHAR(64)</td><td>配置名（key 是 MySQL 保留字，SQL 中需加反引号）</td></tr>
      <tr><td>value</td><td>TEXT</td><td>配置值</td></tr>
    </table>
    <p>目前只存一个配置：<code>jwt_secret</code>（JWT 签名密钥，首次启动自动生成）。</p>

    <h3>表 4：comments（评论 / 回复）</h3>
    <table>
      <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      <tr><td>id</td><td>BIGINT UNSIGNED</td><td>自增主键</td></tr>
      <tr><td>post_id</td><td>BIGINT UNSIGNED</td><td>所属文章</td></tr>
      <tr><td>user_id</td><td>BIGINT UNSIGNED</td><td>评论者</td></tr>
      <tr><td>parent_id</td><td>BIGINT UNSIGNED NULL</td><td>顶级评论 ID（NULL = 顶级评论）</td></tr>
      <tr><td>reply_to_user_id</td><td>BIGINT UNSIGNED NULL</td><td>回复时 @ 的目标用户</td></tr>
      <tr><td>content</td><td>TEXT</td><td>内容（纯文本，≤2000 字）</td></tr>
      <tr><td>created_at</td><td>DATETIME</td><td>创建时间</td></tr>
    </table>
    <div class="box tip">评论只保留<strong>两级</strong>：回复「回复」时仍挂在顶级评论下、用 reply_to_user_id 记住 @ 的对象，界面不无限嵌套。</div>

    <h3>表 5：notifications（站内回复通知）</h3>
    <table>
      <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      <tr><td>id</td><td>BIGINT UNSIGNED</td><td>自增主键</td></tr>
      <tr><td>user_id</td><td>BIGINT UNSIGNED</td><td>接收者</td></tr>
      <tr><td>actor_id</td><td>BIGINT UNSIGNED NULL</td><td>触发者（回复我的人）</td></tr>
      <tr><td>post_id</td><td>BIGINT UNSIGNED</td><td>关联文章</td></tr>
      <tr><td>comment_id</td><td>BIGINT UNSIGNED NULL</td><td>关联评论（点击跳转定位）</td></tr>
      <tr><td>content</td><td>VARCHAR(500)</td><td>展示文本（含昵称快照）</td></tr>
      <tr><td>is_read</td><td>TINYINT(1)</td><td>是否已读</td></tr>
      <tr><td>created_at</td><td>DATETIME</td><td>创建时间</td></tr>
    </table>

    <h3>表 6：post_likes（点赞去重）</h3>
    <table>
      <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      <tr><td>post_id</td><td>BIGINT UNSIGNED</td><td>文章 ID（与 visitor 联合主键）</td></tr>
      <tr><td>visitor</td><td>VARCHAR(64)</td><td>访客标识（浏览器 Cookie，匿名）</td></tr>
      <tr><td>created_at</td><td>DATETIME</td><td>点赞时间</td></tr>
    </table>
    <div class="box tip">点赞<strong>无需登录</strong>：同一浏览器（Cookie）对同一文章只能赞一次，可再次点击取消。posts.likes 为冗余计数，事务内与 post_likes 同步增减。</div>
  </section>
</template>
