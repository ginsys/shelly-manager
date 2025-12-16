import { describe, it, expect, beforeEach } from 'vitest'
import { createRouter, createWebHistory } from 'vue-router'

// Define the routes exactly as in main.ts
const routes = [
  // Main pages
  { 
    path: '/', 
    name: 'devices',
    component: () => Promise.resolve({ template: '<div>Devices Page</div>' }),
    meta: { title: 'Devices' }
  },
  { 
    path: '/devices/:id', 
    name: 'device-detail',
    component: () => Promise.resolve({ template: '<div>Device Details Page</div>' }),
    meta: { title: 'Device Details' }
  },
  
  // Export & Import routes
  { 
    path: '/export/schedules', 
    name: 'export-schedules',
    component: () => Promise.resolve({ template: '<div>Export Schedules Page</div>' }),
    meta: { title: 'Schedule Management', category: 'export' }
  },
  { 
    path: '/export/backup', 
    name: 'export-backup',
    component: () => Promise.resolve({ template: '<div>Export Backup Page</div>' }),
    meta: { title: 'Backup Management', category: 'export' }
  },
  { 
    path: '/export/gitops', 
    name: 'export-gitops',
    component: () => Promise.resolve({ template: '<div>Export GitOps Page</div>' }),
    meta: { title: 'GitOps Export', category: 'export' }
  },
  { 
    path: '/export/history', 
    name: 'export-history',
    component: () => Promise.resolve({ template: '<div>Export History Page</div>' }),
    meta: { title: 'Export History', category: 'export' }
  },
  { 
    path: '/export/:id', 
    name: 'export-detail',
    component: () => Promise.resolve({ template: '<div>Export Detail Page</div>' }),
    meta: { title: 'Export Details', category: 'export' }
  },
  { 
    path: '/import/history', 
    name: 'import-history',
    component: () => Promise.resolve({ template: '<div>Import History Page</div>' }),
    meta: { title: 'Import History', category: 'import' }
  },
  { 
    path: '/import/:id', 
    name: 'import-detail',
    component: () => Promise.resolve({ template: '<div>Import Detail Page</div>' }),
    meta: { title: 'Import Details', category: 'import' }
  },
  
  // Plugin management
  { 
    path: '/plugins', 
    name: 'plugins',
    component: () => Promise.resolve({ template: '<div>Plugins Page</div>' }),
    meta: { title: 'Plugin Management' }
  },
  
  // Metrics and monitoring
  {
    path: '/dashboard',
    name: 'metrics',
    component: () => Promise.resolve({ template: '<div>Metrics Page</div>' }),
    meta: { title: 'Metrics Dashboard' }
  },

  // Admin
  { 
    path: '/admin', 
    name: 'admin',
    component: () => Promise.resolve({ template: '<div>Admin Page</div>' }),
    meta: { title: 'Admin Settings' }
  },
  
  // 404 handler
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => Promise.resolve({ template: '<div>Not Found</div>' }),
    meta: { title: 'Page Not Found' }
  }
]

