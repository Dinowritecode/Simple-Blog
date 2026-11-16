<script setup>
// ============================================================
// MessengerWidget.vue —— 桃信入口（挂在 App.vue 里，全站可用）
//
// 只做两件事：
//   ① 建立 SSE 长连接：登录后立刻连上 /api/user/events，
//      新私信 / 新好友申请 / 新回复都会实时把红点刷新出来
//   ② 打开时渲染 <MessengerWindow>（主页上的小窗口，不是独立页面）
//
// ★ 右下角的悬浮小圆钮已经按需求删掉：桃信改由导航栏右侧的「桃信」
//   图标按钮（UserMenu）与评论区的「私信」按钮打开，页面上不再有浮起的小圆钮。
// ============================================================
import { onBeforeUnmount, onMounted, watch } from 'vue'
import { useAuth } from '../../composables/useAuth'
import { useMessenger } from '../../composables/useMessenger'
import { useSite } from '../../composables/useSite'
import MessengerWindow from './MessengerWindow.vue'

const { isLoggedIn } = useAuth()
const { stage, refreshUnread, connectStream, disconnectStream, reset } = useMessenger()
// 后台可以整个关掉 Momo Talk：关掉后不建连接、不渲染小窗
const { momoTalkEnabled } = useSite()

let pollTimer = null

function start() {
  if (!momoTalkEnabled.value) return
  connectStream()
  refreshUnread()
  // SSE 万一被代理掐断，这个轮询兜底保证红点不会一直不刷新
  clearInterval(pollTimer)
  pollTimer = setInterval(refreshUnread, 45000)
}

function stop() {
  clearInterval(pollTimer)
  pollTimer = null
  disconnectStream()
  reset()
}

onMounted(() => {
  if (isLoggedIn.value) start()
})

onBeforeUnmount(stop)

// 登录状态变化：登录后连接、退出后清空
watch(isLoggedIn, (v) => {
  if (v) start()
  else stop()
})

// 后台把开关关掉时，立刻断开连接并收起小窗
watch(momoTalkEnabled, (on) => {
  if (!on) stop()
  else if (isLoggedIn.value) start()
})
</script>

<template>
  <!-- 功能开关关闭时整块不渲染（连 SSE 长连接都不会建立） -->
  <div v-if="isLoggedIn && momoTalkEnabled" class="mz-widget">
    <!-- 右下角的悬浮小圆钮已删除：入口在导航栏的「Momo Talk」按钮上 -->
    <MessengerWindow v-if="stage !== 'closed'" />
  </div>
</template>

<style scoped>
/* 只留一个定位层：小窗自己用 fixed 定位，这里是它的 z-index 上下文。
   之前的 .mz-fab（右下角小圆钮）相关样式已随按钮一起删除。 */
.mz-widget { position: relative; z-index: 1200; }
</style>
