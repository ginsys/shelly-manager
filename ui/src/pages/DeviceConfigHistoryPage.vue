<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" :to="`/devices/${deviceId}/config`">← Back to Config</router-link>
      <h1 class="title">Configuration History</h1>
      <div class="spacer" />
      <button class="btn" @click="handleRefresh">Refresh</button>
    </div>

    <!-- Loading/Error states -->
    <div v-if="store.loading" class="card state">Loading...</div>
    <div v-else-if="store.error" class="card state error">{{ store.error }}</div>

    <!-- History Table -->
    <div v-else class="card">
      <table v-if="store.history.length > 0" class="table">
        <thead>
          <tr>
            <th>Timestamp</th>
            <th>Source</th>
            <th>User</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in store.history" :key="entry.id">
            <td>{{ formatDate(entry.timestamp) }}</td>
            <td>
              <span :class="['source-badge', entry.source]">{{ entry.source }}</span>
            </td>
            <td>{{ entry.user || '-' }}</td>
            <td>
              <button class="action-btn" @click="viewConfig(entry)">View</button>
              <button class="action-btn" @click="restoreConfig(entry)">Restore</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="state">No configuration history found</div>
    </div>

    <!-- Config Viewer Dialog -->
    <div v-if="viewingConfig" class="modal-overlay" @click.self="viewingConfig = null">
      <div class="modal">
        <h2>Configuration from {{ formatDate(viewingConfig.timestamp) }}</h2>
        <pre class="config-view">{{ JSON.stringify(viewingConfig.config, null, 2) }}</pre>
        <div class="modal-actions">
          <button class="btn" @click="viewingConfig = null">Close</button>
          <button class="btn primary" @click="restoreConfig(viewingConfig)">Restore This Version</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDeviceConfigStore } from '@/stores/deviceConfig'
import type { ConfigHistoryEntry } from '@/api/deviceConfig'

const route = useRoute()
const router = useRouter()
const store = useDeviceConfigStore()

const deviceId = ref<number | string>(route.params.id as string)
const viewingConfig = ref<ConfigHistoryEntry | null>(null)

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function viewConfig(entry: ConfigHistoryEntry) {
  viewingConfig.value = entry
}

async function restoreConfig(entry: ConfigHistoryEntry) {
  if (!confirm('Are you sure you want to restore this configuration?')) return

  try {
    await store.saveStoredConfig(deviceId.value, entry.config)
    viewingConfig.value = null
    alert('Configuration restored successfully')
    router.push(`/devices/${deviceId.value}/config`)
  } catch (e: any) {
    alert('Failed to restore configuration: ' + (e?.message || 'Unknown error'))
  }
}

async function handleRefresh() {
  await store.fetchHistory(deviceId.value)
}

onMounted(() => {
  handleRefresh()
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.btn:hover { background: #f8fafc; }
.btn.primary { background: #2563eb; color: white; border-color: #2563eb; }
.btn.primary:hover { background: #1d4ed8; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 16px; }
.state { text-align: center; color: #64748b; padding: 32px; }
.state.error { color: #b91c1c; }

.table { width: 100%; border-collapse: collapse; }
.table th, .table td { text-align: left; padding: 10px 12px; border-bottom: 1px solid #f1f5f9; }
.table th { background: #f8fafc; font-weight: 600; }

.source-badge { padding: 2px 8px; border-radius: 999px; font-size: 12px; font-weight: 600; }
.source-badge.manual { background: #e0e7ff; color: #3730a3; }
.source-badge.template { background: #fef3c7; color: #92400e; }
.source-badge.import { background: #dcfce7; color: #065f46; }
.source-badge.sync { background: #f3e8ff; color: #6b21a8; }

.action-btn { padding: 4px 8px; font-size: 12px; background: #e5e7eb; border: none; border-radius: 4px; cursor: pointer; margin-right: 4px; }
.action-btn:hover { background: #cbd5e1; }

.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; padding: 24px; border-radius: 8px; max-width: 800px; width: 90%; max-height: 90vh; overflow-y: auto; }
.modal h2 { margin-top: 0; }
.modal-actions { display: flex; gap: 12px; justify-content: flex-end; margin-top: 16px; }
.config-view { background: #f8fafc; padding: 12px; border-radius: 6px; overflow-x: auto; font-family: ui-monospace, monospace; font-size: 13px; margin: 0; max-height: 500px; }
</style>
