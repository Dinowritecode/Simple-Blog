<script setup>
import { useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'

const router = useRouter()
const { displayName, avatarUrl, user, logout } = useAuth()
const { toast } = useToast()

function doLogout() {
  logout()
  toast.info('已退出登录')
  router.push('/admin/login')
}
</script>

<template>
  <div class="admin">
    <aside class="admin-side">
      <div class="admin-brand"><span class="accent">凛冬</span>RinEyce</div>
      <nav class="admin-nav">
        <router-link to="/admin" exact-active-class="active">📋 文章管理</router-link>
        <router-link to="/admin/posts/new" active-class="active">✏️ 写文章</router-link>
        <router-link to="/admin/comments" active-class="active">💬 评论管理</router-link>
        <router-link to="/admin/users" active-class="active">👥 用户管理</router-link>
        <router-link to="/admin/settings" active-class="active">🎨 页面管理</router-link>
        <router-link to="/admin/backup" active-class="active">🗄️ 备份迁移</router-link>
        <router-link to="/" target="_blank">🌐 返回首页</router-link>
      </nav>
      <div class="admin-user">
        <span class="avatar admin-avatar" style="--avatar-size: 40px">
          <img v-if="avatarUrl" :src="avatarUrl" alt="" />
          <span v-else class="avatar-letter">{{ (displayName || '?')[0] }}</span>
        </span>
        <div class="admin-user-id">
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

.admin-side {
  width: 220px;
  flex-shrink: 0;
  background: var(--bg-alt);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  padding: 20px 0;
  position: sticky;
  top: 0;
  height: 100svh;
}

.admin-brand {
  font-weight: 700;
  font-size: 16px;
  padding: 0 22px 20px;
  border-bottom: 1px solid var(--border);
}

.admin-nav { display: flex; flex-direction: column; padding: 16px 12px; gap: 4px; flex: 1; }
.admin-nav a {
  padding: 10px 12px;
  border-radius: 10px;
  color: var(--text-dim);
  font-size: 14.5px;
  transition: all 0.2s ease;
}
.admin-nav a:hover { color: var(--text); background: var(--accent-soft); text-decoration: none; }
.admin-nav a.active { color: var(--accent); background: var(--accent-soft); font-weight: 600; }

.admin-user {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 16px 22px 0;
  border-top: 1px solid var(--border);
  margin: 0 12px;
  font-size: 14px;
}
.admin-avatar { border: 2px solid var(--border); flex-shrink: 0; }
.admin-user-id { min-width: 0; }
.admin-name { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.admin-email { font-size: 11.5px; color: var(--text-dim); overflow: hidden; text-overflow: ellipsis; }
.admin-logout {
  background: none;
  border: none;
  color: var(--text-dim);
  font-size: 12.5px;
  cursor: pointer;
  padding: 0;
  margin-top: 2px;
}
.admin-logout:hover { color: #e5484d; }

.admin-main {
  flex: 1;
  min-width: 0;
  padding: 32px;
}

@media (max-width: 760px) {
  .admin { flex-direction: column; }
  .admin-side {
    width: 100%;
    height: auto;
    position: static;
    flex-direction: row;
    align-items: center;
    padding: 12px 16px;
    gap: 12px;
  }
  .admin-brand { border-bottom: none; padding: 0; }
  .admin-nav { flex-direction: row; padding: 0; gap: 8px; flex: 1; overflow-x: auto; }
  .admin-nav a { padding: 6px 10px; font-size: 13px; white-space: nowrap; }
  .admin-user { border-top: none; margin: 0; padding: 0; }
  .admin-user-id { display: none; }
  .admin-main { padding: 20px 16px; }
}
</style>
