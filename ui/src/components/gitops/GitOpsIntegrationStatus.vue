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
      <div class="status-item" :class="{
        connected: status.ci_status === 'passing',
        warning: status.ci_status === 'failing'
      }">
        <span class="status-icon">
          {{ status.ci_status === 'passing' ? '✅' :
             status.ci_status === 'failing' ? '❌' : '❓' }}
        </span>
        <span>CI Status: {{ status.ci_status || 'Unknown' }}</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
interface IntegrationStatus {
  repository_connected: boolean
  branch_exists: boolean
  webhook_configured: boolean
  ci_status?: string
}

defineProps<{
  status: IntegrationStatus | null
}>()
</script>

<style scoped>
.integration-status {
  margin-bottom: 24px;
  padding: 16px;
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
}

.integration-status h3 {
  margin: 0 0 16px 0;
  color: #1f2937;
  font-size: 1rem;
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
}

.status-item.connected {
  border-color: #10b981;
  background: #f0fdf4;
}

.status-item.warning {
  border-color: #f59e0b;
  background: #fffbeb;
}

.status-icon {
  font-size: 1.25rem;
}
</style>
