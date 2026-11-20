<script setup>
// ============================================================
// TagInput.vue —— 「输入即标签」的输入框
//
// 交互：
//   · 输入文字后按回车、或点别处（失焦）、或输入逗号 → 直接变成一个标签
//   · 标签右侧有 ×，点一下就删
//   · 输入框为空时按退格，删掉最后一个标签
//   · single = true 时只保留一个标签（分类就是这种）
//
// 数据仍然是字符串（分类用原值，标签用逗号分隔），
// 这样后端接口完全不用改。
// ============================================================
import { computed, nextTick, ref } from 'vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  // 是否只允许一个（分类）
  single: { type: Boolean, default: false },
  separator: { type: String, default: ',' },
  placeholder: { type: String, default: '输入后回车' },
  // 可选建议（已有分类之类），点一下直接加
  suggestions: { type: Array, default: () => [] }
})
const emit = defineEmits(['update:modelValue'])

const draft = ref('')
const inputEl = ref(null)

const items = computed(() => {
  const raw = String(props.modelValue || '')
  if (!raw.trim()) return []
  return raw
    .split(props.separator)
    .map((s) => s.trim())
    .filter(Boolean)
})

function commit(text) {
  const v = String(text || '').trim()
  if (!v) return
  // 允许一次粘贴多个（用逗号分隔）
  const parts = v.split(props.separator).map((s) => s.trim()).filter(Boolean)
  if (!parts.length) return
  let next = props.single ? [parts[0]] : [...items.value]
  for (const p of parts) {
    if (!next.includes(p)) next.push(p)
  }
  if (props.single) next = [next[next.length - 1]]
  emit('update:modelValue', next.join(props.separator))
  draft.value = ''
}

function onKeydown(e) {
  if (e.key === 'Enter') {
    e.preventDefault()
    commit(draft.value)
    return
  }
  if (e.key === 'Backspace' && !draft.value && items.value.length) {
    remove(items.value[items.value.length - 1])
  }
}

function remove(tag) {
  const next = items.value.filter((t) => t !== tag)
  emit('update:modelValue', next.join(props.separator))
}

// 失焦时把没提交的内容也变成标签
function onBlur() {
  if (draft.value.trim()) commit(draft.value)
}

function focusInput() {
  nextTick(() => inputEl.value?.focus())
}
</script>

<template>
  <div class="tag-input" @click="focusInput">
    <span v-for="t in items" :key="t" class="tag">
      <span class="tag-text">{{ t }}</span>
      <button class="tag-x" type="button" :title="`移除 ${t}`" @click.stop="remove(t)">×</button>
    </span>
    <input
      ref="inputEl"
      v-model="draft"
      type="text"
      class="tag-entry"
      :placeholder="items.length ? '' : placeholder"
      @keydown="onKeydown"
      @blur="onBlur"
      @input="(e) => { if (e.target.value.includes(separator)) commit(e.target.value) }"
    />
  </div>

  <div v-if="suggestions.length" class="tag-sugg">
    <span class="sugg-label">已有：</span>
    <button
      v-for="s in suggestions"
      :key="s"
      class="sugg"
      :class="{ on: items.includes(s) }"
      type="button"
      :disabled="items.includes(s)"
      @click="commit(s)"
    >{{ s }}</button>
  </div>
</template>

<style scoped>
.tag-input {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-height: 40px;
  padding: 5px 8px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--bg);
  cursor: text;
  transition: border-color 0.18s ease;
}
.tag-input:focus-within { border-color: var(--accent); }

.tag {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  max-width: 100%;
  padding: 3px 4px 3px 9px;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent-ink);
  font-size: 12.5px;
  line-height: 1.2;
}
.tag-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tag-x {
  flex: none;
  width: 16px;
  height: 16px;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: inherit;
  font-size: 13px;
  line-height: 1;
  cursor: pointer;
  opacity: 0.7;
}
.tag-x:hover { background: rgba(0, 0, 0, 0.12); opacity: 1; }

.tag-entry {
  flex: 1;
  min-width: 80px;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--text);
  font-size: 13.5px;
  font-family: inherit;
  padding: 3px 2px;
}

.tag-sugg { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin-top: 6px; }
.sugg-label { font-size: 11.5px; color: var(--text-dim); }
.sugg {
  padding: 2px 9px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: transparent;
  color: var(--text-dim);
  font-size: 11.5px;
  cursor: pointer;
  transition: all 0.16s ease;
}
.sugg:hover:not(:disabled) { border-color: var(--accent); color: var(--accent); }
.sugg.on { opacity: 0.45; cursor: default; }
</style>
