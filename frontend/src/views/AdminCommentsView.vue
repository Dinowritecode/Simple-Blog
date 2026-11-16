<script setup>
// 后台：评论管理（列表 + 删除；删除顶级评论会连同回复一起删）
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { formatDateTime } from '../utils/format'

const items = ref([])
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
    const d = await api.adminListComments({ page: page.value, pageSize })
    items.value = d.items || []
    total.value = d.total || 0
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function remove(c) {
  const label = c.parentId ? `这条回复` : `这条评论及其全部回复`
  if (!confirm(`确定删除${label}吗？此操作不可恢复。`)) return
  busyId.value = c.id
  try {
    await api.adminDeleteComment(c.id)
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
      <h1>评论管理</h1>
      <span class="dash-sub">共 {{ total }} 条（删除顶级评论会连同其下回复一起删除）</span>
    </div>

    <div v-if="loading" class="dash-state">加载中…</div>
    <div v-else-if="error" class="dash-state dash-error">{{ error }}</div>
    <div v-else-if="!items.length" class="dash-state">还没有任何评论</div>

    <div v-else class="dash-table-wrap">
      <table class="dash-table">
        <thead>
          <tr>
            <th>评论内容</th>
            <th>作者</th>
            <th>所属文章</th>
            <th>类型</th>
            <th>时间</th>
            <th class="th-actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in items" :key="c.id" class="comment-row">
            <td class="td-content">
              <span v-if="c.replyTo" class="c-reply-to">回复 @{{ c.replyTo.nickname || '用户' }}</span>
              <div class="c-text">{{ c.content }}</div>
            </td>
            <td>
              <span class="c-author">
                <span class="avatar c-avatar" style="--avatar-size: 26px">
                  <img v-if="c.author.avatar" :src="c.author.avatar" alt="" />
                  <span v-else class="avatar-letter">{{ (c.author.nickname || '?')[0] }}</span>
                </span>
                {{ c.author.nickname || '用户' }}
                <i v-if="c.author.role === 'admin'" class="badge-admin">站长</i>
              </span>
            </td>
            <td class="td-post">
              <router-link :to="`/blog/${c.postSlug || c.postId}`" target="_blank">{{ c.postTitle || `#${c.postId}` }}</router-link>
            </td>
            <td>
              <span class="badge" :class="c.parentId ? 'reply' : 'root'">{{ c.parentId ? '回复' : '评论' }}</span>
            </td>
            <td class="td-date">{{ formatDateTime(c.createdAt) }}</td>
            <td class="td-actions">
              <button class="link-btn danger" @click="remove(c)">
                {{ busyId === c.id ? '删除中…' : '删除' }}
              </button>
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
  align-items: baseline;
  gap: 14px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}
.dash-head h1 { font-size: 1.4rem; }
.dash-sub { color: var(--text-dim); font-size: 13px; }

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
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  vertical-align: middle;
}
.dash-table th { color: var(--text-dim); font-weight: 600; font-size: 12.5px; background: var(--bg-alt); }
.dash-table tbody tr:last-child td { border-bottom: none; }
.dash-table tbody tr:hover { background: var(--bg-alt); }

.td-content { min-width: 260px; }
.c-reply-to { font-size: 12.5px; color: var(--accent); }
.c-text {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 340px;
  line-height: 1.6;
  margin-top: 1px;
}
.c-author { display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; }
.c-avatar { flex-shrink: 0; }
.td-post a { font-size: 13.5px; }
.td-date { font-family: var(--font-mono); color: var(--text-dim); font-size: 12.5px; white-space: nowrap; }

.badge {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 999px;
  font-family: var(--font-mono);
}
.badge.root { color: var(--accent); background: var(--accent-soft); }
.badge.reply { color: var(--text-dim); background: var(--bg-alt); }

.th-actions, .td-actions { text-align: right !important; }
.td-actions { white-space: nowrap; }
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
  .dash-head { flex-direction: column; gap: 6px; align-items: flex-start; }
  .td-date { display: none; }
}
</style>
