<template>
  <main style="padding:16px">
    <h1>Provisioning Dashboard</h1>
    <p style="color:#6b7280;margin-bottom:16px">Overview of provisioning operations</p>

    <StatusDashboard />

    <div class="grid" style="margin-top:24px">
      <section class="card">
        <h2>Recent Tasks</h2>
        <DataTable :rows="recentTasks" :loading="store.tasksLoading" :error="store.tasksError" :cols="4" :rowKey="row => row.id">
          <template #header>
            <th>Device</th>
            <th>Type</th>
            <th>Status</th>
            <th>Updated</th>
          </template>
          <template #row="{ row }">
            <td><router-link :to="`/provisioning/tasks/${row.id}`" class="link">{{ row.deviceName }}</router-link></td>
            <td><span class="badge">{{ row.taskType }}</span></td>
            <td><span :class="['status-badge', `status-${row.status}`]">{{ row.status }}</span></td>
            <td>{{ new Date(row.updatedAt).toLocaleString() }}</td>
          </template>
        </DataTable>
        <router-link to="/provisioning/tasks" class="secondary-button" style="margin-top:12px;display:inline-block">View All Tasks</router-link>
      </section>

      <section class="card">
        <h2>Active Agents</h2>
        <DataTable :rows="store.onlineAgents" :loading="store.agentsLoading" :error="store.agentsError" :cols="3" :rowKey="row => row.id">
          <template #header>
            <th>Name</th>
            <th>Status</th>
            <th>Version</th>
          </template>
          <template #row="{ row }">
            <td>{{ row.name }}</td>
            <td><span :class="['status-badge', `status-${row.status}`]">{{ row.status }}</span></td>
            <td><code>{{ row.version }}</code></td>
          </template>
        </DataTable>
        <router-link to="/provisioning/agents" class="secondary-button" style="margin-top:12px;display:inline-block">View All Agents</router-link>
      </section>
    </div>

    <BulkOperations style="margin-top:24px" />
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useProvisioningStore } from '@/stores/provisioning'
import DataTable from '@/components/DataTable.vue'
import StatusDashboard from '@/components/provisioning/StatusDashboard.vue'
import BulkOperations from '@/components/provisioning/BulkOperations.vue'

const store = useProvisioningStore()

const recentTasks = computed(() => {
  return store.tasks.slice(0, 10)
})

onMounted(() => {
  store.fetchTasks({ limit: 10 })
  store.fetchAgents()
})
</script>

<style scoped>
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
@media (max-width: 768px) {
  .grid { grid-template-columns: 1fr; }
}
.card { border: 1px solid #e5e7eb; padding: 16px; border-radius: 8px; background: white; }
.badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; background: #e5e7eb; }
.status-badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: 500; }
.status-pending { background: #fef3c7; color: #92400e; }
.status-running { background: #dbeafe; color: #1e40af; }
.status-completed { background: #d1fae5; color: #065f46; }
.status-failed { background: #fee2e2; color: #991b1b; }
.status-online { background: #d1fae5; color: #065f46; }
.status-busy { background: #fef3c7; color: #92400e; }
.status-offline { background: #f3f4f6; color: #4b5563; }
.link { color: #2563eb; text-decoration: underline; }
.secondary-button { padding: 8px 16px; background: #e5e7eb; border: none; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
</style>
