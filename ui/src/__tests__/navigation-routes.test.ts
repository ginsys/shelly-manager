import { describe, it, expect, beforeEach } from 'vitest'
import { createRouter, createWebHistory } from 'vue-router'
import { routes as appRoutes } from '@/router/routes'

// The real route table, the one `main.ts` builds the router from. Every page is lazy-loaded, and a
// navigation would load the whole page and its API modules, so each route gets one stub component.
// Paths, names and meta stay the table's own.
const stub = { template: '<div>page</div>' }
const routes = appRoutes.map(route => ({ ...route, component: stub }))

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
    expect(routeNames).toContain('template-common-settings')

    // Export routes
    expect(routeNames).not.toContain('export-schedules')
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

    expect(exportRoutes).toHaveLength(4)
    expect(importRoutes).toHaveLength(2) // history, detail

    // Check specific export routes
    const backupRoute = routes.find(r => r.name === 'export-backup')
    expect(backupRoute?.meta?.title).toBe('Backup Management')

    const gitopsRoute = routes.find(r => r.name === 'export-gitops')
    expect(gitopsRoute?.meta?.title).toBe('GitOps Export')
  })

  // Test individual route navigation
  const testRoutes = [
    { path: '/', name: 'devices' },
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
    expect(pathPatterns).not.toContain('/export/schedules')
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
    const exportRoutes = ['export-backup', 'export-gitops', 'export-history', 'export-detail']
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
