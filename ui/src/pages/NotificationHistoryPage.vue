<template>
  <main style="padding:16px">
    <h1>Notification History</h1>
    <p style="color:#6b7280;margin-bottom:16px">
      View past notification deliveries and their status
    </p>

    <DataTable
      :rows="store.history"
      :loading="store.historyLoading"
      :error="store.historyError"
      :cols="6"
      :rowKey="row => row.id"
    >
      <template #header>
        <th>Trigger</th>
        <th>Channel</th>
        <th>Rule</th>
        <th>Status</th>
        <th>Sent At</th>
        <th>Error</th>
      </template>
      <template #row="{ row }">
        <td><span class="badge">{{ row.triggerType }}</span></td>
        <td>{{ getChannelName(row.channelId) }}</td>
        <td>{{ getRuleName(row.ruleId) }}</td>
        <td>
          <span :class="['status-badge', `status-${row.status}`]">
            {{ row.status }}
          </span>
        </td>
        <td>{{ new Date(row.sentAt ?? row.createdAt).toLocaleString() }}</td>
        <td>{{ row.error || '-' }}</td>
      </template>
    </DataTable>

    <div style="margin-top:16px;display:flex;gap:8px;align-items:center">
      <button
        class="secondary-button"
        :disabled="!store.historyPagination?.has_previous"
        @click="changePage(store.historyPage - 1)"
      >
        ← Previous
      </button>
      <span>Page {{ store.historyPage }} of {{ store.historyPagination?.total_pages || 1 }}</span>
      <button
        class="secondary-button"
        :disabled="!store.historyPagination?.has_next"
        @click="changePage(store.historyPage + 1)"
      >
        Next →
      </button>
      <select :value="store.historyLimit" @change="changePageSize" class="form-input" style="width:auto;margin-left:auto">
        <option :value="25">25 per page</option>
        <option :value="50">50 per page</option>
        <option :value="100">100 per page</option>
      </select>
    </div>

    <section style="margin-top:24px">
      <h2>Summary</h2>
      <div class="stats">
        <div class="card">Total: {{ store.historyTotal }}</div>
        <div class="card">Sent on this page: {{ store.sentNotifications.length }}</div>
        <div class="card">Failed on this page: {{ store.failedNotifications.length }}</div>
        <div class="card">Pending on this page: {{ store.pendingNotifications.length }}</div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useNotificationsStore } from '@/stores/notifications'
import DataTable from '@/components/DataTable.vue'

const store = useNotificationsStore()

onMounted(() => {
  store.fetchHistory()
  store.fetchChannels()
  store.fetchRules()
})

const getChannelName = computed(() => (channelId: string) => {
  return store.channelById(channelId)?.name || 'Unknown'
})

const getRuleName = computed(() => (ruleId: string) => {
  return store.rules.find(r => r.id === ruleId)?.name || 'Unknown'
})

async function changePage(page: number) {
  if (page < 1) return
  // The store commits the page with the server's description of it, after a successful load: a failed
  // load leaves the page and its pager as they were.
  await store.fetchHistory({ page })
}

// The selector shows the limit the store committed, so a size change whose load failed shows the old size.
async function changePageSize(event: Event) {
  const select = event.target as HTMLSelectElement
  await store.fetchHistory({ page: 1, limit: Number(select.value) })
  select.value = String(store.historyLimit)
}
</script>

<style scoped>
.badge {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  background: #e5e7eb;
}
.status-badge {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}
.status-sent {
  background: #d1fae5;
  color: #065f46;
}
.status-failed {
  background: #fee2e2;
  color: #991b1b;
}
.status-pending {
  background: #fef3c7;
  color: #92400e;
}
.stats {
  display: flex;
  gap: 12px;
}
.card {
  border: 1px solid #e5e7eb;
  padding: 8px 12px;
  border-radius: 6px;
}
.secondary-button {
  padding: 8px 16px;
  background: #e5e7eb;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
.secondary-button:hover:not(:disabled) {
  background: #d1d5db;
}
.secondary-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.form-input {
  padding: 8px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
}
</style>
