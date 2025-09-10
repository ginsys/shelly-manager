import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'

const pinia = createPinia()
const router = createRouter({
  history: createWebHistory(),
  routes: [
    // Main pages
    { 
      path: '/', 
      name: 'devices',
      component: () => import('./pages/DevicesPage.vue'),
      meta: { title: 'Devices' }
    },
    { 
      path: '/devices/:id', 
      name: 'device-detail',
      component: () => import('./pages/DeviceDetailPage.vue'),
      meta: { title: 'Device Details' }
    },
    
    // Export & Import routes
    { 
      path: '/export/schedules', 
      name: 'export-schedules',
      component: () => import('./pages/ExportSchedulesPage.vue'),
      meta: { title: 'Schedule Management', category: 'export' }
    },
    { 
      path: '/export/backup', 
      name: 'export-backup',
      component: () => import('./pages/BackupManagementPage.vue'),
      meta: { title: 'Backup Management', category: 'export' }
    },
    { 
      path: '/export/gitops', 
      name: 'export-gitops',
      component: () => import('./pages/GitOpsExportPage.vue'),
      meta: { title: 'GitOps Export', category: 'export' }
    },
    { 
      path: '/export/history', 
      name: 'export-history',
      component: () => import('./pages/ExportHistoryPage.vue'),
      meta: { title: 'Export History', category: 'export' }
    },
    { 
      path: '/export/:id', 
      name: 'export-detail',
      component: () => import('./pages/ExportDetailPage.vue'),
      meta: { title: 'Export Details', category: 'export' }
    },
    { 
      path: '/import/history', 
      name: 'import-history',
      component: () => import('./pages/ImportHistoryPage.vue'),
      meta: { title: 'Import History', category: 'import' }
    },
    { 
      path: '/import/:id', 
      name: 'import-detail',
      component: () => import('./pages/ImportDetailPage.vue'),
      meta: { title: 'Import Details', category: 'import' }
    },
    
    // Plugin management
    { 
      path: '/plugins', 
      name: 'plugins',
      component: () => import('./pages/PluginManagementPage.vue'),
      meta: { title: 'Plugin Management' }
    },
    
    // Metrics and monitoring
    { 
      path: '/metrics', 
      name: 'metrics',
      component: () => import('./pages/MetricsDashboardPage.vue'),
      meta: { title: 'Metrics Dashboard' }
    },
    { 
      path: '/stats', 
      name: 'stats',
      component: () => import('./pages/StatsPage.vue'),
      meta: { title: 'Statistics' }
    },
    
    // Admin
    { 
      path: '/admin', 
      name: 'admin',
      component: () => import('./pages/AdminSettingsPage.vue'),
      meta: { title: 'Admin Settings' }
    },
    
    // 404 handler - must be last
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('./pages/DevicesPage.vue'),
      meta: { title: 'Page Not Found' }
    }
  ]
})

createApp(App)
  .use(pinia)
  .use(router)
  .mount('#app')
