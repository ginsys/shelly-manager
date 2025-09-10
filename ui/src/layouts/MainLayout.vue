<template>
  <div class="layout-root">
    <header class="topbar">
      <div class="brand">Shelly Manager</div>
      <nav class="nav">
        <router-link class="nav-link" to="/">Devices</router-link>
        <div class="nav-dropdown">
          <span class="nav-link dropdown-trigger">Export & Import</span>
          <div class="dropdown-menu">
            <router-link class="dropdown-item" to="/export/backup">Backup Management</router-link>
            <router-link class="dropdown-item" to="/export/gitops">GitOps Export</router-link>
            <router-link class="dropdown-item" to="/export/history">Export History</router-link>
            <router-link class="dropdown-item" to="/import/history">Import History</router-link>
          </div>
        </div>
        <router-link class="nav-link" to="/metrics">Metrics</router-link>
        <router-link class="nav-link" to="/admin">Admin</router-link>
      </nav>
    </header>
    <main class="content">
      <router-view />
    </main>
  </div>
  
</template>

<script setup lang="ts">
// Placeholder layout; when Quasar is installed, replace with QLayout/QHeader/QPageContainer
</script>

<style scoped>
.layout-root { display: flex; flex-direction: column; min-height: 100vh; }
.topbar { height: 56px; display: flex; align-items: center; padding: 0 16px; background: #1f2937; color: #fff; }
.brand { font-weight: 600; margin-right: 24px; }
.nav { display: flex; gap: 12px; }
.nav-link { color: #cbd5e1; text-decoration: none; font-size: 14px; }
.nav-link:hover { color: #fff; }
.content { flex: 1; padding: 16px; background: #f8fafc; }

/* Dropdown styles */
.nav-dropdown {
  position: relative;
  display: inline-block;
}

.dropdown-trigger {
  cursor: pointer;
  display: flex;
  align-items: center;
}

.dropdown-trigger:after {
  content: ' ▼';
  font-size: 10px;
  margin-left: 4px;
}

.dropdown-menu {
  display: none;
  position: absolute;
  top: 100%;
  left: 0;
  min-width: 180px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  z-index: 1000;
  margin-top: 4px;
}

.nav-dropdown:hover .dropdown-menu {
  display: block;
}

.dropdown-item {
  display: block;
  padding: 8px 16px;
  color: #374151;
  text-decoration: none;
  font-size: 14px;
  border-bottom: 1px solid #f3f4f6;
}

.dropdown-item:last-child {
  border-bottom: none;
}

.dropdown-item:hover {
  background: #f3f4f6;
  color: #1f2937;
}
</style>
