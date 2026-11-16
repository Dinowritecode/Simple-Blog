<script setup>
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import NavBar from './components/NavBar.vue'
import SiteFooter from './components/SiteFooter.vue'
import ToastHost from './components/ToastHost.vue'
import MessengerWidget from './components/messenger/MessengerWidget.vue'
import { useAuth } from './composables/useAuth'
import { useSite } from './composables/useSite'
import { useClickFx } from './composables/useClickFx'

const route = useRoute()
const showChrome = () => !route.meta.hideChrome
const { init } = useAuth()

// 在根组件拉一次站点设置：这样「外观定制」（主题色 / 透明度）在任何页面都会生效，
// 包括登录页这种没有自己调用 useSite 的页面。
useSite()

// 全站点击特效（ba-click-fx）：后台可开关 + 调参；关掉时连库都不会加载
useClickFx()

// 启动时校验登录态并刷新用户信息（无效 token 会自动登出）
onMounted(init)
</script>

<template>
  <NavBar v-if="showChrome()" />
  <router-view />
  <SiteFooter v-if="showChrome()" />
  <ToastHost />
  <!-- 桃信：右下角消息小圆钮 + 私信小窗（登录后才出现，不是独立路由页面） -->
  <MessengerWidget v-if="showChrome()" />
</template>
