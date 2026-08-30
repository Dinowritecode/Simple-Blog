import{_ as t,c as n,a as o,o as a}from"./index-B3LiEHuY.js";const e={};function c(r,s){return a(),n("section",null,[...s[0]||(s[0]=[o(`<h2><span class="no">05</span>数据库访问方法</h2><p>日常使用完全不需要碰数据库（后台页面就能管理一切）。以下方法供排查问题、手动改数据时使用。</p><h3>连接参数（docker-compose 默认值）</h3><table><tr><th>项目</th><th>值</th></tr><tr><td>地址 / 端口</td><td>127.0.0.1:3306</td></tr><tr><td>数据库名</td><td>blog</td></tr><tr><td>用户 / 密码</td><td>blog / blog123456（root / root123456）</td></tr></table><h3>方法 1：命令行（mysql 客户端）</h3><pre><code><span class="comment"># 登录（会提示输密码）</span>
mysql -h 127.0.0.1 -P 3306 -u blog -p blog

<span class="comment"># 看有哪些表</span>
SHOW TABLES;

<span class="comment"># 看文章表结构</span>
DESC posts;

<span class="comment"># 查所有已发布文章（标题、状态、点赞数）</span>
SELECT id, title, status, likes FROM posts WHERE status=&#39;published&#39;;

<span class="comment"># 把某篇文章改为「已发布」</span>
UPDATE posts SET status=&#39;published&#39; WHERE id=1;

<span class="comment"># 把点赞数清零</span>
UPDATE posts SET likes=0;</code></pre><h3>方法 2：图形化工具（推荐小白）</h3><p>任选一个：<strong>DBeaver</strong>（免费）、MySQL Workbench（官方免费）、Navicat（付费）。连接时填上面的参数即可，界面里可以直观地浏览/编辑数据。</p><div class="box warn"><strong>⚠️ 改数据要小心：</strong>直接改 <code>users.password_hash</code> 需要 bcrypt 加密后的值（不能用明文，否则登录会失败）。最简单的改密码方式见「后台访问方法」一节。 </div>`,9)])])}const d=t(e,[["render",c]]);export{d as default};
