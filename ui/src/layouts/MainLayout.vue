<template>
  <div class="layout-root" data-testid="app">
    <header class="topbar" data-testid="header">
      <div class="brand" data-testid="brand">Shelly Manager</div>
      <nav class="nav" data-testid="navigation">
        <router-link
          class="nav-link"
          to="/"
          :class="{ active: $route.name === 'devices' }"
          :aria-current="$route.name === 'devices' ? 'page' : undefined"
        >
          Devices
        </router-link>
        
        <router-link
          class="nav-link"
          to="/templates"
          :class="{ active: $route.meta.category === 'templates' }"
          :aria-current="$route.meta.category === 'templates' ? 'page' : undefined"
        >
          Templates
        </router-link>

        <NavMenu
          v-for="menu in navMenus"
          :key="menu.label"
          :label="menu.label"
          :sections="menu.sections"
          :active="menu.categories.includes($route.meta.category as string)"
        />

        <router-link
          class="nav-link"
          to="/plugins"
          :class="{ active: $route.name === 'plugins' }"
          :aria-current="$route.name === 'plugins' ? 'page' : undefined"
        >
          Plugins
        </router-link>

        <router-link
          class="nav-link"
          to="/dashboard"
          :class="{ active: $route.name === 'metrics' || $route.name === 'stats' }"
          :aria-current="$route.name === 'metrics' || $route.name === 'stats' ? 'page' : undefined"
        >
          Metrics
        </router-link>

        <router-link
          class="nav-link"
          to="/admin"
          :class="{ active: $route.name === 'admin' }"
          :aria-current="$route.name === 'admin' ? 'page' : undefined"
        >
          Admin
        </router-link>
      </nav>
      <div class="key-actions">
        <button
          type="button"
          class="key-btn"
          data-testid="admin-key-btn"
          @click="openAdminKeyPrompt"
        >
          Admin key
        </button>
        <button
          v-if="hasSessionKey"
          type="button"
          class="key-btn"
          data-testid="admin-key-forget-btn"
          @click="forgetAdminKey"
        >
          Forget key
        </button>
      </div>
    </header>
    
    <!-- Breadcrumb Navigation -->
    <nav class="breadcrumb" v-if="showBreadcrumb" aria-label="Breadcrumb">
      <div class="breadcrumb-container">
        <router-link class="breadcrumb-item" to="/">
          <span class="breadcrumb-icon" aria-hidden="true">🏠</span>
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

    <AppNotice />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { hasSessionKey, openAdminKeyPrompt, forgetAdminKey } from '@/api/adminKey'
import NavMenu, { type NavMenuSection } from '@/components/NavMenu.vue'
import AppNotice from '@/components/shared/AppNotice.vue'

const route = useRoute()

// One menu per area. `categories` are the route.meta.category values that highlight the menu.
// Detail pages (`:id`) are not listed: they are reached from their list page.
const navMenus: { label: string; categories: string[]; sections: NavMenuSection[] }[] = [
  {
    label: 'Drift',
    categories: ['drift'],
    sections: [
      {
        items: [
          { label: 'Schedules', to: '/drift/schedules', icon: '🗓️' },
          { label: 'Reports', to: '/drift/reports', icon: '📊' },
          { label: 'Trends', to: '/drift/trends', icon: '📈' },
        ],
      },
    ],
  },
  {
    label: 'Notifications',
    categories: ['notifications'],
    sections: [
      {
        items: [
          { label: 'Channels', to: '/notifications/channels', icon: '📡' },
          { label: 'Rules', to: '/notifications/rules', icon: '📏' },
          { label: 'History', to: '/notifications/history', icon: '🕘' },
        ],
      },
    ],
  },
  {
    label: 'Provisioning',
    categories: ['provisioning'],
    sections: [
      {
        items: [
          { label: 'Dashboard', to: '/provisioning', icon: '🧭' },
          { label: 'Tasks', to: '/provisioning/tasks', icon: '🛠️' },
          { label: 'Agents', to: '/provisioning/agents', icon: '🤖' },
        ],
      },
    ],
  },
  {
    label: 'Export & Import',
    categories: ['export', 'import'],
    sections: [
      {
        title: 'Export',
        items: [
          { label: 'Backup Management', to: '/export/backup', icon: '💾' },
          { label: 'GitOps Export', to: '/export/gitops', icon: '🔄' },
          { label: 'Export History', to: '/export/history', icon: '📋' },
        ],
      },
      {
        title: 'Import',
        items: [{ label: 'Import History', to: '/import/history', icon: '📥' }],
      },
    ],
  },
]

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

/* :deep, because the menu triggers are rendered by NavMenu and carry the same link look */
.nav :deep(.nav-link) {
  color: #cbd5e1;
  text-decoration: none;
  font-size: 14px;
  font-weight: 500;
  padding: 8px 16px;
  border-radius: 6px;
  transition: all 0.2s ease;
  position: relative;
}

.nav :deep(.nav-link:hover),
.nav :deep(.nav-link.active) {
  color: #fff;
  background: rgba(255, 255, 255, 0.1);
}

.nav :deep(.nav-link.router-link-active) {
  color: #60a5fa;
  background: rgba(96, 165, 250, 0.1);
}

/* Admin key actions, pushed to the right end of the header */
.key-actions {
  margin-left: auto;
  display: flex;
  gap: 8px;
}

.key-btn {
  background: none;
  border: 1px solid rgba(203, 213, 225, 0.4);
  border-radius: 6px;
  color: #cbd5e1;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  padding: 6px 12px;
}

.key-btn:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.1);
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
  
  .nav :deep(.nav-link) {
    padding: 6px 12px;
    font-size: 13px;
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
