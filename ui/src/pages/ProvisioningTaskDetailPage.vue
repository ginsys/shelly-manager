<template>
  <main style="padding:16px">
    <div style="display:flex;align-items:center;gap:12px;margin-bottom:16px">
      <router-link to="/provisioning/tasks" class="back-button">← Back</router-link>
      <h1 style="margin:0">Task Details</h1>
    </div>

    <div v-if="store.tasksLoading" style="color:#6b7280">Loading...</div>
    <div v-else-if="store.tasksError" class="error">{{ store.tasksError }}</div>
    <div v-else-if="!task" class="error">Task not found</div>

    <div v-else class="card">
      <section class="detail-section">
        <h2>Task Information</h2>
        <dl class="detail-list">
          <dt>ID</dt>
          <dd><code>{{ task.id }}</code></dd>

          <dt>Device</dt>
          <dd>
            <router-link :to="`/devices/${task.deviceId}`" class="link">{{ task.deviceName }}</router-link>
          </dd>

          <dt>Task Type</dt>
          <dd><span class="badge">{{ task.taskType }}</span></dd>

          <dt>Status</dt>
          <dd><span :class="['status-badge', `status-${task.status}`]">{{ task.status }}</span></dd>

          <dt>Created</dt>
          <dd>{{ new Date(task.createdAt).toLocaleString() }}</dd>

          <dt>Updated</dt>
          <dd>{{ new Date(task.updatedAt).toLocaleString() }}</dd>
        </dl>
      </section>

      <section class="detail-section">
        <h2>Configuration</h2>
        <pre class="config-block">{{ JSON.stringify(task.config, null, 2) }}</pre>
      </section>

      <section v-if="task.result" class="detail-section">
        <h2>Result</h2>
        <pre class="config-block">{{ JSON.stringify(task.result, null, 2) }}</pre>
      </section>

      <section v-if="task.error" class="detail-section">
        <h2>Error</h2>
        <div class="error-box">{{ task.error }}</div>
      </section>

      <div class="actions">
        <button
          v-if="task.status === 'running' || task.status === 'pending'"
          class="danger-button"
          @click="handleCancel"
        >
          Cancel Task
        </button>
        <button class="secondary-button" @click="handleRefresh">Refresh</button>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useProvisioningStore } from '@/stores/provisioning'

const route = useRoute()
const store = useProvisioningStore()

const taskId = computed(() => route.params.id as string)
const task = computed(() => store.currentTask)

onMounted(async () => {
  if (taskId.value) {
    await store.fetchTask(taskId.value)
  }
})

async function handleRefresh() {
  if (taskId.value) {
    await store.fetchTask(taskId.value)
  }
}

async function handleCancel() {
  if (!taskId.value) return
  if (!confirm('Cancel this task?')) return

  try {
    await store.cancelProvisioningTask(taskId.value)
    await store.fetchTask(taskId.value)
  } catch (e) {
    alert('Failed to cancel task: ' + (e as Error).message)
  }
}
</script>

<style scoped>
.card { border: 1px solid #e5e7eb; padding: 20px; border-radius: 8px; background: white; }
.detail-section { margin-bottom: 24px; }
.detail-section:last-of-type { margin-bottom: 0; }
.detail-list { display: grid; grid-template-columns: 150px 1fr; gap: 12px; margin: 0; }
dt { font-weight: 600; color: #374151; }
dd { margin: 0; color: #6b7280; }
.config-block { background: #f9fafb; padding: 12px; border-radius: 6px; overflow-x: auto; font-size: 12px; }
.error-box { background: #fee2e2; color: #991b1b; padding: 12px; border-radius: 6px; }
.actions { display: flex; gap: 12px; margin-top: 16px; padding-top: 16px; border-top: 1px solid #e5e7eb; }
.badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; background: #e5e7eb; }
.status-badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: 500; }
.status-pending { background: #fef3c7; color: #92400e; }
.status-running { background: #dbeafe; color: #1e40af; }
.status-completed { background: #d1fae5; color: #065f46; }
.status-failed { background: #fee2e2; color: #991b1b; }
.link { color: #2563eb; text-decoration: underline; }
.back-button { color: #2563eb; text-decoration: none; }
.secondary-button { padding: 8px 16px; background: #e5e7eb; border: none; border-radius: 6px; cursor: pointer; }
.danger-button { padding: 8px 16px; background: #dc2626; color: white; border: none; border-radius: 6px; cursor: pointer; }
.error { color: #dc2626; padding: 12px; background: #fee2e2; border-radius: 6px; }
</style>
