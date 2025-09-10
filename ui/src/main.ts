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
    ,{ path: '/export/backup', component: () => import('./pages/BackupManagementPage.vue') }
    ,{ path: '/export/gitops', component: () => import('./pages/GitOpsExportPage.vue') }
    ,{ path: '/admin', component: () => import('./pages/AdminSettingsPage.vue') }
    ,{ path: '/stats', component: () => import('./pages/StatsPage.vue') }
    ,{ path: '/export/:id', component: () => import('./pages/ExportDetailPage.vue') }
    ,{ path: '/import/:id', component: () => import('./pages/ImportDetailPage.vue') }
    ,{ path: '/metrics', component: () => import('./pages/MetricsDashboardPage.vue') }
  ]
})

createApp(App)
  .use(pinia)
  .use(router)
  .mount('#app')
