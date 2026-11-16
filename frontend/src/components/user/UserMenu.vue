<script setup>
// 右上角用户区：
//  - 铃铛（回复通知，未读红点，点开面板可跳转对应文章）
//  - 头像（悬停弹出小菜单：个人资料 / 修改密码 / 退出登录）
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../api'
import { useAuth } from '../../composables/useAuth'
import { useUi } from '../../composables/useUi'
import { useToast } from '../../composables/useToast'
import { useMessenger } from '../../composables/useMessenger'
import { useSite } from '../../composables/useSite'
import logoUrl from '../../others/icon.svg'
import ChangePasswordModal from './ChangePasswordModal.vue'
import ProfileModal from './ProfileModal.vue'

const router = useRouter()
const { user, displayName, avatarUrl, isAdmin, logout } = useAuth()
const { ui, openProfileEditor } = useUi()
const { toast } = useToast()
// Momo Talk：消息按钮的红点（私信 + 好友申请）与小窗开关
const { totalUnread, chatUnread, friendRequestCount, refreshUnread, open: openMessenger } = useMessenger()
// 功能开关：后台可以整个关掉 Momo Talk（关掉后导航栏按钮与菜单项都不渲染）
const { momoTalkEnabled } = useSite()

// ---------- 头像悬停菜单 ----------
const menuOpen = ref(false)
let menuTimer = null
function enterMenu() { clearTimeout(menuTimer); menuOpen.value = true }
function leaveMenu() { menuTimer = setTimeout(() => (menuOpen.value = false), 160) }

const passOpen = ref(false)

// 头像点击 → 自己的个人主页（悬停仍然展开菜单，两件事互不冲突）
function goProfile() {
  menuOpen.value = false
  if (user.value && user.value.id) router.push(`/u/${user.value.id}`)
}

function doLogout() {
  logout()
  menuOpen.value = false
  router.push('/')
  toast.info('已退出登录')
}

// ---------- 通知 ----------
const bellOpen = ref(false)
const notifs = ref([])
const notifTotal = ref(0)
const unread = ref(0)
const loadingNotifs = ref(false)
let timer = null

async function refreshBadge() {
  if (!unreadTimerAlive()) return
  try {
    const d = await api.listNotifications({ page: 1, pageSize: 1 })
    unread.value = d.unreadCount || 0
    notifTotal.value = d.total || 0
  } catch { /* 忽略 */ }
}
let alive = false
function unreadTimerAlive() { return alive }

async function toggleBell() {
  bellOpen.value = !bellOpen.value
  if (bellOpen.value) await loadNotifs()
}

async function loadNotifs() {
  loadingNotifs.value = true
  try {
    const d = await api.listNotifications({ page: 1, pageSize: 10 })
    notifs.value = d.items || []
    unread.value = d.unreadCount || 0
    notifTotal.value = d.total || 0
    if (unread.value > 0) {
      // 打开即视为已读（后端标记，角标清零）
      await api.markNotificationsRead([]).catch(() => {})
      unread.value = 0
    }
  } catch {
    toast.error('通知加载失败')
  } finally {
    loadingNotifs.value = false
  }
}

function gotoNotif(n) {
  bellOpen.value = false
  const slug = n.postSlug || n.postId // 优先用随机短码（文章删除后 postSlug 为空则回退 id，会 404 由页面处理）
  router.push({ path: `/blog/${slug}`, query: { c: n.commentId || '' } })
}

onMounted(() => {
  alive = true
  refreshBadge()
  refreshUnread() // 桃信角标（私信未读 + 好友申请）
  timer = setInterval(refreshBadge, 30000) // 30 秒轮询一次未读数
})
onBeforeUnmount(() => {
  alive = false
  clearInterval(timer)
})
</script>

