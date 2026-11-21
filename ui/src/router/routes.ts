import type { RouteRecordRaw } from 'vue-router'

// The route table. `main.ts` builds the router from it, and the unit tests read it, so a route cannot
// be added in one place and missed in the other. A route with no `:` in its path must be linked from
// the top bar (`layouts/MainLayout.vue`) or from another page: `__tests__/route-reachability.test.ts`
// fails when it is not.
export const routes: RouteRecordRaw[] = [
  // Main pages
  {
    path: '/',
    name: 'devices',
    component: () => import('../pages/DevicesPage.vue'),
    meta: { title: 'Devices' }
  },
  {
    path: '/devices/:id',
    name: 'device-detail',
    component: () => import('../pages/DeviceDetailPage.vue'),
    meta: { title: 'Device Details' }
  },
  {
    path: '/devices/:id/config',
    name: 'device-config',
    component: () => import('../pages/DeviceConfigPage.vue'),
    meta: { title: 'Device Configuration' }
  },
  {
    path: '/devices/:id/config/history',
    name: 'device-config-history',
    component: () => import('../pages/DeviceConfigHistoryPage.vue'),
    meta: { title: 'Configuration History' }
  },

  // Configuration Templates
  {
    path: '/templates',
    name: 'templates',
    component: () => import('../pages/TemplatesPage.vue'),
    meta: { title: 'Configuration Templates', category: 'templates' }
  },

  {
    path: '/templates/common-settings',
    name: 'template-common-settings',
    component: () => import('../pages/CommonSettingsPage.vue'),
    meta: { title: 'Common Settings', category: 'templates' }
  },

  {
    path: '/templates/:id',
    name: 'template-detail',
    component: () => import('../pages/TemplateDetailPage.vue'),
    meta: { title: 'Template Details', category: 'templates' }
  },

  // Drift Detection
  {
    path: '/drift/schedules',
    name: 'drift-schedules',
    component: () => import('../pages/DriftSchedulesPage.vue'),
    meta: { title: 'Drift Detection Schedules', category: 'drift' }
  },
  {
    path: '/drift/reports',
    name: 'drift-reports',
    component: () => import('../pages/DriftReportsPage.vue'),
    meta: { title: 'Drift Reports', category: 'drift' }
  },
  {
    path: '/drift/trends',
    name: 'drift-trends',
    component: () => import('../pages/DriftTrendsPage.vue'),
    meta: { title: 'Drift Trends', category: 'drift' }
  },

  // Export & Import routes
  // Consolidated: content exports are integrated into Backups & Exports page
  {
    path: '/export/backup',
    name: 'export-backup',
    component: () => import('../pages/BackupManagementPage.vue'),
    meta: { title: 'Backup Management', category: 'export' }
  },
  {
    path: '/export/gitops',
    name: 'export-gitops',
    component: () => import('../pages/GitOpsExportPage.vue'),
    meta: { title: 'GitOps Export', category: 'export' }
  },
  {
    path: '/export/history',
    name: 'export-history',
    component: () => import('../pages/ExportHistoryPage.vue'),
    meta: { title: 'Export History', category: 'export' }
  },
  {
    path: '/export/:id',
    name: 'export-detail',
    component: () => import('../pages/ExportDetailPage.vue'),
    meta: { title: 'Export Details', category: 'export' }
  },
  {
    path: '/import/history',
    name: 'import-history',
    component: () => import('../pages/ImportHistoryPage.vue'),
    meta: { title: 'Import History', category: 'import' }
  },
  {
    path: '/import/:id',
    name: 'import-detail',
    component: () => import('../pages/ImportDetailPage.vue'),
    meta: { title: 'Import Details', category: 'import' }
  },

  // Plugin management
  {
    path: '/plugins',
    name: 'plugins',
    component: () => import('../pages/PluginManagementPage.vue'),
    meta: { title: 'Plugin Management' }
  },

  // Metrics and monitoring
  {
    path: '/dashboard',
    name: 'metrics',
    component: () => import('../pages/MetricsDashboardPage.vue'),
    meta: { title: 'Metrics Dashboard' }
  },

  // Admin
  {
    path: '/admin',
    name: 'admin',
    component: () => import('../pages/AdminSettingsPage.vue'),
    meta: { title: 'Admin Settings' }
  },

  // Notifications
  {
    path: '/notifications/channels',
    name: 'notification-channels',
    component: () => import('../pages/NotificationChannelsPage.vue'),
    meta: { title: 'Notification Channels', category: 'notifications' }
  },
  {
    path: '/notifications/channels/:id',
    name: 'notification-channel-detail',
    component: () => import('../pages/NotificationChannelDetailPage.vue'),
    meta: { title: 'Channel Details', category: 'notifications' }
  },
  {
    path: '/notifications/rules',
    name: 'notification-rules',
    component: () => import('../pages/NotificationRulesPage.vue'),
    meta: { title: 'Notification Rules', category: 'notifications' }
  },
  {
    path: '/notifications/history',
    name: 'notification-history',
    component: () => import('../pages/NotificationHistoryPage.vue'),
    meta: { title: 'Notification History', category: 'notifications' }
  },

  // Provisioning
  {
    path: '/provisioning',
    name: 'provisioning-dashboard',
    component: () => import('../pages/ProvisioningDashboardPage.vue'),
    meta: { title: 'Provisioning Dashboard', category: 'provisioning' }
  },
  {
    path: '/provisioning/tasks',
    name: 'provisioning-tasks',
    component: () => import('../pages/ProvisioningTasksPage.vue'),
    meta: { title: 'Provisioning Tasks', category: 'provisioning' }
  },
  {
    path: '/provisioning/tasks/:id',
    name: 'provisioning-task-detail',
    component: () => import('../pages/ProvisioningTaskDetailPage.vue'),
    meta: { title: 'Task Details', category: 'provisioning' }
  },
  {
    path: '/provisioning/agents',
    name: 'provisioning-agents',
    component: () => import('../pages/ProvisioningAgentsPage.vue'),
    meta: { title: 'Provisioning Agents', category: 'provisioning' }
  },

  // 404 handler - must be last
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('../pages/DevicesPage.vue'),
    meta: { title: 'Page Not Found' }
  }
]
