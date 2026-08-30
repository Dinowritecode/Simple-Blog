<script setup>
const facts = [
  '语言：Rust · Go · C++',
  '方向：系统编程 · 网络服务 · 高性能工具',
  '原则：可读性优先，性能次之，缺一不可'
]

const stats = [
  { num: '3+', label: '年系统级开发经验' },
  { num: '∞', label: '对底层原理的好奇心' },
  { num: '0', label: '个未评审的 unsafe（愿望）' }
]
</script>

<template>
  <section class="section" id="about">
    <div class="section-inner">
      <h2 class="section-title" v-reveal><span class="accent">#</span> 关于我</h2>

      <div class="about-grid">
        <div class="about-text" v-reveal>
          <p>
            我是 <strong>凛冬RinEyce</strong>，一名专注系统级编程的开发者。喜欢与机器贴近的感觉：
            了解内存如何分配、指令如何执行、并发如何调度。
          </p>
          <p>
            日常主力是 <strong>Rust</strong> 与 <strong>Go</strong>，必要时也会直接用 <strong>C++</strong>
            与硬件打交道。相信「简单」是一种美德 —— 代码如此，生活亦然。
          </p>
          <ul class="about-facts">
            <li v-for="fact in facts" :key="fact">
              <span class="accent">▸</span> {{ fact }}
            </li>
          </ul>
        </div>

        <div class="about-stats">
          <div v-for="stat in stats" :key="stat.label" class="stat" v-reveal>
            <strong class="num">{{ stat.num }}</strong>
            <span>{{ stat.label }}</span>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.about-grid {
  display: grid;
  grid-template-columns: 1.4fr 1fr;
  gap: 48px;
  align-items: start;
}

.about-text p { margin-bottom: 16px; color: var(--text-dim); }
.about-text strong { color: var(--text); font-weight: 700; }

.about-facts { list-style: none; margin-top: 8px; }
.about-facts li { margin-bottom: 10px; color: var(--text-dim); font-size: 15px; }
.about-facts .accent { margin-right: 8px; }

.about-stats { display: flex; flex-direction: column; gap: 14px; }

.stat {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 20px 24px;
  box-shadow: var(--shadow);
  transition: transform 0.25s ease, border-color 0.25s ease;
}
.stat:hover { transform: translateY(-3px); border-color: var(--accent); }

.stat .num {
  display: block;
  font-family: var(--font-mono);
  font-size: 1.9rem;
  color: var(--accent);
  font-weight: 700;
  line-height: 1.2;
}
.stat span { color: var(--text-dim); font-size: 14px; }

@media (max-width: 760px) {
  .about-grid { grid-template-columns: 1fr; gap: 32px; }
}
</style>
