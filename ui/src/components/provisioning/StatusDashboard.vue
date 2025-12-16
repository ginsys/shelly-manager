<template>
  <section class="status-dashboard">
    <h2>Provisioning Status</h2>

    <div class="status-grid">
      <div class="status-card">
        <div class="status-label">Total Tasks</div>
        <div class="status-value">{{ store.tasks.length }}</div>
      </div>

      <div class="status-card pending">
        <div class="status-label">Pending</div>
        <div class="status-value">{{ store.pendingTasks.length }}</div>
      </div>

      <div class="status-card running">
        <div class="status-label">Running</div>
        <div class="status-value">{{ store.runningTasks.length }}</div>
      </div>

      <div class="status-card completed">
        <div class="status-label">Completed</div>
        <div class="status-value">{{ store.completedTasks.length }}</div>
      </div>

      <div class="status-card failed">
        <div class="status-label">Failed</div>
        <div class="status-value">{{ store.failedTasks.length }}</div>
      </div>

      <div class="status-card agents">
        <div class="status-label">Active Agents</div>
        <div class="status-value">{{ store.onlineAgents.length }}</div>
      </div>
    </div>

    <div v-if="store.runningTasks.length > 0" class="running-tasks">
      <h3>Currently Running</h3>
      <div v-for="task in store.runningTasks" :key="task.id" class="running-task-item">
        <span class="task-device">{{ task.deviceName }}</span>
        <span class="task-type">{{ task.taskType }}</span>
        <div class="spinner"></div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useProvisioningStore } from '@/stores/provisioning'

const store = useProvisioningStore()
</script>

<style scoped>
.status-dashboard { border: 1px solid #e5e7eb; padding: 20px; border-radius: 8px; background: white; }
.status-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 12px; margin-top: 16px; }
.status-card { padding: 16px; border-radius: 6px; border: 1px solid #e5e7eb; background: #f9fafb; }
.status-card.pending { border-left: 4px solid #f59e0b; }
.status-card.running { border-left: 4px solid #3b82f6; }
.status-card.completed { border-left: 4px solid #10b981; }
.status-card.failed { border-left: 4px solid #ef4444; }
.status-card.agents { border-left: 4px solid #8b5cf6; }
.status-label { font-size: 12px; color: #6b7280; font-weight: 500; margin-bottom: 8px; }
.status-value { font-size: 24px; font-weight: 700; color: #111827; }
.running-tasks { margin-top: 20px; padding-top: 20px; border-top: 1px solid #e5e7eb; }
.running-tasks h3 { margin: 0 0 12px 0; font-size: 14px; color: #374151; }
.running-task-item { display: flex; align-items: center; gap: 12px; padding: 8px 12px; background: #f9fafb; border-radius: 6px; margin-bottom: 8px; }
.task-device { font-weight: 500; color: #111827; }
.task-type { font-size: 12px; color: #6b7280; background: #e5e7eb; padding: 2px 8px; border-radius: 4px; }
.spinner { width: 16px; height: 16px; border: 2px solid #e5e7eb; border-top-color: #3b82f6; border-radius: 50%; animation: spin 1s linear infinite; margin-left: auto; }
@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
