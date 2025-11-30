<template>
  <div class="layout-root" data-testid="app">
    <header class="topbar" data-testid="header">
      <div class="brand" data-testid="brand">Shelly Manager</div>
      <nav class="nav" data-testid="navigation">
        <router-link 
          class="nav-link" 
          to="/"
          :class="{ active: $route.name === 'devices' }"
        >
          Devices
        </router-link>
        
        <div class="nav-dropdown">
          <span 
            class="nav-link dropdown-trigger"
            :class="{ active: $route.meta.category === 'export' || $route.meta.category === 'import' }"
          >
            Export & Import
          </span>
          <div class="dropdown-menu">
            <div class="dropdown-section">
              <div class="dropdown-section-title">Export</div>
              <router-link class="dropdown-item" to="/export/schedules">
                <span class="dropdown-icon">📅</span>
                Schedule Management
              </router-link>
              <router-link class="dropdown-item" to="/export/backup">
                <span class="dropdown-icon">💾</span>
                Backup Management
              </router-link>
              <router-link class="dropdown-item" to="/export/gitops">
                <span class="dropdown-icon">🔄</span>
                GitOps Export
              </router-link>
              <router-link class="dropdown-item" to="/export/history">
                <span class="dropdown-icon">📋</span>
                Export History
              </router-link>
            </div>
            <div class="dropdown-divider"></div>
            <div class="dropdown-section">
              <div class="dropdown-section-title">Import</div>
              <router-link class="dropdown-item" to="/import/history">
                <span class="dropdown-icon">📥</span>
                Import History
              </router-link>
            </div>
          </div>
        </div>
        
        <router-link 
          class="nav-link" 
          to="/plugins"
          :class="{ active: $route.name === 'plugins' }"
        >
          Plugins
        </router-link>
        
        <router-link 
          class="nav-link" 
          to="/dashboard"
          :class="{ active: $route.name === 'metrics' || $route.name === 'stats' }"
        >
          Metrics
        </router-link>
        
        <router-link 
          class="nav-link" 
          to="/admin"
          :class="{ active: $route.name === 'admin' }"
        >
          Admin
        </router-link>
      </nav>
    </header>
    
    <!-- Breadcrumb Navigation -->
    <nav class="breadcrumb" v-if="showBreadcrumb">
      <div class="breadcrumb-container">
        <router-link class="breadcrumb-item" to="/">
          <span class="breadcrumb-icon">🏠</span>
          Home
        </router-link>
        <template v-for="(crumb, index) in breadcrumbs" :key="index">
          <span class="breadcrumb-separator">›</span>
          <router-link 
            v-if="crumb.to" 
            class="breadcrumb-item" 
            :to="crumb.to"
          >
            {{ crumb.text }}
          </router-link>
          <span v-else class="breadcrumb-item current">
            {{ crumb.text }}
          </span>
        </template>
      </div>
    </nav>
    
    <main class="content" data-testid="main-content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()

// Breadcrumb configuration
const breadcrumbs = computed(() => {
  const crumbs = []
  
  switch (route.name) {
    case 'device-detail':
      crumbs.push(
        { text: 'Devices', to: '/' },
        { text: 'Device Details' }
      )
      break
      
    case 'export-schedules':
      crumbs.push(
        { text: 'Export & Import', to: null },
        { text: 'Schedule Management' }
      )
      break
      
    case 'export-backup':
      crumbs.push(
        { text: 'Export & Import', to: null },
        { text: 'Backup Management' }
      )
      break
      
    case 'export-gitops':
      crumbs.push(
        { text: 'Export & Import', to: null },
        { text: 'GitOps Export' }
      )
      break
      
    case 'export-history':
      crumbs.push(
        { text: 'Export & Import', to: null },
        { text: 'Export History' }
      )
      break
      
    case 'export-detail':
      crumbs.push(
        { text: 'Export & Import', to: null },
        { text: 'Export History', to: '/export/history' },
        { text: 'Export Details' }
      )
      break
      
    case 'import-history':
      crumbs.push(
        { text: 'Export & Import', to: null },
        { text: 'Import History' }
      )
      break
      
    case 'import-detail':
      crumbs.push(
        { text: 'Export & Import', to: null },
        { text: 'Import History', to: '/import/history' },
        { text: 'Import Details' }
      )
      break
      
    case 'plugins':
      crumbs.push({ text: 'Plugin Management' })
      break
      
    case 'metrics':
      crumbs.push({ text: 'Metrics Dashboard' })
      break
      
    case 'stats':
      crumbs.push({ text: 'Statistics' })
      break
      
    case 'admin':
      crumbs.push({ text: 'Admin Settings' })
      break
  }
  
  return crumbs
})

const showBreadcrumb = computed(() => {
  return route.name !== 'devices' && breadcrumbs.value.length > 0
})
</script>

<style scoped>
/* Layout */
.layout-root { 
  display: flex; 
  flex-direction: column; 
  min-height: 100vh; 
}

