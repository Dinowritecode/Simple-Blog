import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import reveal from './directives/reveal'
import './style.css'

// 关掉浏览器自己的「刷新后恢复滚动位置」。
// 开着的话，刷新时浏览器会在 Vue 把内容渲染完之前就按旧位置滚一下，
// 结果打开首页直接停在半中间（首屏整个被跳过）。站内的前进/后退仍由
// router 的 scrollBehavior 用 savedPosition 还原，不受影响。
if ('scrollRestoration' in history) history.scrollRestoration = 'manual'

createApp(App).use(router).directive('reveal', reveal).mount('#app')
