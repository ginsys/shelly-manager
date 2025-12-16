<template>
  <main style="padding:16px">
    <h1>Provisioning Tasks</h1>
    <p style="color:#6b7280;margin-bottom:16px">Manage device provisioning tasks</p>

    <div style="margin-bottom:16px;display:flex;gap:12px">
      <select v-model="statusFilter" @change="applyFilter" class="form-input" style="width:auto">
        <option value="">All Statuses</option>
        <option value="pending">Pending</option>
        <option value="running">Running</option>
        <option value="completed">Completed</option>
        <option value="failed">Failed</option>
      </select>
      <button class="secondary-button" @click="store.fetchTasks()">Refresh</button>
    </div>

    <DataTable :rows="store.tasks" :loading="store.tasksLoading" :error="store.tasksError" :cols="6" :rowKey="row => row.id">
      <template #header>
        <th>Device</th>
        <th>Task Type</th>
        <th>Status</th>
        <th>Created</th>
        <th>Updated</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>{{ row.deviceName }}</td>
        <td><span class="badge">{{ row.taskType }}</span></td>
        <td><span :class="['status-badge', `status-${row.status}`]">{{ row.status }}</span></td>
        <td>{{ new Date(row.createdAt).toLocaleString() }}</td>
        <td>{{ new Date(row.updatedAt).toLocaleString() }}</td>
        <td>
          <router-link :to="`/provisioning/tasks/${row.id}`" class="link-button">View</router-link>
          <button v-if="row.status === 'running' || row.status === 'pending'" class="link-button text-red-600" @click="handleCancel(row.id)" style="margin-left:8px">Cancel</button>
        </td>
      </template>
    </DataTable>

    <section style="margin-top:24px">
      <h2>Summary</h2>
      <div class="stats">
        <div class="card">Total: {{ store.tasks.length }}</div>
        <div class="card">Pending: {{ store.pendingTasks.length }}</div>
        <div class="card">Running: {{ store.runningTasks.length }}</div>
        <div class="card">Completed: {{ store.completedTasks.length }}</div>
        <div class="card">Failed: {{ store.failedTasks.length }}</div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useProvisioningStore } from '@/stores/provisioning'
import DataTable from '@/components/DataTable.vue'

const store = useProvisioningStore()
const statusFilter = ref('')

onMounted(() => {
  store.fetchTasks()
})

function applyFilter() {
  store.setTasksStatusFilter(statusFilter.value)
  store.fetchTasks()
}

async function handleCancel(id: string) {
  if (!confirm('Cancel this task?')) return
  try {
    await store.cancelProvisioningTask(id)
  } catch (e) {
    alert('Failed to cancel task: ' + (e as Error).message)
  }
}
</script>

<style scoped>
.badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; background: #e5e7eb; }
.status-badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: 500; }
.status-pending { background: #fef3c7; color: #92400e; }
.status-running { background: #dbeafe; color: #1e40af; }
.status-completed { background: #d1fae5; color: #065f46; }
.status-failed { background: #fee2e2; color: #991b1b; }
.stats { display: flex; gap: 12px; }
.card { border: 1px solid #e5e7eb; padding: 8px 12px; border-radius: 6px; }
.link-button { color: #2563eb; cursor: pointer; background: none; border: none; text-decoration: underline; }
.text-red-600 { color: #dc2626; }
.form-input { padding: 8px; border: 1px solid #d1d5db; border-radius: 4px; }
.secondary-button { padding: 8px 16px; background: #e5e7eb; border: none; border-radius: 6px; cursor: pointer; }
</style>
