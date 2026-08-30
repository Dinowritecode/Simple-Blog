<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import { formatDate } from '../utils/format'

const router = useRouter()

const posts = ref([])
const page = ref(1)
const pageSize = 10
const total = ref(0)
const loading = ref(true)
const error = ref('')
const busyId = ref(null)

const totalPages = () => Math.max(1, Math.ceil(total.value / pageSize))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.adminListPosts({ page: page.value, pageSize })
    posts.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function toggleStatus(post) {
  busyId.value = post.id
  try {
    await api.updatePost(post.id, { status: post.status === 'published' ? 'draft' : 'published' })
    await load()
  } catch (e) {
    alert(e.message)
  } finally {
    busyId.value = null
  }
}

async function remove(post) {
  if (!confirm(`确定删除《${post.title}》吗？此操作不可恢复。`)) return
  try {
    await api.deletePost(post.id)
    await load()
  } catch (e) {
    alert(e.message)
  }
}

function prev() { if (page.value > 1) { page.value--; load() } }
function next() { if (page.value < totalPages()) { page.value++; load() } }
</script>

<template>
  <div class="dash">
    <div class="dash-head">
      <h1>文章管理</h1>
      <router-link class="btn btn-primary" to="/admin/posts/new">＋ 写文章</router-link>
    </div>

    <div v-if="loading" class="dash-state">加载中…</div>
    <div v-else-if="error" class="dash-state dash-error">{{ error }}</div>
    <div v-else-if="!posts.length" class="dash-state">还没有文章，点击右上角「写文章」开始创作～</div>

    <div v-else class="dash-table-wrap">
      <table class="dash-table">
        <thead>
          <tr>
            <th>标题</th>
            <th>状态</th>
            <th>点赞</th>
            <th>更新时间</th>
            <th class="th-actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="post in posts" :key="post.id">
            <td class="td-title">
              <router-link :to="`/admin/posts/${post.id}/edit`">{{ post.title }}</router-link>
            </td>
            <td>
              <span class="badge" :class="post.status">{{ post.status === 'published' ? '已发布' : '草稿' }}</span>
            </td>
            <td class="td-likes">★ {{ post.likes }}</td>
            <td class="td-date">{{ formatDate(post.updatedAt) }}</td>
            <td class="td-actions">
              <button class="link-btn" @click="toggleStatus(post)">
                {{ busyId === post.id ? '…' : post.status === 'published' ? '下架' : '发布' }}
              </button>
              <router-link class="link-btn" :to="`/admin/posts/${post.id}/edit`">编辑</router-link>
              <button class="link-btn danger" @click="remove(post)">删除</button>
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
}
.dash-head h1 { font-size: 1.4rem; }

.dash-state { text-align: center; color: var(--text-dim); padding: 80px 0; }
.dash-error { color: #e5484d; }

.dash-table-wrap {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  overflow-x: auto;
}

.dash-table { width: 100%; border-collapse: collapse; font-size: 14px; }
.dash-table th, .dash-table td {
  text-align: left;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
.dash-table th { color: var(--text-dim); font-weight: 600; font-size: 12.5px; background: var(--bg-alt); }
.dash-table tbody tr:last-child td { border-bottom: none; }
.dash-table tbody tr:hover { background: var(--bg-alt); }

.td-title { max-width: 320px; overflow: hidden; text-overflow: ellipsis; }
.td-title a { color: var(--text); font-weight: 600; }
.td-title a:hover { color: var(--accent); }

.badge {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 999px;
  font-family: var(--font-mono);
}
.badge.published { color: #16a34a; background: rgba(22, 163, 74, 0.1); }
.badge.draft { color: var(--text-dim); background: var(--accent-soft); }

.td-likes { font-family: var(--font-mono); color: var(--text-dim); }
.td-date { font-family: var(--font-mono); color: var(--text-dim); font-size: 13px; }

.th-actions, .td-actions { text-align: right !important; }
.td-actions { display: flex; gap: 10px; justify-content: flex-end; }

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
  .dash-head { flex-direction: column; gap: 14px; align-items: flex-start; }
  .td-date { display: none; }
}
</style>
