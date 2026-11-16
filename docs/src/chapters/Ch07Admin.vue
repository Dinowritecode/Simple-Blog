<template>
  <section>
    <h2><span class="no">07</span>后台访问方法</h2>
    <ol>
      <li>打开站长登录页 <code>http://localhost:8080/admin/login</code>（此页<strong>只有登录、没有注册</strong>）</li>
      <li>站长账号部署时<strong>自动创建</strong>：首次部署会生成<strong>随机初始密码</strong>并打印在启动日志里（如 <code>首次部署：已自动创建站长账号 admin，初始密码为 xxxxxxxx</code>）；也可用环境变量 <code>ADMIN_PASSWORD</code> 固定</li>
      <li>登录成功自动进入后台（文章管理页）；侧边栏可切换到评论管理、用户管理；右上角头像菜单也能直达这三处</li>
    </ol>
    <div class="box warn">站长登录与访客登录是<strong>两条独立入口</strong>：站长走 <code>/admin/login</code>；访客（评论互动）用前台右上角「登录 / 注册」。普通用户即使登录也无法进入后台（返回 403）。</div>

    <h3>修改管理员密码</h3>
    <ul>
      <li><strong>页面上改：</strong>登录后把鼠标悬停在右上角头像上 → 菜单点「修改密码」，输入原密码与新密码即可（见下节「前台右上角」）。</li>
      <li><strong>终端里改：</strong>在运行 <code>go run .</code> 的终端输入 <code>changepass 新密码</code>（见下节「后端控制台命令」）。</li>
      <li><strong>部署时固定：</strong>首次启动前设置环境变量，例如 PowerShell：<code>$env:ADMIN_USER="myuser"; $env:ADMIN_PASSWORD="mypass"; go run .</code>（仅在尚无管理员账号时生效）</li>
      <li><strong>丢失密码：</strong>删掉 MySQL <code>users</code> 表里的记录后重启，会按当前策略重建（设了 <code>ADMIN_PASSWORD</code> 用它的值，否则重新生成随机密码打印到日志）</li>
    </ul>

    <h3>前台右上角（登录入口 &amp; 用户区）</h3>
    <ul>
      <li>未登录时，右上角显示「登录」「注册」两个按钮：「登录」进 <code>/login</code>（邮箱或用户名 + 密码）；「注册」进 <code>/register</code>（只需邮箱 + 密码，暂未开通邮箱验证，注册即自动登录）。站长不在此注册。</li>
      <li>注册完成后页面顶部会弹出提示「注册成功」，可点击<strong>「去上传头像」</strong>进入头像上传弹窗（支持拖拽定位 + 缩放裁剪，输出正方形头像，原图不超过 <strong>5MB</strong>）。</li>
      <li>登录后右上角变成<strong>铃铛 + 头像</strong>：铃铛显示「回复通知」未读红点，点开可看到谁回复了你的评论，点击跳转到对应文章并定位评论；<strong>鼠标悬停头像</strong>弹出小菜单：个人资料（改昵称 / 换头像）、修改密码（需原密码）、退出登录。</li>
      <li>站长（admin）头像菜单里额外有「站长管理」分区：<strong>文章管理 / 评论管理 / 用户管理</strong>——外来注册用户的管理在这里即可直达，不必先回首页。</li>
      <li>普通用户即可<strong>评论 / 回复</strong>（需登录），回复某人的评论时对方会收到通知。</li>
    </ul>

    <h3>后台评论管理、用户管理与页面管理</h3>
    <p>侧边栏相关模块：</p>
    <ul>
      <li><strong>💬 评论管理</strong>（/admin/comments）：查看全部评论与回复（作者、所属文章），可删除；删除<strong>顶级评论会连同其下回复一起删</strong>。评论发布后直接公开展示，无需审核。</li>
      <li><strong>👥 用户管理</strong>（/admin/users）：按邮箱 / 昵称搜索，查看注册用户及其评论数；可删除用户（连带删除其评论与通知）。管理员账号不可删除，也不能删除自己。</li>
      <li><strong>🎨 页面管理</strong>（/admin/settings）：设置<strong>首页背景图</strong>——可直接<strong>上传本地图片</strong>，也可<strong>粘贴图片 URL</strong>（支持 http(s):// 或站内 /uploads 路径），带实时预览与「清除」；保存后刷新首页顶部即可看到效果（会自动加暗色遮罩保证文字可读）。</li>
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