<template>
  <div class="user-area">
    <!-- Momo Talk：入口按钮（私信 / 好友申请的红点提示）。点击在主页上打开私信小窗。
         图标用项目自带的 icon.svg（白色桃子剪影），mask 取形状、底色用 currentColor。 -->
    <button
      v-if="momoTalkEnabled"
      class="icon-btn mz-btn"
      :class="{ 'mz-has': totalUnread > 0 }"
      aria-label="Momo Talk 私信"
      :title="totalUnread > 0 ? `Momo Talk：${totalUnread} 条未读` : 'Momo Talk 私信'"
      @click="openMessenger()"
    >
      <span class="mz-btn-logo" :style="{ '--mz-logo': `url(${logoUrl})` }" aria-hidden="true"></span>
      <span v-if="totalUnread > 0" class="bell-dot">{{ totalUnread > 99 ? '99+' : totalUnread }}</span>
    </button>

    <!-- 通知铃铛 -->
    <div class="bell-wrap">
      <button class="icon-btn bell-btn" :class="{ active: bellOpen }" aria-label="通知" title="回复通知" @click="toggleBell">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </svg>
        <span v-if="unread > 0" class="bell-dot">{{ unread > 99 ? '99+' : unread }}</span>
      </button>

      <transition name="pop">
        <div v-if="bellOpen" class="panel notif-panel">
          <div class="panel-head">回复通知 <span v-if="notifTotal" class="panel-count">{{ notifTotal }}</span></div>
          <div v-if="loadingNotifs" class="panel-empty">加载中…</div>
          <div v-else-if="!notifs.length" class="panel-empty">暂无通知。有人回复你的评论时会显示在这里</div>
          <div v-else class="notif-list">
            <button v-for="n in notifs" :key="n.id" class="notif-item" :class="{ unread: !n.isRead }" @click="gotoNotif(n)">
              <span class="avatar notif-avatar" style="--avatar-size: 32px">
                <img v-if="n.actor && n.actor.avatar" :src="n.actor.avatar" alt="" />
                <span v-else class="avatar-letter">{{ (n.actor?.nickname || '?')[0] }}</span>
              </span>
              <span class="notif-body">
                <span class="notif-text">{{ n.content }}</span>
                <span class="notif-time">{{ n.createdAt.slice(5, 16) }}</span>
              </span>
            </button>
          </div>
        </div>
      </transition>
    </div>

    <!-- 头像 + 悬停菜单 -->
    <div class="me-wrap" @mouseenter="enterMenu" @mouseleave="leaveMenu">
      <button class="avatar me-avatar" style="--avatar-size: 34px" aria-label="我的主页" title="我的主页" @click="goProfile">
        <img v-if="avatarUrl" :src="avatarUrl" alt="头像" />
        <span v-else class="avatar-letter">{{ (displayName || '?')[0] }}</span>
      </button>

      <transition name="pop">
        <div v-if="menuOpen" class="panel me-panel" @mouseenter="enterMenu" @mouseleave="leaveMenu">
          <div class="me-head">
            <span class="avatar" style="--avatar-size: 40px">
              <img v-if="avatarUrl" :src="avatarUrl" alt="" />
              <span v-else class="avatar-letter">{{ (displayName || '?')[0] }}</span>
            </span>
            <span class="me-id">
              <span class="me-name">{{ displayName }}<i v-if="isAdmin" class="badge-admin">站长</i></span>
              <span class="me-email">{{ user?.email }}</span>
              <span class="me-uid" v-if="momoTalkEnabled">
                Momo Talk ID：{{ user?.userId || '——' }}
                <em v-if="totalUnread > 0" class="me-uid-dot">{{ totalUnread }}</em>
              </span>
            </span>
          </div>
          <div class="menu-list">
            <button class="menu-item" @click="goProfile">
              <span>🏠</span> 我的主页
            </button>
            <button v-if="momoTalkEnabled" class="menu-item" @click="menuOpen = false; openMessenger()">
              <span>💗</span> Momo Talk 私信
              <em v-if="chatUnread > 0" class="menu-badge">{{ chatUnread }}</em>
            </button>
            <button v-if="momoTalkEnabled" class="menu-item" @click="menuOpen = false; openMessenger({ tab: 'friends' })">
              <span>👥</span> 新的朋友
              <em v-if="friendRequestCount > 0" class="menu-badge">{{ friendRequestCount }}</em>
            </button>
            <button class="menu-item" @click="menuOpen = false; openProfileEditor()">
              <span>👤</span> 个人资料（昵称 / 头像）
            </button>
            <button class="menu-item" @click="menuOpen = false; passOpen = true">
              <span>🔑</span> 修改密码
            </button>
            <!-- 站长快捷管理（外来注册用户 / 评论都在这里管理） -->
            <template v-if="isAdmin">
              <div class="menu-divider">站长管理</div>
              <button class="menu-item" @click="menuOpen = false; router.push('/admin')">
                <span>📋</span> 文章管理
              </button>
              <button class="menu-item" @click="menuOpen = false; router.push('/admin/comments')">
                <span>💬</span> 评论管理
              </button>
              <button class="menu-item" @click="menuOpen = false; router.push('/admin/users')">
                <span>👥</span> 用户管理
              </button>
            </template>
            <div class="menu-divider"></div>
            <button class="menu-item danger" @click="doLogout">
              <span>↪️</span> 退出登录
            </button>
          </div>
        </div>
      </transition>
    </div>

    <!-- 弹窗 -->
    <ChangePasswordModal :open="passOpen" @close="passOpen = false" />
    <ProfileModal :open="ui.profileOpen" @close="ui.profileOpen = false" />
  </div>
