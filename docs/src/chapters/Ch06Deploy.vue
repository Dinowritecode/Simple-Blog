<template>
  <section>
    <h2><span class="no">06</span>完整部署流程（从零开始）</h2>

    <h3>第 0 步：准备环境（只做一次）</h3>
    <ul>
      <li>安装 <strong>Go 1.25+</strong>：https://go.dev/dl/（验证：<code>go version</code>）</li>
      <li>安装 <strong>Node.js 18+</strong>：https://nodejs.org/（验证：<code>node --version</code>）</li>
      <li>安装 <strong>Docker</strong>（推荐，用于跑 MySQL）：https://www.docker.com/products/docker-desktop</li>
    </ul>

    <h3>第 1 步：启动 MySQL</h3>
    <p><strong>方式一：Docker（推荐）</strong>——在项目根目录执行：</p>
    <pre><code>docker compose up -d</code></pre>
    <p>会自动下载 MySQL 8 镜像并启动，账号密码见「数据库访问方法」。</p>
    <p><strong>方式二：本地安装 MySQL 8</strong>——安装后手动建库并授权：</p>
    <pre><code>CREATE DATABASE IF NOT EXISTS blog CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'blog'@'localhost' IDENTIFIED BY 'blog123456';
GRANT ALL PRIVILEGES ON blog.* TO 'blog'@'localhost';
FLUSH PRIVILEGES;</code></pre>
    <div class="box tip">后端启动时会<strong>自动建库建表</strong>，第 1 步只是保证「库存在、账号有权限」。</div>

    <h3>第 2 步：安装前端依赖并构建</h3>
    <pre><code><span class="comment"># 进入前端目录</span>
cd frontend
npm install
npm run build   <span class="comment"># 产物输出到 frontend/dist（后端会直接托管它）</span></code></pre>

    <h3>第 3 步：启动后端</h3>
    <pre><code><span class="comment"># 回到项目根目录，进入后端目录</span>
cd ../backend
go build -o blog-server.exe .    <span class="comment"># 编译（可跳过，直接用 go run）</span>
./blog-server.exe                 <span class="comment"># 或 go run .</span></code></pre>
    <p>看到日志 <code>The backend service has been successfully started: http://localhost:8080</code> 即成功（控制台出现 <code>&gt; </code> 提示符，可输入 <code>help</code> 查看命令）。若连接 MySQL 报错，检查数据库是否已启动、账号密码是否匹配。</p>
    <div class="box tip">国内网络下载 Go 依赖慢/失败时，先执行：<code>$env:GOPROXY="https://goproxy.cn,direct"; go run .</code></div>

    <h3>第 4 步：验证</h3>
    <table>
      <tr><th>地址</th><th>内容</th></tr>
      <tr><td>http://localhost:8080</td><td>个人主页</td></tr>
      <tr><td>http://localhost:8080/blog</td><td>博客列表</td></tr>
      <tr><td>http://localhost:8080/admin/login</td><td>后台登录</td></tr>
    </table>

    <h3>开发模式（改代码热更新）</h3>
    <p>开两个终端：终端 1 跑 <code>cd backend &amp;&amp; go run .</code>；终端 2 跑 <code>cd frontend &amp;&amp; npm run dev</code>，然后访问 <code>http://localhost:5173</code>。前端改动实时生效；后端改动需重启。Vite 会自动把 <code>/api</code>、<code>/uploads</code> 代理到 8080，所以前后端互通无跨域问题。</p>

    <h3>进阶：后端跑在云服务器（MySQL），本地调试前端（go run 模式）</h3>
    <p>适用场景：想在<strong>云服务器上用 MySQL</strong> 放正式数据，前端仍在本地 <code>npm run dev</code> 热更新调试。后端在服务器上用 <code>go run .</code> 运行（与编译产物等价，只是现场编译，方便看日志、改码重启）。本地 Vite 通过代理指向服务器，浏览器只见 localhost:5173，无跨域问题。</p>

    <h4>第 1 步：服务器装 Go 与 MySQL（只做一次，Ubuntu 示例）</h4>
    <pre><code><span class="comment"># Go 1.25+（版本号以官网为准，需匹配项目 go.mod 的 1.25.0）</span>
wget https://go.dev/dl/go1.25.3.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.3.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' &gt;&gt; ~/.bashrc &amp;&amp; source ~/.bashrc
go version

<span class="comment"># MySQL 8（本教程按 MySQL 写；若用 MariaDB，命令基本一致）</span>
sudo apt update &amp;&amp; sudo apt install -y mysql-server
sudo systemctl enable --now mysql
sudo mysql   <span class="comment"># Ubuntu 默认 root 走 auth_socket，直接用 sudo mysql 进入</span></code></pre>
    <p>在 MySQL 里建库建账号（仅服务器本机访问即可，<strong>别把 3306 开放到公网</strong>）：</p>
    <pre><code>CREATE DATABASE IF NOT EXISTS blog CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'blog'@'localhost' IDENTIFIED BY '换成强密码';
GRANT ALL PRIVILEGES ON blog.* TO 'blog'@'localhost';
FLUSH PRIVILEGES;</code></pre>
    <div class="box tip">表结构不用手动建——后端启动时会自动建库建表并完成迁移。如服务器是 ARM 架构，把上面的 tar 包换成 <code>linux-arm64</code> 版本。</div>

    <h4>第 2 步：把后端源码传到服务器并 go run</h4>
    <pre><code><span class="comment"># 本地 PowerShell：整个 backend 目录上传</span>
scp -r D:\website\backend root@你的服务器IP:/srv/blog/app
sudo mkdir -p /srv/blog/uploads</code></pre>
    <pre><code><span class="comment"># 服务器：用 tmux 保持进程不因断开 SSH 而死</span>
