<template>
  <div class="layout-root">
    <header class="topbar">
      <div class="brand">Shelly Manager</div>
      <nav class="nav">
        <router-link class="nav-link" to="/">Devices</router-link>
        <!-- Future: <router-link to="/metrics" class="nav-link">Metrics</router-link> -->
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
</style>
