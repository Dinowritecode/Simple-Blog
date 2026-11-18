// ============================================================
// useAdminSetup.js —— 后台「首次初始化」状态
//
// 一个页面加载里最多请求一次 /api/admin/onboarding，结果缓存在模块级 Promise 里。
// AdminLayout 与 AdminSetupView 都要判断，没必要各拉一遍。
// ============================================================

import { ref } from 'vue'
import { api } from '../api'

const done = ref(null) // null = 还没查过；true/false = 已查过
let inflight = null

/** 查询是否已完成初始化；force=true 时强制重新请求 */
export function refreshAdminSetup(force = false) {
  if (!force && inflight) return inflight
  inflight = api
    .getOnboarding()
    .then((d) => {
      done.value = !!d.done
      return done.value
    })
    .catch(() => {
      // 查不到就当作「已初始化」：不能让引导页因为接口抖动把站长挡在后台外面
      done.value = true
      return true
    })
  return inflight
}

/** 完成初始化后调用：清掉缓存，下次进后台重新判断 */
export function resetAdminSetupCache() {
  inflight = null
  done.value = null
}

export function useAdminSetup() {
  return { done, refreshAdminSetup, resetAdminSetupCache }
}
