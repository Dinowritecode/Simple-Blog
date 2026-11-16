<script setup>
// ============================================================
// AdminBackupView.vue —— 后台「备份与迁移」
//
// 两件事：
//   ① 把当前数据导出成一个 zip（数据库 + 文章正文 + 上传文件，可勾选）
//   ② 上传一个备份 zip 并恢复（**整体替换**，需要输入确认词）
//
// 导出的下载走 fetch + Blob 而不是直接用 <a href>：
// 导出接口要带 Authorization 头，浏览器直接导航是不会带上的。
// 代价是文件要先落在内存里 —— 所以特别大的站（几百 MB 以上）建议改用
// 服务端的 `backup` 命令，那一份是直接流式写盘的。
// ============================================================

import { computed, onMounted, ref } from 'vue'
import { api } from '../api'
import { useToast } from '../composables/useToast'

const { toast } = useToast()

const loading = ref(true)
const info = ref(null)

// ---------- 导出 ----------
const wantDB = ref(true)
const wantPosts = ref(true)
const wantUploads = ref(true)
const exporting = ref(false)

// ---------- 恢复 ----------
const CONFIRM_WORD = 'REPLACE'
const fileInput = ref(null)
const picked = ref(null) // 选中的 File
const confirmText = ref('')
const restoring = ref(false)
const report = ref(null)

const parts = computed(() => {
  const out = []
  if (wantDB.value) out.push('db')
  if (wantPosts.value) out.push('posts')
  if (wantUploads.value) out.push('uploads')
  return out
})
const canExport = computed(() => parts.value.length > 0 && !exporting.value)
const canRestore = computed(
  () => !!picked.value && confirmText.value.trim() === CONFIRM_WORD && !restoring.value
)

// 本次导出的大致体积（数据库部分很小，主要看文件）
const exportSize = computed(() => {
  if (!info.value) return 0
  let n = 0
  if (wantPosts.value) n += info.value.postsBytes || 0
  if (wantUploads.value) n += info.value.uploadsBytes || 0
  return n
})

function human(bytes) {
  const n = Number(bytes) || 0
  if (n < 1024) return n + ' B'
  const units = ['KB', 'MB', 'GB', 'TB']
  let v = n
  for (const u of units) {
    v /= 1024
    if (v < 1024) return v.toFixed(1) + ' ' + u
  }
  return v.toFixed(1) + ' PB'
}

