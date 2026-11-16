// ============================================================
// useUi.js —— 跨组件 UI 状态
// 目前：资料编辑弹窗（改昵称/头像）的开关。
// 注册成功后可能由「登录注册页」触发打开头像上传弹窗，
// 而弹窗本体在导航栏的 UserMenu 里，所以用模块级状态共享。
// ============================================================

import { reactive } from 'vue'

const ui = reactive({
  profileOpen: false
})

export function useUi() {
  function openProfileEditor() {
    ui.profileOpen = true
  }
  function closeProfileEditor() {
    ui.profileOpen = false
  }
  return { ui, openProfileEditor, closeProfileEditor }
}
