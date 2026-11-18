const BASE = '/api'

/**
 * 统一请求封装：自动携带 JWT、解析 JSON、抛出带状态码的错误
 * 401（登录过期）时自动清除本地凭证
 */
async function request(path, options = {}) {
  const token = localStorage.getItem('token')
  const headers = { ...(options.headers || {}) }
  const isForm = options.body instanceof FormData
  if (options.body && !isForm) headers['Content-Type'] = 'application/json'
  if (token) headers['Authorization'] = `Bearer ${token}`

  let res
  try {
    res = await fetch(BASE + path, { ...options, headers })
  } catch {
    throw new Error('网络异常，请确认后端服务已启动')
  }

  // 204 / 空响应体 / 非 JSON 容错（JSON.parse 是同步的，用 try/catch 而不是 .catch）
  let text = ''
  try {
    text = await res.text()
  } catch {
    text = ''
  }
  let data = {}
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = {}
    }
  }

  if (!res.ok) {
    if (res.status === 401 && !path.startsWith('/auth/login')) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
    }
    const err = new Error(data.error || `请求失败 (${res.status})`)
    err.status = res.status
    throw err
  }
  return data
}

function qs(params = {}) {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') search.set(k, v)
  })
  const s = search.toString()
  return s ? '?' + s : ''
}

