// ============================================================
// useToast.js —— 轻量全局提示（页面顶部小提示条）
// 用法：
//   const { toast } = useToast()
//   toast.success('注册成功！', { actionLabel: '去上传头像', onAction: openProfile })
//   toast.error('出错了')
// 组件 <ToastHost /> 挂在 App.vue 上负责渲染。
// ============================================================

import { reactive } from 'vue'

const items = reactive([])
let seq = 0

function remove(id) {
  const i = items.findIndex((t) => t.id === id)
  if (i > -1) items.splice(i, 1)
}

function push(text, opts = {}) {
  const { type = 'success', actionLabel = '', onAction = null, duration = 5000 } = opts
  const id = ++seq
  items.push({ id, text, type, actionLabel, onAction })
  if (duration) setTimeout(() => remove(id), duration)
  return id
}

export function useToast() {
  return {
    items,
    remove,
    toast: {
      success: (text, opts) => push(text, { ...opts, type: 'success' }),
      error: (text, opts) => push(text, { ...opts, type: 'error' }),
      info: (text, opts) => push(text, { ...opts, type: 'info' })
    }
  }
}