async function load() {
  loading.value = true
  try {
    info.value = await api.backupInfo()
  } catch (e) {
    toast.error(e.message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

// ---------- 导出 ----------
async function doExport() {
  if (!canExport.value) return
  exporting.value = true
  try {
    // 手动带上 JWT：这是唯一一处不能走 request() 的接口（要拿二进制）
    const res = await fetch(api.backupExportUrl(parts.value.join(',')), {
      headers: { Authorization: 'Bearer ' + (localStorage.getItem('token') || '') }
    })
    if (!res.ok) {
      let msg = `导出失败 (${res.status})`
      try {
        const j = await res.json()
        if (j.error) msg = j.error
      } catch {
        /* 非 JSON 响应，用默认文案 */
      }
      throw new Error(msg)
    }
    const blob = await res.blob()
    const name = filenameFrom(res.headers.get('Content-Disposition'))
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    document.body.appendChild(a)
    a.click()
    a.remove()
    // 立刻 revoke 会让部分浏览器来不及开始下载，延后一点
    setTimeout(() => URL.revokeObjectURL(url), 10000)
    toast.success(`已导出 ${name}（${human(blob.size)}）`)
  } catch (e) {
    toast.error(e.message)
  } finally {
    exporting.value = false
  }
}

// 从 Content-Disposition 里取文件名，取不到就自己拼一个
function filenameFrom(header) {
  const m = /filename="?([^";]+)"?/.exec(header || '')
  if (m) return m[1]
  const t = new Date()
  const p = (n) => String(n).padStart(2, '0')
  return `rinblog-backup-${t.getFullYear()}${p(t.getMonth() + 1)}${p(t.getDate())}-${p(t.getHours())}${p(t.getMinutes())}${p(t.getSeconds())}.zip`
}

// ---------- 恢复 ----------
function pickFile() {
  fileInput.value && fileInput.value.click()
}
function onFile(e) {
  const f = e.target.files && e.target.files[0]
  e.target.value = ''
  if (!f) return
  if (!/\.zip$/i.test(f.name)) return toast.error('请选择 .zip 备份文件')
  picked.value = f
  report.value = null
}

async function doRestore() {
  if (!canRestore.value) return
  restoring.value = true
  report.value = null
  try {
    const d = await api.importBackup(picked.value, confirmText.value.trim())
    report.value = d.report || null
    confirmText.value = ''
    picked.value = null
    toast.success('恢复完成，数据已替换为备份内容')
    load() // 概览重新统计
  } catch (e) {
    toast.error(e.message)
  } finally {
    restoring.value = false
  }
}

// 概览里的表名排序：行数多的在前
const tableRows = computed(() => {
  const t = (info.value && info.value.tables) || {}
  return Object.entries(t).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
})
</script>

<template>
  <div class="backup-page">
    <header class="page-head">
      <h1>🗄️ 备份与迁移</h1>
      <p class="page-desc">
        把整站数据打成一个 zip 备份，或从备份整体恢复。
        换服务器时在新机器上跑一次「恢复」即可 —— 数据库、文章正文、上传的图片一起搬过去。
      </p>
    </header>

    <div v-if="loading" class="state">加载中…</div>

    <template v-else-if="info">
      <!-- ---------- 当前数据规模 ---------- -->
      <section class="card">
        <h2>当前数据</h2>
        <div class="stats">
          <div class="stat">
            <span class="stat-n">{{ info.totalRows }}</span>
            <span class="stat-l">数据库行（{{ tableRows.length }} 张表）</span>
          </div>
          <div class="stat">
            <span class="stat-n">{{ info.posts }}</span>
            <span class="stat-l">文章正文（{{ human(info.postsBytes) }}）</span>
          </div>
          <div class="stat">
            <span class="stat-n">{{ info.uploads }}</span>
            <span class="stat-l">上传文件（{{ human(info.uploadsBytes) }}）</span>
          </div>
        </div>
        <div class="tables">
          <span v-for="[name, n] in tableRows" :key="name" class="tbl">
            {{ name }}<em>{{ n }}</em>
          </span>
        </div>
      </section>

      <!-- ---------- 导出 ---------- -->
      <section class="card">
        <h2>📤 导出备份</h2>
        <p class="desc">
          生成一个 zip 归档：<code>manifest.json</code> + <code>db/*.ndjson</code>（每张表一行一条 JSON）
          + <code>posts/</code>（文章正文）+ <code>uploads/</code>（图片）。
        </p>

        <label class="check">
          <input type="checkbox" v-model="wantDB" />
          <span class="check-body">
            <span class="check-name">数据库</span>
            <span class="check-desc">{{ info.totalRows }} 行 / {{ tableRows.length }} 张表（用户、文章、评论、私信、设置等）</span>
          </span>
        </label>
        <label class="check">
          <input type="checkbox" v-model="wantPosts" :disabled="!info.posts" />
          <span class="check-body">
            <span class="check-name">文章正文</span>
            <span class="check-desc">{{ info.posts }} 个文件 / {{ human(info.postsBytes) }}（不勾选的话文章会只剩标题和摘要）</span>
          </span>
        </label>
        <label class="check">
          <input type="checkbox" v-model="wantUploads" :disabled="!info.uploads" />
          <span class="check-body">
            <span class="check-name">上传文件</span>
            <span class="check-desc">{{ info.uploads }} 个文件 / {{ human(info.uploadsBytes) }}（封面图、头像等）</span>
          </span>
        </label>

        <div class="actions">
          <span class="hint">
            预计大小约 <strong>{{ human(exportSize) }}</strong>（不含数据库部分，它通常只有几十 KB）
          </span>
          <button class="btn-sm" :disabled="!canExport" @click="doExport">
            {{ exporting ? '正在打包…' : '⬇️ 下载备份' }}
          </button>
        </div>
      </section>

      <!-- ---------- 恢复 ---------- -->
      <section class="card danger-card">
        <h2>📥 从备份恢复</h2>
        <div class="warn">
          <strong>这是破坏性操作。</strong>
          恢复会<strong>清空当前的数据库与文件</strong>，然后整体替换成备份里的内容，且无法撤销。
          确认无误前，建议先在上面导出一份当前数据。
        </div>

        <div class="row">
          <input ref="fileInput" type="file" accept=".zip,application/zip" class="hidden-file" @change="onFile" />
          <button class="btn-sm ghost" @click="pickFile">选择备份文件…</button>
          <span v-if="picked" class="picked">
            {{ picked.name }} <em>{{ human(picked.size) }}</em>
          </span>
          <span v-else class="hint">还没有选择文件</span>
        </div>

        <div class="row">
          <label class="confirm-label">
            输入 <code>{{ CONFIRM_WORD }}</code> 以确认：
          </label>
          <input
            class="confirm-input"
            type="text"
            autocomplete="off"
            :placeholder="CONFIRM_WORD"
            v-model="confirmText"
          />
        </div>

        <div class="actions">
          <span class="hint">
            {{ canRestore ? '确认无误，可以执行恢复' : `选择文件并输入 ${CONFIRM_WORD} 后按钮才会可用` }}
          </span>
          <button class="btn-sm danger" :disabled="!canRestore" @click="doRestore">
            {{ restoring ? '正在恢复…' : '♻️ 恢复并覆盖' }}
          </button>
        </div>

        <!-- 恢复结果 -->
        <div v-if="report" class="report">
          <h3>✅ 恢复完成</h3>
          <p class="desc">备份时间：{{ report.manifest.createdAt }}</p>
          <div class="tables">
            <span v-for="(n, name) in report.tables" :key="name" class="tbl">{{ name }}<em>{{ n }}</em></span>
          </div>
          <p class="desc">
            文件：文章正文 {{ report.posts }} 个 / 上传 {{ report.uploads }} 个
          </p>
          <p v-if="report.missing && report.missing.length" class="warn-line">
            ⚠️ 备份与本机对不上的表：{{ report.missing.join('、') }}
          </p>
        </div>
      </section>

      <!-- ---------- 命令行 ---------- -->
      <section class="card">
        <h2>💻 命令行（迁移到新机器时更顺手）</h2>
        <p class="desc">
          新服务器上往往还没法登后台，这时可以直接用二进制的命令。
          命令行版本是<strong>直接流式写盘</strong>的，不受浏览器内存限制，几百 MB 的备份也稳。
        </p>
        <pre class="code"># 在当前机器上导出（不启动服务）
blog-server backup /path/backup-20260926.zip

# 只备份数据库和正文，不含上传文件
blog-server backup out.zip db,posts

# 在新机器上恢复（会整体替换，REPLACE 是必须的确认词）
blog-server restore /path/backup-20260926.zip REPLACE</pre>
        <p class="desc">
          新机器的准备顺序：装好二进制 → 配好 <code>DB_*</code> 环境变量 → 先跑一次让它自动建表
          → 执行 <code>restore</code> → 正常启动服务。
        </p>
      </section>
    </template>
  </div>
</template>

<style scoped>
.backup-page { max-width: 760px; }
.page-head { margin-bottom: 24px; }
.page-head h1 { font-size: 1.5rem; margin-bottom: 8px; }
.page-desc { color: var(--text-dim); font-size: 13.5px; line-height: 1.8; }

.state { color: var(--text-dim); padding: 40px 0; }

.card {
  background: var(--glass-solid-bg);
  backdrop-filter: var(--glass-solid-blur);
  -webkit-backdrop-filter: var(--glass-solid-blur);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 24px;
  margin-bottom: 20px;
}
.card h2 { font-size: 1.1rem; margin-bottom: 10px; }
.card h3 { font-size: 0.98rem; margin-bottom: 8px; }
.desc { color: var(--text-dim); font-size: 13px; line-height: 1.8; margin-bottom: 14px; }
.desc code, .code { font-family: var(--font-mono); font-size: 12px; }
.desc code { background: color-mix(in srgb, var(--text) 8%, transparent); padding: 1px 5px; border-radius: 4px; }

/* 恢复卡片整体偏红，视觉上先提醒一次 */
.danger-card { border-color: color-mix(in srgb, #e5484d 45%, transparent); }
.warn {
  background: color-mix(in srgb, #e5484d 10%, transparent);
  border: 1px solid color-mix(in srgb, #e5484d 30%, transparent);
  border-radius: 10px;
  padding: 12px 14px;
  font-size: 13px;
  line-height: 1.8;
  color: var(--text);
  margin-bottom: 16px;
}
.warn-line { color: #e5484d; font-size: 12.5px; margin-top: 8px; }

/* ---------- 当前数据 ---------- */
.stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-bottom: 14px; }
.stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 14px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--text) 5%, transparent);
}
.stat-n { font-size: 1.35rem; font-weight: 700; color: var(--accent); font-family: var(--font-mono); }
.stat-l { font-size: 12px; color: var(--text-dim); }

.tables { display: flex; flex-wrap: wrap; gap: 6px; }
.tbl {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--text-dim);
  background: color-mix(in srgb, var(--text) 7%, transparent);
  border-radius: 6px;
  padding: 4px 8px;
}
.tbl em { font-style: normal; margin-left: 6px; color: var(--accent); }

/* ---------- 勾选项 ---------- */
.check {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
  margin-bottom: 10px;
  cursor: pointer;
  transition: border-color 0.2s ease;
}
.check:hover { border-color: var(--accent); }
.check input { margin-top: 3px; width: 17px; height: 17px; accent-color: var(--accent); flex-shrink: 0; cursor: pointer; }
.check-body { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.check-name { font-size: 14px; font-weight: 600; }
.check-desc { font-size: 12px; color: var(--text-dim); line-height: 1.6; }

/* ---------- 行 / 按钮 ---------- */
.row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 12px; }
.hidden-file { display: none; }
.picked { font-size: 13px; font-family: var(--font-mono); }
.picked em { font-style: normal; color: var(--text-dim); margin-left: 6px; }
.hint { font-size: 12.5px; color: var(--text-dim); }

.confirm-label { font-size: 13px; color: var(--text); white-space: nowrap; }
.confirm-label code {
  font-family: var(--font-mono);
  background: color-mix(in srgb, var(--text) 8%, transparent);
  padding: 1px 5px;
  border-radius: 4px;
}
.confirm-input { flex: 1; min-width: 140px; max-width: 220px; font-family: var(--font-mono); }

.actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 16px;
}
.actions .btn-sm { flex-shrink: 0; }
.actions .btn-sm.danger { background: #e5484d; color: #fff; }
.actions .btn-sm.danger:disabled { opacity: 0.5; cursor: not-allowed; }

/* ---------- 结果 ---------- */
.report {
  margin-top: 18px;
  padding: 16px;
  border-radius: 10px;
  background: color-mix(in srgb, #22c55e 10%, transparent);
  border: 1px solid color-mix(in srgb, #22c55e 30%, transparent);
}

/* ---------- 代码块 ---------- */
.code {
  background: var(--bg-alt);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px;
  font-size: 12px;
  line-height: 1.9;
  overflow-x: auto;
  white-space: pre;
  margin-bottom: 14px;
  color: var(--text);
}

@media (max-width: 620px) {
  .stats { grid-template-columns: 1fr; }
  .actions { flex-direction: column; align-items: stretch; }
  .actions .btn-sm { width: 100%; }
}
</style>
