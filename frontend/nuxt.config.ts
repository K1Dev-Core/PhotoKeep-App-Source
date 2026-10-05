export default defineNuxtConfig({
  ssr: false,
  devtools: { enabled: false },
  app: { head: { title: 'PhotoKeep — สำรองรูปภาพ', meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }] } },
  compatibilityDate: '2026-01-01',
  vite: { server: { hmr: { host: 'localhost' } } }
})
