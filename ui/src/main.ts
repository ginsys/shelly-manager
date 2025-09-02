import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'

const pinia = createPinia()
const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('./pages/DevicesPage.vue') },
    { path: '/devices/:id', component: () => import('./pages/DeviceDetailPage.vue') },
    { path: '/export/history', component: () => import('./pages/ExportHistoryPage.vue') }
    ,{ path: '/import/history', component: () => import('./pages/ImportHistoryPage.vue') }
  ]
})

createApp(App)
  .use(pinia)
  .use(router)
  .mount('#app')
