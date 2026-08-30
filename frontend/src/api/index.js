const BASE = '/api'

/**
 * 统一请求封装：自动携带 JWT、解析 JSON、抛出带状态码的错误
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

  const data = await res.json().catch(() => ({}))

  if (!res.ok) {
    // 登录过期：清除本地凭证
    if (res.status === 401 && path.startsWith('/admin')) {
      localStorage.removeItem('token')
      localStorage.removeItem('username')
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
  // 认证
  login: (username, password) =>
    request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }),

  // 公开：文章
  listPosts: (params) => request('/posts' + qs(params)),
  getPost: (id) => request(`/posts/${id}`),
  likePost: (id) => request(`/posts/${id}/like`, { method: 'POST' }),

  // 管理：文章
  adminListPosts: (params) => request('/admin/posts' + qs(params)),
  adminGetPost: (id) => request(`/admin/posts/${id}`),
  createPost: (data) => request('/admin/posts', { method: 'POST', body: JSON.stringify(data) }),
  updatePost: (id, data) => request(`/admin/posts/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePost: (id) => request(`/admin/posts/${id}`, { method: 'DELETE' }),

  // 管理：图片上传
  uploadImage: (file) => {
    const fd = new FormData()
    fd.append('file', file)
    return request('/admin/upload', { method: 'POST', body: fd })
  }
}