cd /srv/blog/app
tmux new -s blog

<span class="comment"># 在 tmux 里设置环境变量（国内服务器建议配 GOPROXY 加速首次拉依赖）</span>
export GOPROXY="https://goproxy.cn,direct"
export DB_HOST=127.0.0.1 DB_PORT=3306 DB_USER=blog DB_PASSWORD='强密码' DB_NAME=blog
export ADMIN_PASSWORD='站长强密码' JWT_SECRET='一长串随机字符'
export UPLOAD_DIR=/srv/blog/uploads
<span class="comment"># 聊天「实时状态层」用的 Redis（不配也能跑：只是没有在线状态与「正在输入」提示）</span>
<span class="comment"># 注意：Redis 只放短命状态（在线/输入中/限流/事件广播），消息正文始终落 MySQL</span>
export REDIS_ADDR=127.0.0.1:6379
export REDIS_PASSWORD=''
export REDIS_DB=0
export REDIS_PREFIX='blog:'
go run .</code></pre>
    <p>首次运行会自动下载 Go 依赖，看到 <code>The backend service has been successfully started</code> 即成功。tmux 用法：按 <code>Ctrl+B</code> 再按 <code>D</code> 离开会话（进程继续跑）；回来用 <code>tmux attach -t blog</code>。</p>
    <div class="box tip">启动日志里会出现三条与聊天存储有关的提示，确认它们符合预期：<br />
      · <code>聊天存储：已创建 messages 按月分区表</code>（或「空的旧 messages 表已升级为按月分区结构」）<br />
      · <code>Redis 实时状态层已启用：127.0.0.1:6379</code> —— 没配 REDIS_ADDR 时会提示「实时状态层关闭」，聊天功能不受影响<br />
      · 若看到「messages 表存在数据且不是分区结构」，说明表里有真实聊天记录，系统**故意不重建**以保证数据安全；想升级请先备份导出并清空消息，再在终端输入 <code>chatpartition</code> 命令。</div>
    <div class="box warn">后端默认监听所有网卡的 8080 端口，务必用防火墙<strong>只放行你的公网 IP</strong>：<code>sudo ufw allow from 你的公网IP to any port 8080 proto tcp</code>。否则任何人都能访问后台与上传接口。</div>

    <h4>第 3 步：本地前端连接服务器后端</h4>
    <p><code>frontend/vite.config.js</code> 支持用环境变量 <code>API_PROXY</code> 指定后端地址（默认仍为本地 8080）：</p>
    <pre><code><span class="comment"># 本地 PowerShell，进入 frontend 目录</span>
cd D:\website\frontend
$env:API_PROXY = "http://你的服务器IP:8080"
npm run dev</code></pre>
    <p>打开 <code>http://localhost:5173</code>：博客、登录注册、评论点赞、图片上传都走服务器后端与 MySQL；上传的文件存在服务器的 <code>/srv/blog/uploads</code>，经 <code>/uploads</code> 代理同样能显示。想切回本地后端调试：<code>Remove-Item Env:API_PROXY</code> 后重跑 <code>npm run dev</code>。</p>

    <h4>第 4 步：日常改码调试循环</h4>
    <ol>
      <li>本地改好后端某个 <code>.go</code> 文件 → 只上传该文件：<code>scp D:\website\backend\posts.go root@IP:/srv/blog/app/posts.go</code></li>
      <li>服务器 tmux 里 <code>Ctrl+C</code> → 重新 <code>go run .</code>，观察日志 / <code>&gt; </code> 控制台（可用 <code>info</code>、<code>changepass</code> 命令）</li>
      <li>浏览器刷新本地前端验证（后端改动需重启；前端在本地是热更新）</li>
    </ol>

    <h4>常见问题</h4>
    <table>
      <tr><th>现象</th><th>处理</th></tr>
      <tr><td><code>dial tcp 127.0.0.1:3306 ... refused</code></td><td>服务器 MySQL 没启动：<code>sudo systemctl start mysql</code></td></tr>
      <tr><td><code>Access denied for user 'blog'@'localhost'</code></td><td>DB_USER / DB_PASSWORD 与第 1 步建的用户不一致，核对 export</td></tr>
      <tr><td>本地前端提示「网络异常 / 502」</td><td>浏览器先访问 <code>http://服务器IP:8080/api/posts</code>：打不开 = 防火墙没放行你的 IP，或后端没起（tmux 里看日志）</td></tr>
      <tr><td>国内服务器拉 Go 依赖很慢</td><td>启动前 export <code>GOPROXY="https://goproxy.cn,direct"</code>；或本机 <code>go mod vendor</code> 后把 vendor/ 一起传上去，用 <code>go run -mod=vendor .</code></td></tr>
      <tr><td>后端重启时报 8080 被占用</td><td>tmux 里 <code>Ctrl+C</code> 无效时：<code>pkill -f 'go run'</code> 或 <code>pkill -f blog</code></td></tr>
    </table>

    <h3>部署到服务器（正式发布，前端也部署在服务器）</h3>
    <p>调试就绪后，想把<strong>整个站点</strong>（含前端与后台）长期跑在服务器上，可再按下面做（可选步骤，不影响上面的联调模式）：</p>
    <ol>
      <li>服务器装好 Go/Node/MySQL（或 Docker）</li>
      <li>按上面流程构建前端（<code>npm run build</code>）与后端（<code>go build</code>）</li>
      <li>把 <code>frontend/dist</code>、后端可执行文件、<code>docs/</code> 传到服务器</li>
      <li>设置环境变量（数据库地址、管理员密码等），启动后端进程（可用 systemd / supervisor 守护）</li>
      <li>可选：用 Nginx 反向代理 80 端口到 8080，并配 HTTPS</li>
    </ol>
  </section>
</template>