</template>

<style scoped>
/* 铃铛 + 头像：间距与导航栏其它按钮保持一致（.nav-actions 的 gap 也是 8px） */
.user-area { display: flex; align-items: center; gap: 8px; position: relative; }

/* 铃铛 */
.bell-wrap { position: relative; }
.bell-btn.active { color: var(--accent-ink); border-color: var(--accent-ink); }

/* Momo Talk 按钮：有未读时染成粉色，让「消息按钮」自带红点提示。
   图标是白色桃子剪影（icon.svg），这里用 mask 把它当成「形状」、底色取 currentColor：
   于是它和铃铛/月亮这些描边图标永远是同一个颜色，深浅主题 / hover / 未读变粉
   全部自动跟随 —— 不需要任何 filter 染色（滤镜只能得到近似色，还会在夜间模式下跑偏）。 */
.mz-btn { position: relative; }
.mz-btn.mz-has { color: #ff5a8a; }
.mz-btn.mz-has:hover { color: #ff5a8a; }
.mz-btn-logo {
  width: 18px;
  height: 18px;
  display: block;
  background-color: currentColor;
  -webkit-mask: var(--mz-logo) no-repeat center / contain;
  mask: var(--mz-logo) no-repeat center / contain;
}
.bell-dot {
  position: absolute;
  top: -4px;
  right: -4px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 999px;
  background: #e5484d;
  color: #fff;
  font-size: 10.5px;
  line-height: 16px;
  text-align: center;
  font-family: var(--font-mono);
}

/* 头像 */
.me-wrap { position: relative; display: flex; }
.me-avatar { cursor: pointer; border: 2px solid var(--border); transition: border-color 0.2s ease; }
.me-wrap:hover .me-avatar { border-color: var(--accent); }

/* 下拉面板（通用） */
.panel {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: var(--shadow);
  z-index: 500;
  overflow: hidden;
}

/* 通知面板 */
.notif-panel { width: 320px; }
.panel-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  font-size: 13.5px;
  font-weight: 700;
  border-bottom: 1px solid var(--border);
}
.panel-count {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--text-dim);
  background: var(--bg-alt);
  padding: 1px 7px;
  border-radius: 999px;
}
.panel-empty { padding: 28px 16px; text-align: center; color: var(--text-dim); font-size: 13px; line-height: 1.7; }
.notif-list { max-height: 360px; overflow-y: auto; }
.notif-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  width: 100%;
  text-align: left;
  border: none;
  background: none;
  padding: 11px 14px;
  cursor: pointer;
  border-bottom: 1px solid var(--border);
}
.notif-item:last-child { border-bottom: none; }
.notif-item:hover { background: var(--bg-alt); }
.notif-item.unread { background: var(--accent-soft); }
.notif-avatar { flex-shrink: 0; margin-top: 2px; }
.notif-body { min-width: 0; display: flex; flex-direction: column; gap: 3px; }
.notif-text { font-size: 13px; color: var(--text); line-height: 1.5; word-break: break-word; }
.notif-time { font-family: var(--font-mono); font-size: 11.5px; color: var(--text-dim); }

/* 我的菜单 */
.me-panel { width: 260px; }
.me-head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px;
  border-bottom: 1px solid var(--border);
}
.me-id { display: flex; flex-direction: column; min-width: 0; }
.me-name { font-weight: 700; font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.me-email { font-size: 12px; color: var(--text-dim); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
/* 桃信 9 位 ID（别人靠它搜到你） */
.me-uid {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 2px;
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: #ff7fa8;
}
.me-uid-dot {
  font-style: normal;
  background: #e5484d;
  color: #fff;
  border-radius: 999px;
  padding: 0 5px;
  font-size: 10px;
  line-height: 14px;
}
/* 菜单项右侧的小计数（桃信未读 / 好友申请） */
.menu-badge {
  margin-left: auto;
  font-style: normal;
  background: #e5484d;
  color: #fff;
  border-radius: 999px;
  padding: 0 6px;
  font-size: 10.5px;
  line-height: 15px;
  font-family: var(--font-mono);
}
.menu-list { padding: 6px; display: flex; flex-direction: column; }
.menu-divider {
  margin: 6px 10px 4px;
  font-size: 11px;
  color: var(--text-dim);
  letter-spacing: 0.5px;
}
.menu-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  border: none;
  background: none;
  padding: 9px 10px;
  border-radius: 8px;
  font-size: 13.5px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease;
}
.menu-item:hover { background: var(--bg-alt); }
.menu-item span { width: 18px; text-align: center; }
.menu-item.danger { color: #e5484d; }
.me-panel { width: 260px; }
</style>
