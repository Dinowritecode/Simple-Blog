<script setup>
// 全局提示条宿主：渲染 useToast 队列，页面顶部弹出
import { useToast } from '../composables/useToast'

const { items, remove } = useToast()

function run(item) {
  if (typeof item.onAction === 'function') item.onAction()
  remove(item.id)
}
</script>

<template>
  <Teleport to="body">
    <div class="toast-host" aria-live="polite">
      <transition-group name="toast">
        <div v-for="t in items" :key="t.id" class="toast" :class="`toast-${t.type}`">
          <span class="toast-icon">{{ t.type === 'success' ? '✅' : t.type === 'error' ? '⚠️' : '💡' }}</span>
          <span class="toast-text">{{ t.text }}</span>
          <button v-if="t.actionLabel" class="toast-action" @click="run(t)">{{ t.actionLabel }}</button>
          <button class="toast-close" aria-label="关闭" @click="remove(t.id)">✕</button>
        </div>
      </transition-group>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-host {
  position: fixed;
  top: 74px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 2000;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  pointer-events: none;
  width: min(92vw, 560px);
}
.toast {
  pointer-events: auto;
  display: flex;
  align-items: center;
  gap: 10px;
  width: fit-content;
  max-width: 100%;
  padding: 11px 16px;
  border-radius: 12px;
  background: var(--card);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
  font-size: 14px;
  color: var(--text);
}
.toast-error { border-color: rgba(229, 72, 77, 0.5); }
.toast-success { border-color: rgba(22, 163, 74, 0.4); }
.toast-icon { flex-shrink: 0; }
.toast-text { line-height: 1.5; }
.toast-action {
  flex-shrink: 0;
  border: none;
  background: var(--accent-fill);
  color: var(--on-accent);
  border-radius: 8px;
  padding: 5px 12px;
  font-size: 13px;
  cursor: pointer;
}
.toast-action:hover { filter: brightness(1.1); }
.toast-close {
  flex-shrink: 0;
  border: none;
  background: none;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 12px;
  padding: 2px;
}
.toast-close:hover { color: var(--text); }

.toast-enter-active, .toast-leave-active { transition: all 0.3s ease; }
.toast-enter-from { opacity: 0; transform: translateY(-12px); }
.toast-leave-to { opacity: 0; transform: translateY(-8px); }
</style>
