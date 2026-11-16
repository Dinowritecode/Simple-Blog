<template>
  <section>
    <h2><span class="no">12</span>常见问题 FAQ</h2>
    <table>
      <tr><th>问题</th><th>解答</th></tr>
      <tr><td>后台登录提示「用户名或密码错误」</td><td>默认账号 admin/admin123；若改过环境变量，用改后的；确认后端日志里打印的默认账号</td></tr>
      <tr><td>启动报错 Error 1045 (Access denied)</td><td>MySQL 账号/密码不对，检查 DB_USER / DB_PASSWORD 环境变量</td></tr>
      <tr><td>启动报错 connect: connection refused</td><td>MySQL 没启动：<code>docker compose up -d</code> 或启动本地 MySQL 服务</td></tr>
      <tr><td>前台能打开但接口报「网络异常」</td><td>后端没启动或 MySQL 断了；看后端终端日志</td></tr>
      <tr><td>8080 打开的页面是旧内容</td><td>前端改过没重新构建：<code>cd frontend &amp;&amp; npm run build</code></td></tr>
      <tr><td>上传图片失败</td><td>图片类型不在白名单（jpg/jpeg/png/gif/webp）或超过 10MB；检查是否已登录（401）</td></tr>
      <tr><td>8080/5173 端口被占用</td><td>换端口：<code>$env:PORT="9000"; go run .</code> 或 <code>npm run dev -- --port 5174</code></td></tr>
      <tr><td>第一次 go run 一直下载依赖</td><td>正常，一次性。国内慢就加镜像：<code>$env:GOPROXY="https://goproxy.cn,direct"; go run .</code></td></tr>
    </table>
  </section>
</template>
