<template>
  <section>
    <h2><span class="no">07</span>后台访问方法</h2>
    <ol>
      <li>打开 <code>http://localhost:8080/admin/login</code>（开发模式为 5173）</li>
      <li>输入默认账号：<strong>admin / admin123</strong></li>
      <li>登录后进入文章管理页</li>
    </ol>

    <h3>修改管理员密码</h3>
    <ul>
      <li><strong>最简单（推荐）：用后端控制台命令</strong>——在运行 <code>go run .</code> 的终端里输入 <code>changepass 新密码</code>，立即生效、无需重启，见下节「后端控制台命令」。</li>
      <li><strong>首次启动前：</strong>用环境变量指定，例如 PowerShell：<code>$env:ADMIN_USER="myuser"; $env:ADMIN_PASSWORD="mypass"; go run .</code>（仅在 users 表为空时生效）</li>
      <li><strong>已启动过：</strong>删掉 MySQL <code>users</code> 表里的记录后重启，会用环境变量（或默认值）重建；或在数据库里把 <code>password_hash</code> 改成 bcrypt 加密后的值</li>
    </ul>

    <h3>后端控制台命令</h3>
    <p>运行 <code>go run .</code> 后，后端会在终端里提供一个交互式控制台（跑在独立 goroutine，不影响 HTTP 服务），输入 <code>help</code> 可随时查看命令列表：</p>
    <table>
      <tr><th>命令</th><th>作用</th></tr>
      <tr><td><code>help</code></td><td>显示所有可用命令</td></tr>
      <tr><td><code>info</code></td><td>查看服务 / 数据库 / 账户信息：HTTP 端口、MySQL 地址与账号密码、数据库名、连接状态、JWT 密钥状态、文章统计</td></tr>
      <tr><td><code>changepass</code></td><td>交互式修改管理员密码（按提示输入两次确认）</td></tr>
      <tr><td><code>changepass &lt;新密码&gt;</code></td><td>直接修改管理员密码（至少 6 位）</td></tr>
      <tr><td><code>changepass &lt;用户名&gt; &lt;新密码&gt;</code></td><td>修改指定用户的密码</td></tr>
      <tr><td><code>exit</code> / <code>quit</code></td><td>退出后端</td></tr>
    </table>
    <p>也可以不启动服务、一次性执行：<code>go run . info</code>、<code>go run . changepass admin 123456</code>。</p>
    <div class="box tip">控制台命令依赖数据库连接；后台运行（如部署为服务、无终端输入）时控制台自动静默禁用，不影响服务。</div>
  </section>
</template>
