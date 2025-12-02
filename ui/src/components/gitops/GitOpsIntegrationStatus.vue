<template>
  <section class="integration-status" v-if="status">
    <h3>Git Integration Status</h3>
    <div class="status-grid">
      <div class="status-item" :class="{ connected: status.repository_connected }">
        <span class="status-icon">{{ status.repository_connected ? '✅' : '❌' }}</span>
        <span>Repository Connection</span>
      </div>
      <div class="status-item" :class="{ connected: status.branch_exists }">
        <span class="status-icon">{{ status.branch_exists ? '✅' : '❌' }}</span>
        <span>Branch Available</span>
      </div>
      <div class="status-item" :class="{ connected: status.webhook_configured }">
        <span class="status-icon">{{ status.webhook_configured ? '✅' : '❌' }}</span>
        <span>Webhook Configured</span>
      </div>
      <div class="status-item" :class="{ connected: status.ci_status === 'passing', warning: status.ci_status === 'failing' }">
        <span class="status-icon">{{ status.ci_status === 'passing' ? '✅' : status.ci_status === 'failing' ? '❌' : '❓' }}</span>
        <span>CI Status: {{ status.ci_status || 'Unknown' }}</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{ status: { repository_connected?: boolean; branch_exists?: boolean; webhook_configured?: boolean; ci_status?: string } | null }>()
</script>

<style scoped>
.status-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 10px }
.status-item { display: flex; align-items: center; gap: 8px; border: 1px solid #e5e7eb; border-radius: 8px; padding: 10px; background: #fff }
.status-item.connected { border-color: #059669; background: #ecfdf5 }
.status-item.warning { border-color: #f59e0b; background: #fffbeb }
.status-icon { font-size: 1.1rem }
</style>

