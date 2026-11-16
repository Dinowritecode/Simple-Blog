import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 独立文档站构建配置
export default defineConfig({
  plugins: [vue()],
  // 关键：使用相对路径，构建产物可放到任意目录/直接双击打开（无需服务器）
  base: './',
  server: {
    port: 5174, // 与主项目前端(5173)区分开
    open: false
  },
  build: {
    outDir: 'dist'
  }
})
