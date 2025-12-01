<template>
  <main style="padding:16px">
    <h1>Provisioning Agents</h1>
    <p style="color:#6b7280;margin-bottom:16px">Manage provisioning agents and their status</p>

    <div style="margin-bottom:16px">
      <button class="secondary-button" @click="store.fetchAgents()">Refresh</button>
    </div>

    <DataTable :rows="store.agents" :loading="store.agentsLoading" :error="store.agentsError" :cols="6" :rowKey="row => row.id">
      <template #header>
        <th>Name</th>
        <th>Status</th>
        <th>Version</th>
        <th>Capabilities</th>
        <th>Last Seen</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>{{ row.name }}</td>
        <td><span :class="['status-badge', `status-${row.status}`]">{{ row.status }}</span></td>
        <td><code>{{ row.version }}</code></td>
        <td>
          <div class="capabilities">
            <span v-for="cap in row.capabilities" :key="cap" class="capability-badge">{{ cap }}</span>
          </div>
        </td>
        <td>{{ formatLastSeen(row.lastSeen) }}</td>
        <td>
          <button class="link-button" @click="handleRefreshAgent(row.id)">Refresh Status</button>
        </td>
      </template>
    </DataTable>

    <section style="margin-top:24px">
      <h2>Summary</h2>
      <div class="stats">
        <div class="card">Total: {{ store.agents.length }}</div>
        <div class="card">Online: {{ store.onlineAgents.length }}</div>
        <div class="card">Busy: {{ store.busyAgents.length }}</div>
        <div class="card">Offline: {{ store.offlineAgents.length }}</div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useProvisioningStore } from '@/stores/provisioning'
import DataTable from '@/components/DataTable.vue'

const store = useProvisioningStore()

onMounted(() => {
  store.fetchAgents()
})

function formatLastSeen(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)

  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`

  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}h ago`

  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ago`
}

async function handleRefreshAgent(id: string) {
  try {
    await store.refreshAgentStatus(id)
  } catch (e) {
    alert('Failed to refresh agent status: ' + (e as Error).message)
  }
}
</script>

<style scoped>
.badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; background: #e5e7eb; }
.status-badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: 500; }
.status-online { background: #d1fae5; color: #065f46; }
.status-busy { background: #fef3c7; color: #92400e; }
.status-offline { background: #f3f4f6; color: #4b5563; }
.capabilities { display: flex; gap: 4px; flex-wrap: wrap; }
.capability-badge { padding: 2px 6px; border-radius: 3px; font-size: 11px; background: #dbeafe; color: #1e40af; }
.stats { display: flex; gap: 12px; }
.card { border: 1px solid #e5e7eb; padding: 8px 12px; border-radius: 6px; }
.link-button { color: #2563eb; cursor: pointer; background: none; border: none; text-decoration: underline; }
.secondary-button { padding: 8px 16px; background: #e5e7eb; border: none; border-radius: 6px; cursor: pointer; }
</style>
