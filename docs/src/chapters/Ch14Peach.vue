<template>
  <section>
    <h2><span class="no">14</span>桃信 · 私信与好友系统</h2>

    <p class="lead">「桃信」是站内的私信小窗：每个用户有一个 <strong>9 位桃信 ID</strong>，可以按 ID 搜人、加好友、实时聊天，评论区也能直接私信作者。它不是独立页面，而是浮在主页上的小窗口。</p>

    <h3>14.1 功能一览</h3>
    <table>
      <tr><th>能力</th><th>使用位置</th><th>说明</th></tr>
      <tr><td>9 位桃信 ID</td><td>头像菜单 / 资料卡</td><td>注册时自动分配（100000000 ~ 999999999），全站唯一，别人靠它找到你</td></tr>
      <tr><td>按 ID 搜人</td><td>小窗左侧搜索框</td><td>输入 9 位数字或昵称即可搜索，并直接加好友 / 发消息</td></tr>
      <tr><td>加好友</td><td>搜索结果 / 资料卡</td><td>发起申请 → 对方在「新的朋友」里同意 / 拒绝；双向申请会自动成为好友</td></tr>
      <tr><td>好友通知红点</td><td>导航栏「消息」按钮 / 右下角小圆钮</td><td>未读私信 + 待处理好友申请 + 未读回复，三项汇总成红点数字</td></tr>
      <tr><td>私信聊天</td><td>小窗「私信」</td><td>消息实时到达（SSE 推送），支持向上翻页看历史</td></tr>
      <tr><td>评论区私聊</td><td>文章评论</td><td>每条评论都有「私信」按钮，点一下直接和作者开聊（无需先加好友）</td></tr>
      <tr><td>用户等级</td><td>列表头像右下角爱心</td><td>粉色爱心里的数字就是等级（当前只展示，暂不接经验值）</td></tr>
    </table>

    <h3>14.2 窗口的三种状态（动画顺序）</h3>
    <div class="box">
      <ol>
        <li><strong>开场</strong>：屏幕中央一颗桃子图标<strong>从无到有放大出现</strong>（粉色区域此时是中间那块大圆角矩形）</li>
        <li><strong>变形</strong>：粉色区域<strong>缩小并上移</strong>，收成面板顶部那条粉色标题栏；图标同步缩小淡出，正好落进标题栏的 Logo 位置</li>
        <li><strong>面板</strong>：左侧图标栏 + 中间列表列 + 右侧资料卡 / 聊天区淡入</li>
      </ol>
      <p class="comment">关闭时倒着播一遍（面板淡出 → 粉色回落到中间 → 消失）。实现见 <code>frontend/src/components/messenger/MessengerWindow.vue</code>：粉色层与图标都靠 CSS transition 过渡位置 / 尺寸 / 圆角，阶段由 <code>splash → morph → panel</code> 三个 class 驱动。</p>
    </div>

    <h3>14.3 实时消息：为什么用 SSE</h3>
    <div class="box">
      <p>私信的实时需求是「服务器 → 浏览器」单向推送，发消息本身走普通 POST 就够了，所以没有引入 WebSocket，而是用 <strong>SSE（Server-Sent Events）</strong>：</p>
      <ul>
        <li>SSE 就是一条普通 HTTP 长连接，Go 标准库直接支持（<code>http.Flusher</code>），不用第三方依赖</li>
        <li>浏览器 <code>EventSource</code> 断线会自动重连（默认 3 秒）</li>
        <li>成本低：一个用户一条连接，25 秒发一次心跳注释帧防止被代理掐断</li>
      </ul>
      <p>事件类型三种：<code>chat</code>（新私信）、<code>friend</code>（好友申请 / 被同意 / 被删除）、<code>notification</code>（评论回复）。前端收到后立刻刷新对应数据，所以聊天和红点都是实时的。</p>
      <p class="comment">注意：<code>EventSource</code> 不能自定义请求头，token 只能放在查询参数 <code>?token=xxx</code>；因此这条路由刻意注册在「公开组」并由 handler 自己鉴权（见 <code>backend/main.go</code> 的注释），否则会被只认 Authorization 头的登录中间件先挡成 401。</p>
    </div>

    <h3>14.4 存储分层：谁存什么，为什么这么分</h3>
    <p class="lead">聊天的数据可以分成两类，它们的读写特征完全相反，所以故意放到了两个不同的存储里：</p>
    <table>
      <tr><th></th><th>消息正文（持久层）</th><th>实时状态（Redis）</th></tr>
      <tr><td>内容</td><td>消息、会话、已读位置、好友关系</td><td>谁在线、谁正在输入、发消息频率计数、跨实例事件广播</td></tr>
      <tr><td>特征</td><td>只增不改、必须永久保存、要能翻历史</td><td>生命周期几秒到几分钟、丢了无所谓、写入极频繁</td></tr>
      <tr><td>存放</td><td><strong>MariaDB 按月分区表</strong></td><td><strong>Redis</strong>（带 TTL 的短命键 + pub/sub）</td></tr>
      <tr><td>换成 MySQL 会怎样</td><td>合适</td><td>每个心跳都写库、还要定时清理过期键，非常不划算</td></tr>
      <tr><td>换成 Redis 会怎样</td><td>不合适：内存放不下历史消息，且不保证持久</td><td>合适</td></tr>
    </table>

    <div class="box tip">
      <p><strong>Redis 是可选的加速件，不是必需品。</strong>没配 <code>REDIS_ADDR</code> 时整个实时层关闭：接口照常工作，只是不显示「在线 / 正在输入」；运行中 Redis 掉线也会自动降级（在线一律返回 false、限流直接放行、广播退回单进程内分发）。所以「Redis 挂了」不会让聊天不可用。</p>
    </div>

    <h3>14.5 消息表为什么按月分区</h3>
    <div class="box">
      <p>聊天的数据是「只增不改、越老越冷」，按月分区（<code>PARTITION BY RANGE COLUMNS(created_at)</code>）带来两个直接好处：</p>
      <ul>
        <li><strong>归档</strong>：<code>ALTER TABLE messages DROP PARTITION p202601</code> 是元数据操作，秒级完成；如果改用 <code>DELETE FROM messages WHERE created_at &lt; ...</code>，会扫全表并留下大量 undo/碎片 —— 这是聊天库最常见的性能事故来源</li>
        <li><strong>查询裁剪</strong>：按时间范围查历史时只扫命中的分区</li>
      </ul>
      <p>代价有两个，都在代码里处理掉了：</p>
      <ol>
        <li><strong>主键必须包含分区键</strong>：主键是 <code>(id, conversation_id, created_at)</code>。id 必须排最前 —— MySQL 要求自增列是某个键的首列（否则报 1075）；这样主键前缀又刚好支撑「按 id 倒序取最近 N 条」，会话维度另有 <code>idx_conv</code>。</li>
        <li><strong>分区表不允许外键</strong>（错误 1506）：所以 <code>messages</code> 上原来的 <code>ON DELETE CASCADE</code> 改由两个<strong>触发器</strong>承担（删会话→删该会话消息、删用户→删他发的消息），语义与原来完全一致。见 <code>chatstore/migrate.go</code> 的 <code>ensureMessageTriggers</code>。</li>
      </ol>
      <p class="comment">分区边界取「自然月 1 号」而不是「今天」，这样 <code>p202609</code> 就是 9 月整月，归档时语义清晰。后台任务每天自动把未来 3 个月的分区建好。</p>
    </div>

    <h3>14.6 存储层是可替换的（为换 PostgreSQL 预留）</h3>
    <div class="box">
      <p>所有消息/会话的读写都收在 <code>backend/chatstore</code> 包的 <code>Store</code> 接口后面，HTTP 层（<code>chat.go</code>）不写任何 SQL。所以换库的动作是：</p>
      <ol>
        <li>按 <code>chatstore/schema_postgres.sql</code> 建表（已给出等价的分区 + 索引设计）</li>
        <li>新增 <code>chatstore/postgres.go</code> 实现同一个 <code>Store</code> 接口</li>
        <li><code>main.go</code> 里把 <code>chatstore.NewMariaDB(db)</code> 换成新实现</li>
      </ol>
      <p class="comment">当前这台机器只装了 MariaDB，所以 PostgreSQL 那份只作为迁移蓝图，不参与编译。</p>
    </div>

    <h3>14.7 数据库表设计</h3>
    <table>
      <tr><th>表</th><th>作用</th><th>关键字段 / 结构</th></tr>
      <tr>
        <td><code>users</code>（新增两列）</td>
        <td>用户表补上桃信身份</td>
        <td><code>user_id</code> 9 位桃信 ID（唯一）、<code>level</code> 等级（默认 1）</td>
      </tr>
      <tr>
        <td><code>friends</code></td>
        <td>好友关系（含待处理申请）</td>
        <td><code>small_id</code> / <code>big_id</code>（恒有 small &lt; big，保证一对用户只有一行）、<code>requester_id</code> 发起人、<code>status</code> = pending / accepted / rejected</td>
      </tr>
      <tr>
        <td><code>conversations</code></td>
        <td>一对一会话</td>
        <td><code>user_a</code> / <code>user_b</code>（同样 small &lt; big）+ 唯一索引</td>
      </tr>
      <tr>
        <td><code>conversation_members</code></td>
        <td>会话成员（每人一行）</td>
        <td><code>last_read_id</code> 我读到哪条了 → 未读数 = 会话内 id 更大的消息条数；普通外键级联</td>
      </tr>
      <tr>
        <td><code>messages</code></td>
        <td>私信正文（<strong>按月分区</strong>）</td>
        <td>主键 <code>(id, conversation_id, created_at)</code>、<code>idx_conv</code>、<code>idx_sender</code>；无外键，改用触发器级联</td>
      </tr>
    </table>
    <p>另外两个设计取舍：</p>
    <ul>
      <li><strong>为什么 small_id &lt; big_id</strong>：同一对用户无论谁先操作都落到同一行，靠唯一索引天然去重，不需要查两次。</li>
      <li><strong>为什么把「谁发起的」单独存</strong>：同一行既能表示「A 申请加 B（待处理）」，也能表示「A 与 B 已是好友」，省掉一张申请表。</li>
      <li><strong>会话与好友解耦</strong>：评论区可以直接私信陌生人（不必先加好友），所以会话表独立存在。</li>
    </ul>

    <h3>14.8 Redis 里都放了什么</h3>
    <table>
      <tr><th>键 / 频道</th><th>类型</th><th>TTL</th><th>作用</th></tr>
      <tr><td><code>blog:online:&lt;用户ID&gt;</code></td><td>字符串</td><td>90 秒</td><td>在线状态；SSE 连接建立时写入、每 25 秒心跳续期、断开即删</td></tr>
      <tr><td><code>blog:typing:&lt;会话ID&gt;:&lt;用户ID&gt;</code></td><td>字符串</td><td>6 秒</td><td>「正在输入」，前端每 3 秒续一次，不续就自动消失</td></tr>
      <tr><td><code>blog:rate:&lt;用户ID&gt;:&lt;时间窗&gt;</code></td><td>计数器</td><td>10 秒</td><td>发消息限流（10 秒内最多 20 条）</td></tr>
      <tr><td><code>blog:events</code></td><td>pub/sub 频道</td><td>—</td><td>跨实例广播新私信 / 好友申请 / 输入中 / 上下线</td></tr>
    </table>
    <p class="comment">注意：<strong>Redis 里不放任何消息正文</strong>。消息只落 MariaDB，Redis 只承担「短命状态」与「事件分发」，所以即使 Redis 数据全丢，聊天记录也不会少一条。</p>
    <div class="box">
      <p>环境变量（都有默认值，不配也能跑）：</p>
      <table>
        <tr><th>变量</th><th>默认</th><th>说明</th></tr>
        <tr><td><code>REDIS_ADDR</code></td><td>（空）</td><td>为空 = 不启用实时层；如 <code>127.0.0.1:6379</code></td></tr>
        <tr><td><code>REDIS_PASSWORD</code></td><td>（空）</td><td>Redis 密码</td></tr>
        <tr><td><code>REDIS_DB</code></td><td><code>0</code></td><td>库号</td></tr>
        <tr><td><code>REDIS_PREFIX</code></td><td><code>blog:</code></td><td>键前缀，便于多套环境共用一个 Redis</td></tr>
      </table>
    </div>

    <h3>14.9 完整启动流程示例</h3>
    <div class="box">
      <pre>