/* Header */
.topbar { 
  height: 56px; 
  display: flex; 
  align-items: center; 
  padding: 0 16px; 
  background: #1f2937; 
  color: #fff; 
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

.brand { 
  font-weight: 600; 
  margin-right: 24px; 
  font-size: 18px;
  flex-shrink: 0;
  white-space: nowrap;
  min-width: fit-content;
}

/* Navigation */
.nav { 
  display: flex; 
  gap: 4px; 
  align-items: center;
}

.nav-link { 
  color: #cbd5e1; 
  text-decoration: none; 
  font-size: 14px; 
  font-weight: 500;
  padding: 8px 16px;
  border-radius: 6px;
  transition: all 0.2s ease;
  position: relative;
}

.nav-link:hover, 
.nav-link.active { 
  color: #fff; 
  background: rgba(255, 255, 255, 0.1);
}

.nav-link.router-link-active {
  color: #60a5fa;
  background: rgba(96, 165, 250, 0.1);
}

/* Dropdown Navigation */
.nav-dropdown {
  position: relative;
  display: inline-block;
}

.dropdown-trigger {
  cursor: pointer;
  display: flex;
  align-items: center;
  user-select: none;
}

.dropdown-trigger:after {
  content: ' ▼';
  font-size: 10px;
  margin-left: 4px;
  transition: transform 0.2s ease;
}

.nav-dropdown:hover .dropdown-trigger:after {
  transform: rotate(180deg);
}

.dropdown-menu {
  display: none;
  position: absolute;
  top: 100%;
  left: 0;
  min-width: 220px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.15);
  z-index: 1000;
  margin-top: 4px;
  overflow: hidden;
}

/* Invisible bridge to maintain hover state */
.dropdown-menu::before {
  content: '';
  position: absolute;
  top: -8px;
  left: 0;
  right: 0;
  height: 8px;
  background: transparent;
}

.nav-dropdown:hover .dropdown-menu,
.dropdown-menu:hover {
  display: block;
  animation: fadeIn 0.15s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(-8px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Dropdown Sections */
.dropdown-section {
  padding: 8px 0;
}

.dropdown-section-title {
  font-size: 11px;
  font-weight: 600;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 4px 16px;
  margin-bottom: 4px;
}

.dropdown-divider {
  height: 1px;
  background: #f3f4f6;
  margin: 4px 0;
}

.dropdown-item {
  display: flex;
  align-items: center;
  padding: 10px 16px;
  color: #374151;
  text-decoration: none;
  font-size: 14px;
  transition: all 0.15s ease;
}

.dropdown-item:hover {
  background: #f8fafc;
  color: #1f2937;
}

.dropdown-item.router-link-active {
  background: #eff6ff;
  color: #2563eb;
  font-weight: 500;
}

.dropdown-icon {
  margin-right: 8px;
  font-size: 16px;
  width: 20px;
  text-align: center;
}

/* Breadcrumb Navigation */
.breadcrumb {
  background: #f9fafb;
  border-bottom: 1px solid #e5e7eb;
  padding: 12px 0;
  font-size: 14px;
}

.breadcrumb-container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 16px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
}

.breadcrumb-item {
  color: #6b7280;
  text-decoration: none;
  display: flex;
  align-items: center;
  transition: color 0.2s ease;
}

.breadcrumb-item:hover {
  color: #374151;
}

.breadcrumb-item.current {
  color: #1f2937;
  font-weight: 500;
}

.breadcrumb-icon {
  margin-right: 4px;
  font-size: 12px;
}

.breadcrumb-separator {
  color: #d1d5db;
  margin: 0 6px;
  font-weight: 300;
}

/* Main Content */
.content { 
  flex: 1; 
  padding: 24px 16px; 
  background: #f8fafc;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
  box-sizing: border-box;
}

/* Responsive Design */
@media (max-width: 768px) {
  .topbar {
    padding: 0 12px;
  }
  
  .brand {
    margin-right: 16px;
    font-size: 16px;
  }
  
  .nav {
    gap: 2px;
  }
  
  .nav-link {
    padding: 6px 12px;
    font-size: 13px;
  }
  
  .dropdown-menu {
    min-width: 200px;
    right: 0;
    left: auto;
  }
  
  .content {
    padding: 16px 12px;
  }
  
  .breadcrumb-container {
    padding: 0 12px;
    font-size: 13px;
  }
}

@media (max-width: 640px) {
  .nav-link:not(.dropdown-trigger) span {
    display: none;
  }
  
  .nav-link:not(.dropdown-trigger):after {
    content: attr(data-mobile-label);
  }
  
  /* Show only icons on very small screens */
  .nav-link[to="/"] { min-width: 32px; }
  .nav-link[to="/plugins"] { min-width: 32px; }
  .nav-link[to="/dashboard"] { min-width: 32px; }
  .nav-link[to="/admin"] { min-width: 32px; }
}

/* Dark mode support */
@media (prefers-color-scheme: dark) {
  .breadcrumb {
    background: #1f2937;
    border-bottom-color: #374151;
  }
  
  .breadcrumb-item {
    color: #9ca3af;
  }
  
  .breadcrumb-item:hover {
    color: #d1d5db;
  }
  
  .breadcrumb-item.current {
    color: #f9fafb;
  }
  
  .breadcrumb-separator {
    color: #6b7280;
  }
}
</style>
