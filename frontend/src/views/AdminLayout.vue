<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import { refreshAdminSetup } from '../composables/useAdminSetup'

// 侧栏图标：用 CSS mask + currentColor 上色，
// 所以图标会自动跟随「默认 / 悬停 / 选中」的文字颜色，不用准备两套图。
// 以「原始 SVG 文本」导入，并把写死的 stroke="#333" 换成 currentColor：
// 这样图标颜色会跟着链接的 color 走（默认白 / 悬停白 / 选中蓝），一套图就够。
// 不用 CSS mask 是因为小 SVG 会被 Vite 内联成 data URI，塞进 CSS 变量容易出转义问题。
import editorRaw from '../others/editor.svg?raw'
import commentsRaw from '../others/comments.svg?raw'
import usersRaw from '../others/every-user.svg?raw'
import pageRaw from '../others/page.svg?raw'
import configRaw from '../others/config.svg?raw'
import serverRaw from '../others/server.svg?raw'

// 图标里除了描边，还有实心小圆点（server.svg 的指示灯）用的是 fill="#333"，
// 只换 stroke 的话这几个点会一直是黑的 —— 两种都换成 currentColor。
const toIcon = (raw) =>
  raw
    .replace(/stroke="#333"/g, 'stroke="currentColor"')
    .replace(/fill="#333"/g, 'fill="currentColor"')

const router = useRouter()
const { displayName, avatarUrl, user, logout } = useAuth()
const { toast } = useToast()

// 首次进后台：还没做初始化（头像 + 邮箱）就先领到引导页，做完才放行。
// 判断结果在一次页面加载里只会请求一次（见 useAdminSetup 的缓存）。
onMounted(async () => {
  const ready = await refreshAdminSetup()
  if (!ready) router.replace('/admin/setup')
})

const navItems = [
  { to: '/admin', exact: true, label: '文章管理', icon: toIcon(editorRaw) },
  { to: '/admin/comments', label: '评论管理', icon: toIcon(commentsRaw) },
  { to: '/admin/users', label: '用户管理', icon: toIcon(usersRaw) },
  { to: '/admin/settings', label: '页面管理', icon: toIcon(pageRaw) },
  { to: '/admin/account', label: '设置', icon: toIcon(configRaw) },
  { to: '/admin/backup', label: '备份迁移', icon: toIcon(serverRaw) }
]

function doLogout() {
  logout()
  toast.info('已退出登录')
  router.push('/admin/login')
}
</script>

<template>
  <div class="admin">
    <!-- 缩略侧栏：默认只显示图标，鼠标悬停时展开为完整菜单 -->
    <aside class="admin-side">
      <div class="admin-brand">
        <span class="label">网站后台</span>
      </div>
      <nav class="admin-nav">
        <router-link
          v-for="it in navItems"
          :key="it.to"
          :to="it.to"
          :exact-active-class="it.exact ? 'active' : undefined"
          active-class="active"
          :title="it.label"
        >
          <span class="ico" v-html="it.icon"></span>
          <span class="label">{{ it.label }}</span>
        </router-link>
      </nav>
      <div class="admin-user">
        <span class="avatar admin-avatar" style="--avatar-size: 40px">
          <img v-if="avatarUrl" :src="avatarUrl" alt="" />
        </span>
        <div class="admin-user-id label">
          <div class="admin-name">{{ displayName }}<i class="badge-admin">站长</i></div>
          <div class="admin-email">{{ user?.email }}</div>
          <button class="admin-logout" @click="doLogout">退出登录</button>
        </div>
      </div>
    </aside>

    <main class="admin-main">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.admin {
  display: flex;
  min-height: 100svh;
  background: var(--bg);
}

/* ============ 缩略侧栏 ============
   默认 68px（只有图标），鼠标悬停整条展开到 220px。
   纯 CSS 实现（:hover + transition），不需要 JS 监听。 */