export const api = {
  // ---------- 认证 ----------
  login: (account, password) =>
    request('/auth/login', { method: 'POST', body: JSON.stringify({ account, password }) }),
  register: (email, password) =>
    request('/auth/register', { method: 'POST', body: JSON.stringify({ email, password }) }),

  // ---------- 站点设置（公开读取 / 管理员保存） ----------
  getSettings: () => request('/settings'),
  getStats: () => request('/stats'), // 全站统计：文章数 / 分类数 / 标签数
  saveHomeBg: (homeBg) => request('/admin/settings/home', { method: 'PUT', body: JSON.stringify({ homeBg }) }),
  // 页面管理：文章详情页背景图（与首页背景图是两个独立设置）
  savePostBg: (postBg) => request('/admin/settings/postbg', { method: 'PUT', body: JSON.stringify({ postBg }) }),
  saveNotice: (notice) => request('/admin/settings/notice', { method: 'PUT', body: JSON.stringify({ notice }) }),
  // 页面管理：首页文案（昵称 / 介绍 / 打字动画）+ 首屏头像 + 登录页背景
  // 参数是对象，只传要改的字段：{nickname, desc, roles, avatar, loginBg}
  saveContent: (payload) => request('/admin/settings/content', { method: 'PUT', body: JSON.stringify(payload) }),
  savePageSize: (pageSize) => request('/admin/settings/pagesize', { method: 'PUT', body: JSON.stringify({ pageSize }) }),
  saveTheme: (theme) => request('/admin/settings/theme', { method: 'PUT', body: JSON.stringify(theme) }),
  // 功能开关：整个启用 / 关闭 Momo Talk（私信 + 好友）
  saveMomoTalk: (enabled) => request('/admin/settings/momotalk', { method: 'PUT', body: JSON.stringify({ enabled }) }),
  // 页面管理：切换全站字体（system | rouhei | round）
  saveFont: (siteFont) => request('/admin/settings/font', { method: 'PUT', body: JSON.stringify({ siteFont }) }),
  // 页面管理：点击特效（ba-click-fx）——整体保存一份参数对象
  saveClickFx: (clickFx) => request('/admin/settings/clickfx', { method: 'PUT', body: JSON.stringify(clickFx) }),

  // 后台首次进入的初始化引导（上传头像 + 填邮箱，做一次就不再出现）
  getOnboarding: () => request('/admin/onboarding'),
  completeOnboarding: (email) => request('/admin/onboarding', { method: 'POST', body: JSON.stringify({ email }) }),

  // ---------- 用户中心（需登录） ----------
  getMe: () => request('/user/me'),
  // 昵称 / 主页封面：两个字段都可选，只传要改的那个
  updateProfile: (payload) =>
    request('/user/profile', {
      method: 'PUT',
      body: JSON.stringify(typeof payload === 'string' ? { nickname: payload } : payload)
    }),
  changePassword: (oldPassword, newPassword) =>
    request('/user/password', { method: 'PUT', body: JSON.stringify({ oldPassword, newPassword }) }),
  uploadAvatar: (file) => {
    const fd = new FormData()
    fd.append('avatar', file)
    return request('/user/avatar', { method: 'POST', body: fd })
  },
  listNotifications: (params) => request('/user/notifications' + qs(params)),
  markNotificationsRead: (ids) =>
    request('/user/notifications/read', { method: 'POST', body: JSON.stringify({ ids: ids || [] }) }),

  // ---------- 公开：文章 ----------
  // params 支持 page / pageSize / category / tag / q（后三个可叠加，口径与 /search 一致）
  listPosts: (params) => request('/posts' + qs(params)),
  listCategories: () => request('/categories'), // 分类列表 + 每类文章数（分类器用）
  listTags: () => request('/tags'), // 标签列表 + 每个标签的文章数（搜索页标签云）
  getPost: (id) => request(`/posts/${id}`),
  toggleLike: (id) => request(`/posts/${id}/like`, { method: 'POST' }),

  // ---------- 说说（类似 Twitter 的短动态，与文章分开） ----------
  // 读和点赞是公开的（匿名访客也能赞，按 blog_visitor Cookie 去重）；
  // 发布 / 删除 / 上传配图需要登录。
  listMoments: (params) => request('/moments' + qs(params)),
  likeMoment: (id) => request(`/moments/${id}/like`, { method: 'POST' }),
  viewMoment: (id) => request(`/moments/${id}/view`, { method: 'POST' }),
  createMoment: (data) => request('/user/moments', { method: 'POST', body: JSON.stringify(data) }),
  deleteMoment: (id) => request(`/user/moments/${id}`, { method: 'DELETE' }),
  // 说说评论：与文章评论同一套两级结构，前端复用同一个 CommentSection 组件
  listMomentComments: (id) => request(`/moments/${id}/comments`),
  createMomentComment: (id, data) =>
    request(`/moments/${id}/comments`, { method: 'POST', body: JSON.stringify(data) }),

  // ---------- 个人主页（公开） ----------
  // 基本信息 + 三个统计数字 + 他最近的说说。不含邮箱。
  getUserProfile: (id) => request(`/users/${id}/profile`),
  uploadMomentImage: (file) => {
    const fd = new FormData()
    fd.append('file', file)
    return request('/user/upload', { method: 'POST', body: fd })
  },

  // ---------- 公开：站内搜索 ----------
  // 一次拿到「文章 / 分类 / 标签 / 用户」四类结果（每类最多 limit 条，默认 6、上限 30）。
  // 未登录时后端不返回用户（响应里 usersLocked=true），其它三类照常。
  search: (q, params) => request('/search' + qs({ q, ...params })),

  // ---------- 评论 / 回复（发表需登录） ----------
  listComments: (postId) => request(`/posts/${postId}/comments`),
  createComment: (postId, data) =>
    request(`/posts/${postId}/comments`, { method: 'POST', body: JSON.stringify(data) }),

  // ---------- 管理：文章 ----------
  adminListPosts: (params) => request('/admin/posts' + qs(params)),
  adminGetPost: (id) => request(`/admin/posts/${id}`),
  createPost: (data) => request('/admin/posts', { method: 'POST', body: JSON.stringify(data) }),
  updatePost: (id, data) => request(`/admin/posts/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePost: (id) => request(`/admin/posts/${id}`, { method: 'DELETE' }),

  // ---------- 管理：评论 ----------
  adminListComments: (params) => request('/admin/comments' + qs(params)),
  adminDeleteComment: (id) => request(`/admin/comments/${id}`, { method: 'DELETE' }),

  // ---------- 管理：用户 ----------
  adminListUsers: (params) => request('/admin/users' + qs(params)),
  adminDeleteUser: (id) => request(`/admin/users/${id}`, { method: 'DELETE' }),

  // ---------- 管理：图片上传（正文/封面） ----------
  uploadImage: (file) => {
    const fd = new FormData()
    fd.append('file', file)
    return request('/admin/upload', { method: 'POST', body: fd })
  },

  // ---------- 管理：备份 / 迁移 ----------
  // 当前数据概览（各表行数 / 正文与上传的文件数与体积）
  backupInfo: () => request('/admin/backup/info'),
  /**
   * 下载备份的地址。用 <a download> 触发浏览器原生下载，
   * 不走 fetch —— 备份可能几百 MB，塞进内存再存盘是自找麻烦。
   * parts：'db,posts,uploads' 的任意组合
   */
  backupExportUrl: (parts) => BASE + '/admin/backup/export?parts=' + encodeURIComponent(parts),
  /**
   * 上传备份并恢复（**会清空并整体替换现有数据**）。
   * @param {File} file 备份 zip
   * @param {string} confirm 必须精确等于 REPLACE
   */
  importBackup: (file, confirm) => {
    const fd = new FormData()
    fd.append('file', file)
    fd.append('confirm', confirm)
    return request('/admin/backup/import', { method: 'POST', body: fd })
  },

  // ============================================================
  // 【桃信】私信 / 好友系统
  // ============================================================

  // 消息按钮小红点：未读私信 / 待处理好友申请 / 未读回复，三项汇总
  messengerUnread: () => request('/messenger/unread'),

  // 按 9 位桃信 ID 或昵称搜索用户
  searchUsers: (q) => request('/users/search' + qs({ q })),
  // 某个用户的桃信资料（:id 可以是 9 位 ID 或内部 ID）
  getUser: (id) => request(`/users/${encodeURIComponent(id)}`),

  // 好友
  listFriends: () => request('/friends'),
  listFriendRequests: () => request('/friends/requests'),
  requestFriend: (userId) =>
    request('/friends/request', { method: 'POST', body: JSON.stringify({ userId: String(userId) }) }),
  acceptFriend: (relationId) => request(`/friends/${relationId}/accept`, { method: 'POST' }),
  rejectFriend: (relationId) => request(`/friends/${relationId}/reject`, { method: 'POST' }),
  removeFriend: (userId) => request(`/friends/${userId}`, { method: 'DELETE' }),

  // 会话 / 私信
  listConversations: () => request('/conversations'),
  /** 打开（没有则创建）与某人的会话；peerId 传 9 位桃信 ID */
  openConversation: (peerId) =>
    request('/conversations', { method: 'POST', body: JSON.stringify({ peerId: String(peerId) }) }),
  listMessages: (convId, params) => request(`/conversations/${convId}/messages` + qs(params)),
  sendMessage: (convId, content) =>
    request(`/conversations/${convId}/messages`, { method: 'POST', body: JSON.stringify({ content }) }),
  readConversation: (convId) => request(`/conversations/${convId}/read`, { method: 'POST' }),
  /** 「我正在输入」——瞬时状态，只写 Redis（不落库、不产生历史记录） */
  sendTyping: (convId) => request(`/conversations/${convId}/typing`, { method: 'POST' })
}

/**
 * 桃信实时事件流（SSE）地址。
 * EventSource 不能自定义请求头，所以 token 走查询参数（后端 events.go 支持）。
 * 返回空串表示未登录，调用方应跳过连接。
 */
export function eventStreamUrl() {
  const token = localStorage.getItem('token')
  if (!token) return ''
  return BASE + '/user/events?token=' + encodeURIComponent(token)
}
