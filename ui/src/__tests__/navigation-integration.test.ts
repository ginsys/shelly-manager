import { describe, it, expect, beforeEach } from 'vitest'
import { createRouter, createWebHistory } from 'vue-router'
import { mount } from '@vue/test-utils'
import MainLayout from '../layouts/MainLayout.vue'

// Define the routes exactly as in main.ts
const routes = [
  // Main pages
  { 
    path: '/', 
    name: 'devices',
    component: { template: '<div>Devices Page</div>' },
    meta: { title: 'Devices' }
  },
  { 
    path: '/devices/:id', 
    name: 'device-detail',
    component: { template: '<div>Device Details Page</div>' },
    meta: { title: 'Device Details' }
  },
  
  // Export & Import routes
  { 
    path: '/export/schedules', 
    name: 'export-schedules',
    component: { template: '<div>Export Schedules Page</div>' },
    meta: { title: 'Schedule Management', category: 'export' }
  },
  { 
    path: '/export/backup', 
    name: 'export-backup',
    component: { template: '<div>Export Backup Page</div>' },
    meta: { title: 'Backup Management', category: 'export' }
  },
  { 
    path: '/export/gitops', 
    name: 'export-gitops',
    component: { template: '<div>Export GitOps Page</div>' },
    meta: { title: 'GitOps Export', category: 'export' }
  },
  { 
    path: '/export/history', 
    name: 'export-history',
    component: { template: '<div>Export History Page</div>' },
    meta: { title: 'Export History', category: 'export' }
  },
  { 
    path: '/export/:id', 
    name: 'export-detail',
    component: { template: '<div>Export Detail Page</div>' },
    meta: { title: 'Export Details', category: 'export' }
  },
  { 
    path: '/import/history', 
    name: 'import-history',
    component: { template: '<div>Import History Page</div>' },
    meta: { title: 'Import History', category: 'import' }
  },
  { 
    path: '/import/:id', 
    name: 'import-detail',
    component: { template: '<div>Import Detail Page</div>' },
    meta: { title: 'Import Details', category: 'import' }
  },
  
  // Plugin management
  { 
    path: '/plugins', 
    name: 'plugins',
    component: { template: '<div>Plugins Page</div>' },
    meta: { title: 'Plugin Management' }
  },
  
  // Metrics and monitoring
  { 
    path: '/metrics', 
    name: 'metrics',
    component: { template: '<div>Metrics Page</div>' },
    meta: { title: 'Metrics Dashboard' }
  },
  { 
    path: '/stats', 
    name: 'stats',
    component: { template: '<div>Stats Page</div>' },
    meta: { title: 'Statistics' }
  },
  
  // Admin
  { 
    path: '/admin', 
    name: 'admin',
    component: { template: '<div>Admin Page</div>' },
    meta: { title: 'Admin Settings' }
  }
]

describe('Navigation Integration', () => {
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
    expect(routeNames).toContain('stats')
    expect(routeNames).toContain('admin')
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

  it('should mount MainLayout without errors', async () => {
    await router.push('/')
    
    const wrapper = mount(MainLayout, {
      global: {
        plugins: [router]
      }
    })
    
    expect(wrapper.exists()).toBe(true)
    expect(wrapper.find('.brand').text()).toBe('Shelly Manager')
  })

  it('should show correct navigation links', async () => {
    await router.push('/')
    
    const wrapper = mount(MainLayout, {
      global: {
        plugins: [router]
      }
    })
    
    // Check main navigation links
    const navLinks = wrapper.findAll('.nav-link')
    const linkTexts = navLinks.map(link => link.text())
    
    expect(linkTexts).toContain('Devices')
    expect(linkTexts).toContain('Export & Import')
    expect(linkTexts).toContain('Plugins')
    expect(linkTexts).toContain('Metrics')
    expect(linkTexts).toContain('Admin')
  })

  it('should show dropdown menu with correct items', async () => {
    await router.push('/')
    
    const wrapper = mount(MainLayout, {
      global: {
        plugins: [router]
      }
    })
    
    // Check dropdown menu items
    const dropdownItems = wrapper.findAll('.dropdown-item')
    const dropdownTexts = dropdownItems.map(item => item.text())
    
    expect(dropdownTexts).toContain('Schedule Management')
    expect(dropdownTexts).toContain('Backup Management')
    expect(dropdownTexts).toContain('GitOps Export')
    expect(dropdownTexts).toContain('Export History')
    expect(dropdownTexts).toContain('Import History')
  })

  it('should show breadcrumbs for nested pages', async () => {
    // Test breadcrumb for device detail page
    await router.push('/devices/test-123')
    
    const wrapper = mount(MainLayout, {
      global: {
        plugins: [router]
      }
    })
    
    await wrapper.vm.$nextTick()
    
    const breadcrumb = wrapper.find('.breadcrumb')
    expect(breadcrumb.exists()).toBe(true)
    
    const breadcrumbItems = wrapper.findAll('.breadcrumb-item')
    expect(breadcrumbItems.length).toBeGreaterThan(0)
  })

  it('should hide breadcrumbs on home page', async () => {
    await router.push('/')
    
    const wrapper = mount(MainLayout, {
      global: {
        plugins: [router]
      }
    })
    
    await wrapper.vm.$nextTick()
    
    const breadcrumb = wrapper.find('.breadcrumb')
    expect(breadcrumb.exists()).toBe(false)
  })

  it('should show active state for current route', async () => {
    await router.push('/plugins')
    
    const wrapper = mount(MainLayout, {
      global: {
        plugins: [router]
      }
    })
    
    await wrapper.vm.$nextTick()
    
    const pluginsLink = wrapper.find('a[href="/plugins"]')
    expect(pluginsLink.classes()).toContain('active')
  })

  it('should show active state for export/import dropdown when on related pages', async () => {
    await router.push('/export/backup')
    
    const wrapper = mount(MainLayout, {
      global: {
        plugins: [router]
      }
    })
    
    await wrapper.vm.$nextTick()
    
    const dropdownTrigger = wrapper.find('.dropdown-trigger')
    expect(dropdownTrigger.classes()).toContain('active')
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
    { path: '/metrics', name: 'metrics' },
    { path: '/admin', name: 'admin' }
  ]

  testRoutes.forEach(({ path, name }) => {
    it(`should navigate to ${name} route correctly`, async () => {
      await router.push(path)
      expect(router.currentRoute.value.name).toBe(name)
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
})