describe('Navigation Routes Configuration', () => {
  let router: any
  
  beforeEach(() => {
    router = createRouter({
      history: createWebHistory(),
      routes
    })
  })

  it('should have all required routes configured', () => {
    const routeNames = routes.map(r => r.name)
    
    // Main navigation routes
    expect(routeNames).toContain('devices')
    expect(routeNames).toContain('device-detail')
    
    // Export routes
    expect(routeNames).toContain('export-schedules')
    expect(routeNames).toContain('export-backup')
    expect(routeNames).toContain('export-gitops')
    expect(routeNames).toContain('export-history')
    expect(routeNames).toContain('export-detail')
    
    // Import routes
    expect(routeNames).toContain('import-history')
    expect(routeNames).toContain('import-detail')
    
    // Other routes
    expect(routeNames).toContain('plugins')
    expect(routeNames).toContain('metrics')
    expect(routeNames).toContain('admin')
    expect(routeNames).toContain('not-found')
  })

  it('should have correct meta information for route categories', () => {
    const exportRoutes = routes.filter(r => r.meta?.category === 'export')
    const importRoutes = routes.filter(r => r.meta?.category === 'import')
    
    expect(exportRoutes).toHaveLength(5) // schedules, backup, gitops, history, detail
    expect(importRoutes).toHaveLength(2) // history, detail
    
    // Check specific export routes
    const scheduleRoute = routes.find(r => r.name === 'export-schedules')
    expect(scheduleRoute?.meta?.title).toBe('Schedule Management')
    
    const backupRoute = routes.find(r => r.name === 'export-backup')
    expect(backupRoute?.meta?.title).toBe('Backup Management')
    
    const gitopsRoute = routes.find(r => r.name === 'export-gitops')
    expect(gitopsRoute?.meta?.title).toBe('GitOps Export')
  })

  // Test individual route navigation
  const testRoutes = [
    { path: '/', name: 'devices' },
    { path: '/export/schedules', name: 'export-schedules' },
    { path: '/export/backup', name: 'export-backup' },
    { path: '/export/gitops', name: 'export-gitops' },
    { path: '/export/history', name: 'export-history' },
    { path: '/import/history', name: 'import-history' },
    { path: '/plugins', name: 'plugins' },
    { path: '/dashboard', name: 'metrics' },
    { path: '/admin', name: 'admin' }
  ]

  testRoutes.forEach(({ path, name }) => {
    it(`should navigate to ${name} route correctly`, async () => {
      await router.push(path)
      expect(router.currentRoute.value.name).toBe(name)
      expect(router.currentRoute.value.path).toBe(path)
    })
  })

  it('should handle parameterized routes correctly', async () => {
    await router.push('/devices/shelly-plus-1-abc123')
    expect(router.currentRoute.value.name).toBe('device-detail')
    expect(router.currentRoute.value.params.id).toBe('shelly-plus-1-abc123')

    await router.push('/export/export-456')
    expect(router.currentRoute.value.name).toBe('export-detail')
    expect(router.currentRoute.value.params.id).toBe('export-456')

    await router.push('/import/import-789')
    expect(router.currentRoute.value.name).toBe('import-detail')
    expect(router.currentRoute.value.params.id).toBe('import-789')
  })

  it('should handle 404 routes correctly', async () => {
    await router.push('/non-existent-route')
    expect(router.currentRoute.value.name).toBe('not-found')
  })

  it('should have correct path patterns', () => {
    const pathPatterns = routes.map(r => r.path)
    
    // Verify path patterns are correct
    expect(pathPatterns).toContain('/')
    expect(pathPatterns).toContain('/devices/:id')
    expect(pathPatterns).toContain('/export/schedules')
    expect(pathPatterns).toContain('/export/backup')
    expect(pathPatterns).toContain('/export/gitops')
    expect(pathPatterns).toContain('/export/history')
    expect(pathPatterns).toContain('/export/:id')
    expect(pathPatterns).toContain('/import/history')
    expect(pathPatterns).toContain('/import/:id')
    expect(pathPatterns).toContain('/plugins')
    expect(pathPatterns).toContain('/dashboard')
    expect(pathPatterns).toContain('/admin')
    expect(pathPatterns).toContain('/:pathMatch(.*)*')
  })

  it('should categorize routes correctly for navigation menu', () => {
    const mainRoutes = ['devices', 'plugins', 'metrics', 'admin']
    const exportRoutes = ['export-schedules', 'export-backup', 'export-gitops', 'export-history', 'export-detail']
    const importRoutes = ['import-history', 'import-detail']
    const detailRoutes = ['device-detail', 'export-detail', 'import-detail']
    
    mainRoutes.forEach(name => {
      const route = routes.find(r => r.name === name)
      expect(route).toBeDefined()
    })
    
    exportRoutes.forEach(name => {
      const route = routes.find(r => r.name === name)
      expect(route?.meta?.category).toBe('export')
    })
    
    importRoutes.forEach(name => {
      const route = routes.find(r => r.name === name)
      expect(route?.meta?.category).toBe('import')
    })
    
    detailRoutes.forEach(name => {
      const route = routes.find(r => r.name === name)
      expect(route?.path).toContain(':id')
    })
  })
})
