<template>
  <section>
    <h2><span class="no">04</span>数据库设计</h2>
    <p>数据库名默认 <code>blog</code>（utf8mb4 字符集），共 3 张表。后端启动时<strong>自动创建</strong>，无需手动建表。</p>

    <h3>表 1：users（管理员账号）</h3>
    <table>
      <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      <tr><td>id</td><td>BIGINT UNSIGNED</td><td>自增主键</td></tr>
      <tr><td>username</td><td>VARCHAR(64)</td><td>用户名，唯一</td></tr>
      <tr><td>password_hash</td><td>VARCHAR(255)</td><td>bcrypt 加密后的密码（不是明文！）</td></tr>
      <tr><td>created_at</td><td>DATETIME</td><td>创建时间，默认当前时间</td></tr>
    </table>

    <h3>表 2：posts（博客文章）</h3>
    <table>
      <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      <tr><td>id</td><td>BIGINT UNSIGNED</td><td>自增主键</td></tr>
      <tr><td>title</td><td>VARCHAR(255)</td><td>文章标题</td></tr>
      <tr><td>summary</td><td>TEXT</td><td>摘要（列表页展示）</td></tr>
      <tr><td>cover</td><td>TEXT</td><td>封面图 URL（可空）</td></tr>
      <tr><td>content</td><td>LONGTEXT</td><td>正文 HTML（wangEditor 产出）</td></tr>
      <tr><td>status</td><td>VARCHAR(16)</td><td><code>draft</code> 草稿（仅后台可见）/ <code>published</code> 已发布（前台可见）</td></tr>
      <tr><td>likes</td><td>INT</td><td>点赞数</td></tr>
      <tr><td>created_at</td><td>DATETIME</td><td>创建时间</td></tr>
      <tr><td>updated_at</td><td>DATETIME</td><td>更新时间（每次修改自动刷新）</td></tr>
    </table>

    <h3>表 3：settings（键值对配置）</h3>
    <table>
      <tr><th>字段</th><th>类型</th><th>说明</th></tr>
      <tr><td>key</td><td>VARCHAR(64)</td><td>配置名（注意：key 是 MySQL 保留字，SQL 中需加反引号）</td></tr>
      <tr><td>value</td><td>TEXT</td><td>配置值</td></tr>
    </table>
    <p>目前只存一个配置：<code>jwt_secret</code>（JWT 签名密钥，首次启动自动生成）。</p>
  </section>
</template>
