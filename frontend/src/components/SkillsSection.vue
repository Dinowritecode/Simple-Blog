<script setup>
const skills = [
  {
    logo: '🦀',
    name: 'Rust',
    desc: '所有权与生命周期、异步运行时（tokio）、CLI 工具与 WASM 开发。',
    tags: ['tokio', 'serde', 'clap', 'wasm']
  },
  {
    logo: '🐹',
    name: 'Go',
    desc: '高并发网络服务、微服务与中间件，追求简单可靠的工程实践。',
    tags: ['goroutine', 'gin', 'grpc', 'etcd']
  },
  {
    logo: '⚙️',
    name: 'C++',
    desc: '性能敏感模块、游戏/图形相关开发，熟悉 Modern C++（17/20）。',
    tags: ['C++20', 'STL', 'CMake', 'OpenGL']
  }
]
</script>

<template>
  <section class="section section-alt" id="skills">
    <div class="section-inner">
      <h2 class="section-title" v-reveal><span class="accent">#</span> 技能</h2>

      <div class="skills-grid">
        <article
          v-for="(skill, i) in skills"
          :key="skill.name"
          class="skill-card"
          v-reveal="{ delay: i * 100 }"
        >
          <div class="skill-head">
            <span class="skill-logo">{{ skill.logo }}</span>
            <h3>{{ skill.name }}</h3>
          </div>
          <p>{{ skill.desc }}</p>
          <ul class="tags">
            <li v-for="tag in skill.tags" :key="tag">{{ tag }}</li>
          </ul>
        </article>
      </div>
    </div>
  </section>
</template>

<style scoped>
.skills-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 22px;
}

.skill-card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 28px 26px;
  box-shadow: var(--shadow);
  transition: transform 0.25s ease, border-color 0.25s ease, box-shadow 0.25s ease;
}
.skill-card:hover {
  transform: translateY(-5px);
  border-color: var(--accent);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.08);
}

.skill-head { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.skill-logo { font-size: 1.8rem; }
.skill-head h3 { font-size: 1.2rem; }

.skill-card p { color: var(--text-dim); font-size: 14.5px; margin-bottom: 16px; }

.tags { list-style: none; display: flex; flex-wrap: wrap; gap: 8px; }
.tags li {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid transparent;
  border-radius: 999px;
  padding: 3px 12px;
}
</style>