①注册两个用户 → 各自拿到 9 位桃信 ID（如 A=602590943，B=256882874）
②A 搜索 B 的 ID      → GET  /api/users/search?q=256882874
③A 发起好友申请      → POST /api/friends/request      {"userId":"256882874"}
④B 收到实时红点      → SSE  event: friend
⑤B 同意              → POST /api/friends/{relationId}/accept
⑥A 打开会话并发消息  → POST /api/conversations        {"peerId":"256882874"}

                       POST /api/conversations/3/messages  {"content":"在吗？"}
⑦B 在线时立即收到    → SSE  event: chat   （消息直接进聊天窗口）
⑧B 打字              → POST /api/conversations/3/typing
   A 立刻看到          → SSE  event: typing （6 秒后自动消失）
⑨B 打开会话即已读    → GET  /api/conversations/3/messages（后端顺手推进 last_read_id）</pre>
    </div>

    <h3>14.10 接口清单</h3>
    <table>
      <tr><th>方法</th><th>路径</th><th>鉴权</th><th>说明</th></tr>
      <tr><td>GET</td><td>/api/user/events</td><td>token 查询参数</td><td>SSE 实时事件流（chat / typing / friend / presence / notification）</td></tr>
      <tr><td>GET</td><td>/api/messenger/unread</td><td>登录</td><td>红点汇总：未读私信 / 待处理申请 / 未读回复</td></tr>
      <tr><td>GET</td><td>/api/users/search?q=</td><td>登录</td><td>按 9 位桃信 ID 或昵称搜索（带 relation 关系）</td></tr>
      <tr><td>GET</td><td>/api/users/:id</td><td>登录</td><td>用户桃信资料（:id 支持内部 ID 或 9 位 ID，含 online）</td></tr>
      <tr><td>GET</td><td>/api/friends</td><td>登录</td><td>好友列表（含最近一条私信、未读数、online）+ 申请列表</td></tr>
      <tr><td>GET</td><td>/api/friends/requests</td><td>登录</td><td>只取申请列表（incoming / outgoing）</td></tr>
      <tr><td>POST</td><td>/api/friends/request</td><td>登录</td><td>发起好友申请 <code>{"userId":"..."}</code></td></tr>
      <tr><td>POST</td><td>/api/friends/:id/accept</td><td>登录</td><td>同意申请（:id 是 friends 表行 ID）</td></tr>
      <tr><td>POST</td><td>/api/friends/:id/reject</td><td>登录</td><td>拒绝申请</td></tr>
      <tr><td>DELETE</td><td>/api/friends/:id</td><td>登录</td><td>删除好友 / 撤回申请（:id 是对方用户 ID）</td></tr>
      <tr><td>GET</td><td>/api/conversations</td><td>登录</td><td>会话列表（含未读数、最近一条私信、peer.online）</td></tr>
      <tr><td>POST</td><td>/api/conversations</td><td>登录</td><td>打开（不存在则创建）会话 <code>{"peerId":"..."}</code></td></tr>
      <tr><td>GET</td><td>/api/conversations/:id/messages</td><td>登录</td><td>会话消息（limit / before 翻页），并标记已读，附带 peerOnline / peerTyping</td></tr>
      <tr><td>POST</td><td>/api/conversations/:id/messages</td><td>登录</td><td>发私信 <code>{"content":"..."}</code>；触发限流时返回 429 + Retry-After</td></tr>
      <tr><td>POST</td><td>/api/conversations/:id/read</td><td>登录</td><td>把会话标记为已读</td></tr>
      <tr><td>POST</td><td>/api/conversations/:id/typing</td><td>登录</td><td>「我正在输入」（只写 Redis，不落库）</td></tr>
    </table>

    <h3>14.11 运维命令</h3>
    <p>后端自带一个终端命令控制台（启动后在终端直接输入），与聊天相关的命令：</p>
    <table>
      <tr><th>命令</th><th>说明</th></tr>
      <tr><td><code>info</code></td><td>查看服务 / 数据库 / 账户信息</td></tr>
      <tr><td><code>chatpartition</code></td><td>把<strong>空的</strong>消息表升级为按月分区结构；表里还有消息时会拒绝执行（不冒险）。想升级请先备份并清空消息</td></tr>
    </table>

    <h3>14.12 相关文件</h3>
    <table>
      <tr><th>文件</th><th>职责</th></tr>
      <tr><td>backend/chatstore/store.go</td><td>存储层接口（Store）+ 统一的 pair 规范化</td></tr>
      <tr><td>backend/chatstore/mariadb.go</td><td>MariaDB 实现：分区 DDL、消息读写、未读统计、分区维护</td></tr>
      <tr><td>backend/chatstore/migrate.go</td><td>分区升级、触发器、迁移锁；含「有数据就不动」的安全策略</td></tr>
      <tr><td>backend/chatstore/schema_postgres.sql</td><td>PostgreSQL 等价 schema（换库蓝图）</td></tr>
      <tr><td>backend/rts.go</td><td>Redis 实时状态层：在线 / 输入中 / 限流 / pub-sub，全部可降级</td></tr>
      <tr><td>backend/events.go</td><td>SSE 事件总线 + 本机与跨实例广播</td></tr>
      <tr><td>backend/friends.go</td><td>用户搜索、好友申请与处理</td></tr>
      <tr><td>backend/chat.go</td><td>私信 HTTP 层（只调 store/rts，不写 SQL）</td></tr>
      <tr><td>frontend/src/composables/useMessenger.js</td><td>小窗状态 + SSE 连接 + 全部桃信数据读写</td></tr>
      <tr><td>frontend/src/components/messenger/</td><td>MessengerWidget（入口圆钮）、MessengerWindow（小窗与动画）、FriendList、UserProfilePane、ChatPane、UserSearchBox、UserRow、LevelHeart</td></tr>
    </table>
  </section>
</template>
