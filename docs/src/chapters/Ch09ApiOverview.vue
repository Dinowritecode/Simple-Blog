<template>
  <section>
    <h2><span class="no">09</span>API 总览与约定</h2>

    <h3>通用约定</h3>
    <table>
      <tr><th>项目</th><th>说明</th></tr>
      <tr><td>基础地址</td><td><code>http://localhost:8080/api</code></td></tr>
      <tr><td>数据格式</td><td>请求/响应均为 JSON（上传接口除外）</td></tr>
      <tr><td>字符编码</td><td>UTF-8（中文直接传即可）</td></tr>
      <tr><td>鉴权方式</td><td>需登录/需管理员的接口在请求头加 <code>Authorization: Bearer &lt;token&gt;</code>；「登录」= 任意注册用户，「管理员」= 站长（role=admin），否则 401 / 403</td></tr>
      <tr><td>错误格式</td><td>非 2xx 时返回 <code>{"error": "错误原因"}</code></td></tr>
      <tr><td>token 有效期</td><td>7 天，过期后需重新登录</td></tr>
    </table>

    <h3>接口一览</h3>
    <table>
      <tr><th>方法</th><th>路径</th><th>鉴权</th><th>说明</th></tr>
      <tr><td>POST</td><td>/api/auth/login</td><td>无</td><td>登录（邮箱或用户名），拿 token + 用户信息</td></tr>
      <tr><td>POST</td><td>/api/auth/register</td><td>无</td><td>访客注册（邮箱+密码），成功即自动登录</td></tr>
      <tr><td>GET</td><td>/api/posts</td><td>无</td><td>已发布文章分页列表</td></tr>
      <tr><td>GET</td><td>/api/posts/:slug</td><td>无</td><td>文章详情（随机短码，含 liked 状态）</td></tr>
      <tr><td>POST</td><td>/api/posts/:slug/like</td><td>无</td><td>点赞/取消（匿名，同一访客一次）</td></tr>
      <tr><td>GET</td><td>/api/posts/:slug/comments</td><td>无</td><td>评论列表（两级树）</td></tr>
      <tr><td>POST</td><td>/api/posts/:slug/comments</td><td>登录</td><td>发表评论 / 回复</td></tr>
      <tr><td>GET</td><td>/api/user/me</td><td>登录</td><td>当前用户信息</td></tr>
      <tr><td>PUT</td><td>/api/user/profile</td><td>登录</td><td>修改昵称</td></tr>
      <tr><td>PUT</td><td>/api/user/password</td><td>登录</td><td>修改密码（需原密码）</td></tr>
      <tr><td>POST</td><td>/api/user/avatar</td><td>登录</td><td>上传头像（≤5MB）</td></tr>
      <tr><td>GET</td><td>/api/user/notifications</td><td>登录</td><td>回复通知列表 + 未读数</td></tr>
      <tr><td>POST</td><td>/api/user/notifications/read</td><td>登录</td><td>标记通知已读</td></tr>
      <tr><td>GET/POST/PUT/DELETE</td><td>/api/admin/posts…</td><td>管理员</td><td>文章管理（含草稿）</td></tr>
      <tr><td>POST</td><td>/api/admin/upload</td><td>管理员</td><td>上传图片（正文/封面）</td></tr>
      <tr><td>GET/DELETE</td><td>/api/admin/comments…</td><td>管理员</td><td>评论管理（列表 / 删除）</td></tr>
      <tr><td>GET/DELETE</td><td>/api/admin/users…</td><td>管理员</td><td>用户管理（列表搜索 / 删除）</td></tr>
      <tr><td>GET</td><td>/api/settings</td><td>无</td><td>公开站点设置（如首页背景图 homeBg）</td></tr>
      <tr><td>PUT</td><td>/api/admin/settings/home</td><td>管理员</td><td>保存首页背景图 URL / 清空</td></tr>
      <tr><td colspan="4" class="group-row">桃信 · 私信与好友（详见第 14 章）</td></tr>
      <tr><td>GET</td><td>/api/user/events</td><td>token 查询参数</td><td>SSE 实时事件流（新私信 / 好友申请 / 回复通知）</td></tr>
      <tr><td>GET</td><td>/api/messenger/unread</td><td>登录</td><td>消息按钮红点汇总（私信 + 好友申请 + 未读回复）</td></tr>
      <tr><td>GET</td><td>/api/users/search?q=</td><td>登录</td><td>按 9 位桃信 ID 或昵称搜索用户</td></tr>
      <tr><td>GET</td><td>/api/users/:id</td><td>登录</td><td>用户桃信资料（含等级与好友关系）</td></tr>
      <tr><td>GET</td><td>/api/friends</td><td>登录</td><td>好友列表 + 好友申请</td></tr>
      <tr><td>POST</td><td>/api/friends/request</td><td>登录</td><td>发起好友申请</td></tr>
      <tr><td>POST</td><td>/api/friends/:id/accept · reject</td><td>登录</td><td>同意 / 拒绝好友申请</td></tr>
      <tr><td>DELETE</td><td>/api/friends/:id</td><td>登录</td><td>删除好友 / 撤回申请</td></tr>
      <tr><td>GET/POST</td><td>/api/conversations</td><td>登录</td><td>会话列表 / 打开（或创建）会话</td></tr>
      <tr><td>GET/POST</td><td>/api/conversations/:id/messages</td><td>登录</td><td>会话消息（读取即已读）/ 发私信</td></tr>
    </table>
    <div class="box tip">站长登录入口是 <code>/admin/login</code>（无注册，账号部署时自动创建）；访客注册/登录走前台右上角按钮（<code>/register</code>、<code>/login</code>）。</div>
  </section>
</template>