.admin-side {
  width: 68px;
  flex-shrink: 0;
  /* 侧边栏用站点主题色做底，上面的文字全部走白色系 */
  background: var(--accent);
  border-right: 0;              /* 去掉那条细黑边，缩略时和内容区无缝衔接 */
  display: flex;
  flex-direction: column;
  padding: 20px 0;
  position: sticky;
  top: 0;
  height: 100svh;
  color: #fff;
  overflow: hidden;
  transition: width 0.2s ease;
  will-change: width;
}
.admin-side:hover { width: 220px; }

/* 展开时才出现的文字：默认透明，悬停淡入 */
.admin-side .label {
  opacity: 0;
  white-space: nowrap;
  transition: opacity 0.14s ease;
  pointer-events: none;
}
.admin-side:hover .label { opacity: 1; pointer-events: auto; }

.admin-brand {
  font-weight: 700;
  font-size: 16px;
  color: #fff;
  padding: 0 22px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.22);
  min-height: 20px;
}

.admin-nav { display: flex; flex-direction: column; padding: 16px 12px; gap: 4px; flex: 1; }
/* 固定 44px 高 + 44px 宽（68 - 12×2）：缩略时每项正好是个正方形 */
.admin-nav a {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  height: 44px;
  box-sizing: border-box;
  padding: 0 12px;
  border-radius: 10px;
  color: rgba(255, 255, 255, 0.82);
  font-size: 14.5px;
  transition: background 0.18s ease, color 0.18s ease;
}

/* 图标容器：SVG 已经用 currentColor 描边，颜色自动跟随链接状态 */
.admin-nav .ico {
  width: 20px;
  height: 20px;
  flex: none;
  display: block;
}
.admin-nav .ico :deep(svg) { width: 100%; height: 100%; display: block; }

/* 悬停：淡白色蒙层 */
.admin-nav a:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.16);
  text-decoration: none;
}
/* 选中：白底蓝字（图标同样跟着变蓝） */
.admin-nav a.active {
  color: var(--accent);
  background: #fff;
  font-weight: 600;
}

/* 底部：缩略时只留头像并水平居中，展开时恢复成「头像 + 昵称/邮箱/退出」 */
.admin-user {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: center;
  padding: 16px 0 0;
  border-top: 1px solid rgba(255, 255, 255, 0.22);
  margin: 0 12px;
  font-size: 14px;
  color: #fff;
}
/* 缩略态：文字块直接从布局里拿掉（只透明的话仍占宽度，头像会被挤到左边） */
.admin-side:not(:hover) .admin-user { margin: 0; }
.admin-side:not(:hover) .admin-user-id { display: none; }
.admin-side:hover .admin-user {
  justify-content: flex-start;
  padding: 16px 10px 0;
}
.admin-avatar { border: 2px solid rgba(255, 255, 255, 0.6); flex-shrink: 0; }
.admin-user-id { min-width: 0; }
.admin-name { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.admin-email {
  font-size: 11.5px;
  color: rgba(255, 255, 255, 0.78);
  overflow: hidden;
  text-overflow: ellipsis;
}
.admin-logout {
  background: none;
  border: none;
  color: rgba(255, 255, 255, 0.82);
  font-size: 12.5px;
  cursor: pointer;
  padding: 0;
  margin-top: 2px;
}
.admin-logout:hover { color: #fff; text-decoration: underline; }

.admin-main {
  flex: 1;
  min-width: 0;
  padding: 32px;
}

@media (max-width: 760px) {
  .admin { flex-direction: column; }
  .admin-side,
  .admin-side:hover {
    width: 100%;
    height: auto;
    position: static;
    flex-direction: row;
    align-items: center;
    padding: 12px 16px;
    gap: 12px;
    overflow-x: auto;
  }
  .admin-brand { border-bottom: none; padding: 0; }
  .admin-brand .label { opacity: 1; pointer-events: auto; }
  .admin-nav { flex-direction: row; padding: 0; gap: 8px; flex: 1; overflow-x: auto; }
  .admin-nav a { padding: 6px 10px; font-size: 13px; }
  /* 窄屏放不下文字，只留图标 */
  .admin-nav a .label { display: none; }
  .admin-user { border-top: none; margin: 0; padding: 0; }
  .admin-user-id { display: none; }
  .admin-main { padding: 20px 16px; }
}
</style>
