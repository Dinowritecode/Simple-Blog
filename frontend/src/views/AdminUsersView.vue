<script setup>
// 后台：用户管理（列表 + 搜索 + 删除；站长账号与自身不可删）
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { useAuth } from '../composables/useAuth'
import { formatDate } from '../utils/format'

const { user } = useAuth()
const items = ref([])
const page = ref(1)
const pageSize = 10
const total = ref(0)
const loading = ref(true)
const error = ref('')
const keyword = ref('')
const busyId = ref(null)

const totalPages = () => Math.max(1, Math.ceil(total.value / pageSize))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const d = await api.adminListUsers({ page: page.value, pageSize, keyword: keyword.value })
    items.value = d.items || []
    total.value = d.total || 0
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)

function search() {
  page.value = 1
  load()
}

async function remove(u) {
  if (!confirm(`确定删除用户「${u.nickname || u.email}」吗？\n其评论与通知会被一并删除，不可恢复。`)) return
  busyId.value = u.id
  try {
    await api.adminDeleteUser(u.id)
    await load()
  } catch (e) {
    alert(e.message)
  } finally {
    busyId.value = null
  }
}

function prev() { if (page.value > 1) { page.value--; load() } }
function next() { if (page.value < totalPages()) { page.value++; load() } }
</script>

<template>
  <div class="dash">
    <div class="dash-head">
      <h1>用户管理</h1>
      <div class="search-bar">
        <input v-model="keyword" placeholder="按邮箱 / 昵称搜索" @keyup.enter="search" />
        <button class="btn-sm ghost" @click="search">搜索</button>
      </div>
    </div>

    <div v-if="loading" class="dash-state">加载中…</div>
    <div v-else-if="error" class="dash-state dash-error">{{ error }}</div>
    <div v-else-if="!items.length" class="dash-state">没有找到用户</div>

    <div v-else class="dash-table-wrap">
      <table class="dash-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>用户</th>
            <th>邮箱</th>
            <th>角色</th>
            <th>评论数</th>
            <th>注册时间</th>
            <th class="th-actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in items" :key="u.id">
            <td class="td-id">{{ u.id }}</td>
            <td>
              <span class="u-user">
                <span class="avatar u-avatar" style="--avatar-size: 30px">
                  <img v-if="u.avatar" :src="u.avatar" alt="" />
                  <span v-else class="avatar-letter">{{ (u.nickname || u.email || '?')[0] }}</span>
                </span>
                <span class="u-name">{{ u.nickname || '(未设置昵称)' }}</span>
              </span>
            </td>
            <td class="td-email">{{ u.email }}</td>
            <td>
              <span class="badge" :class="u.role">{{ u.role === 'admin' ? '站长' : '用户' }}</span>
            </td>
            <td class="td-num">{{ u.commentCount }}</td>
            <td class="td-date">{{ formatDate(u.createdAt) }}</td>
            <td class="td-actions">
              <template v-if="u.id === user?.id">
                <span class="td-self">当前账号</span>
              </template>
              <template v-else-if="u.role === 'admin'">
                <span class="td-self">—</span>
              </template>
              <template v-else>
                <button class="link-btn danger" @click="remove(u)">
                  {{ busyId === u.id ? '删除中…' : '删除' }}
                </button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="totalPages() > 1" class="pager">
        <button class="btn btn-ghost" :disabled="page <= 1" @click="prev">← 上一页</button>
        <span class="pager-info">{{ page }} / {{ totalPages() }}</span>
        <button class="btn btn-ghost" :disabled="page >= totalPages()" @click="next">下一页 →</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dash-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
  gap: 12px;
  flex-wrap: wrap;
}
.dash-head h1 { font-size: 1.4rem; }

.search-bar { display: flex; gap: 8px; }
.search-bar input {
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--bg);
  color: var(--text);
  font-size: 14px;
  outline: none;
  width: 200px;
}
.search-bar input:focus { border-color: var(--accent); }

.dash-state { text-align: center; color: var(--text-dim); padding: 80px 0; }
.dash-error { color: #e5484d; }

.dash-table-wrap {
  background: var(--glass-solid-bg);
  backdrop-filter: var(--glass-solid-blur);
  -webkit-backdrop-filter: var(--glass-solid-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  overflow-x: auto;
}
.dash-table { width: 100%; border-collapse: collapse; font-size: 14px; }
.dash-table th, .dash-table td {
  text-align: left;
  padding: 11px 16px;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
.dash-table th { color: var(--text-dim); font-weight: 600; font-size: 12.5px; background: var(--bg-alt); }
.dash-table tbody tr:last-child td { border-bottom: none; }
.dash-table tbody tr:hover { background: var(--bg-alt); }

.td-id, .td-num { font-family: var(--font-mono); color: var(--text-dim); }
.td-email { color: var(--text-dim); }
.td-date { font-family: var(--font-mono); color: var(--text-dim); font-size: 12.5px; }

.u-user { display: inline-flex; align-items: center; gap: 8px; }
.u-avatar { border: 1px solid var(--border); }
.u-name { font-weight: 600; }

.badge {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 999px;
  font-family: var(--font-mono);
}
.badge.admin { color: var(--accent); background: var(--accent-soft); }
.badge.user { color: var(--text-dim); background: var(--bg-alt); }

.th-actions, .td-actions { text-align: right !important; }
.td-self { color: var(--text-dim); font-size: 12.5px; }

.link-btn {
  background: none;
  border: none;
  color: var(--accent);
  font-size: 13.5px;
  cursor: pointer;
  padding: 0;
}
.link-btn:hover { text-decoration: underline; }
.link-btn.danger { color: #e5484d; }

.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 20px;
  padding: 18px;
}
.pager .btn:disabled { opacity: 0.4; cursor: not-allowed; transform: none; }
.pager-info { font-family: var(--font-mono); color: var(--text-dim); font-size: 14px; }

@media (max-width: 760px) {
  .dash-head { flex-direction: column; align-items: flex-start; }
  .search-bar { width: 100%; }
  .search-bar input { flex: 1; width: auto; }
  .td-email, .td-date { display: none; }
}
</style>
