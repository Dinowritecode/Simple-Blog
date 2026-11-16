import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 后端代理目标：默认本地 8080；联调远程后端时设置环境变量 API_PROXY，例如：
//   PowerShell:  $env:API_PROXY = "http://服务器IP:8080"; npm run dev
const backendTarget = process.env.API_PROXY || 'http://localhost:8080'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    host: '127.0.0.1',
    open: false,
    // 忽略「临时目录 + 重命名」式原子写留下的中间产物。
    // 这类 .tmpdir 会被瞬间创建又删除，chokidar 对它 fs.watch 时会抛 EBUSY，
    // 而这是个未捕获异常，会让整个 dev server 直接退出 —— 表现就是「改了代码，
    // 浏览器刷新却没反应（其实服务已经死了，浏览器给的是缓存页）」。
    // 同理还有下面两类，实测都会以同样的方式把 dev server 干掉：
    //   · Chrome/Edge 的 user-data-dir：里面的 Cookies / Cache 是被浏览器独占锁定的
    //     文件，watch 必 EBUSY（踩过一次：frontend/dist-harness/prof*/Default/Network/Cookies）
    //   · 验证台产物目录（dist-harness / *-harness）：里面常常夹带上面那种 profile
    // 这类目录都不需要热更新，直接整体不监听最稳。
    watch: {
      ignored: [
        '**/*.tmpdir/**', '**/.*.tmpdir/**', '**/*.tmp', '**/.*.tmp',
        '**/.tools/**',
        // 验证台产物目录（注意只忽略「产物目录」本身，src/tools 下的验证代码照常热更新）
        '**/dist-harness/**', '**/dist-*/**',
        // 浏览器 user-data-dir：里面的 Cookies/Cache 被浏览器独占锁定，watch 必 EBUSY
        '**/prof*/**', '**/user-data*/**', '**/chrome-profile/**',
        '**/Default/Cache/**', '**/Default/Network/**',
        '**/Cookies', '**/Cookies-journal',
        // 手动丢进来的资源压缩包（字体/图片素材等一起放在 src 下时）：
        // 这类文件在解压/复制过程中会被独占锁定，watch 必 EBUSY 并直接干掉 dev server
        '**/*.zip', '**/*.rar', '**/*.7z', '**/*.tar', '**/*.gz'
      ]
    },
    // 开发时把 /api 与 /uploads 代理到 Go 后端（本地或远程服务器均可）
    // /api/user/events 是 SSE 长连接：必须关掉缓冲/超时，否则事件会被攒着不发、
    // 或者空闲一会儿就被代理掐断（表现为「私信不实时」/「红点不亮」）。
    proxy: {
      '/api': {
        target: backendTarget,
        changeOrigin: true,
        ws: false,
        timeout: 0,
        proxyTimeout: 0,
        configure: (proxy) => {
          proxy.on('proxyReq', (proxyReq, req) => {
            if (req.url && req.url.includes('/api/user/events')) {
              proxyReq.setHeader('Accept', 'text/event-stream')
              proxyReq.setHeader('Cache-Control', 'no-cache')
              proxyReq.setHeader('Connection', 'keep-alive')
            }
          })
          proxy.on('proxyRes', (proxyRes, req) => {
            if (req.url && req.url.includes('/api/user/events')) {
              // 关掉 Node 的响应缓冲，让每条事件立刻透传给浏览器
              proxyRes.headers['x-accel-buffering'] = 'no'
              proxyRes.headers['cache-control'] = 'no-cache, no-transform'
            }
          })
        }
      },
      '/uploads': backendTarget
    }
  },
  build: {
    outDir: 'dist',
    assetsInlineLimit: 4096
  }
})
