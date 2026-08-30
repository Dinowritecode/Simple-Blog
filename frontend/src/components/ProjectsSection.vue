<script setup>
const githubUrl = 'https://github.com/Dinowritecode'

const projects = [
  {
    name: 'cargo-winter',
    lang: 'Rust',
    desc: '一个极简的 Rust 项目脚手架工具：一条命令生成带 CI、lint 与文档的完整工程结构。',
    stars: 128
  },
  {
    name: 'frost-proxy',
    lang: 'Go',
    desc: '基于 Go 的高性能轻量反向代理，支持动态路由与优雅热重载，内存占用极低。',
    stars: 86
  },
  {
    name: 'glacier-engine',
    lang: 'C++',
    desc: '一个实验性的 2D 游戏引擎，手写渲染管线与 ECS，用于学习图形学与性能优化。',
    stars: 42
  }
]
</script>

<template>
  <section class="section" id="projects">
    <div class="section-inner">
      <h2 class="section-title" v-reveal><span class="accent">#</span> 项目</h2>

      <div class="projects-grid">
        <article
          v-for="(project, i) in projects"
          :key="project.name"
          class="project-card"
          v-reveal="{ delay: i * 100 }"
        >
          <div class="project-top">
            <span class="project-dot"></span>
            <span class="project-lang">{{ project.lang }}</span>
          </div>
          <h3>{{ project.name }}</h3>
          <p>{{ project.desc }}</p>
          <div class="project-foot">
            <span class="project-stars">★ {{ project.stars }}</span>
            <a :href="githubUrl" target="_blank" rel="noopener" class="project-link">查看源码 →</a>
          </div>
        </article>
      </div>
    </div>
  </section>
</template>

<style scoped>
.projects-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 22px;
}

.project-card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 26px;
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  gap: 12px;
  transition: transform 0.25s ease, border-color 0.25s ease;
}
.project-card:hover { transform: translateY(-5px); border-color: var(--accent); }

.project-top { display: flex; align-items: center; gap: 10px; }
.project-dot {
  width: 10px; height: 10px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 0 4px var(--accent-soft);
}
.project-lang {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-dim);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 1px 8px;
}

.project-card h3 { font-size: 1.15rem; }
.project-card p { color: var(--text-dim); font-size: 14.5px; flex: 1; }

.project-foot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-top: 1px dashed var(--border);
  padding-top: 14px;
  font-size: 13.5px;
}
.project-stars { color: var(--text-dim); font-family: var(--font-mono); }
.project-link { font-weight: 600; }
.project-link:hover { text-decoration: underline; }
</style>
