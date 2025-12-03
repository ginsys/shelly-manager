<template>
  <div class="page" data-testid="devices-page">
    <div class="toolbar">
      <h1 class="title" data-testid="page-title">Devices</h1>
      <div class="spacer" />
      <ColumnToggle
        :columns="columnOptions"
        :visibleColumns="store.visibleColumns"
        @toggle="handleColumnToggle"
        @selectAll="handleSelectAllColumns"
        @selectNone="handleSelectNoneColumns"
      />
      <button class="primary-button" @click="showCreateDialog = true" data-testid="add-device-btn">
        + Add Device
      </button>
      <input
        class="search"
        v-model="store.search"
        type="text"
        placeholder="Search (name, IP, MAC, type)"
        data-testid="device-search"
      />
      <select v-model.number="store.pageSize" class="select" data-testid="page-size-select">
        <option :value="10">10</option>
        <option :value="25">25</option>
        <option :value="50">50</option>
        <option :value="100">100</option>
      </select>
    </div>

    <div class="card" data-testid="device-list">
      <div v-if="store.loading" class="state" data-testid="loading-state">Loading...</div>

      <ErrorState
        v-else-if="store.error"
        title="Failed to Load Devices"
        :message="store.error"
        :retryable="true"
        @retry="handleRetry"
      />

      <EmptyState
        v-else-if="store.sortedItems.length === 0"
        title="No Devices Found"
        message="Get started by adding your first Shelly device"
        icon="🔌"
      >
        <template #action>
          <button class="primary-button" @click="showCreateDialog = true">
            + Add Device
          </button>
        </template>
      </EmptyState>

      <table v-else class="table" data-testid="devices-table">
        <thead>
          <tr>
            <th v-if="store.visibleColumns.name" @click="store.toggleSort('name')">
              Name <SortIcon :field="'name'" :sort="store.sort" />
            </th>
            <th v-if="store.visibleColumns.ip" @click="store.toggleSort('ip')">
              IP <SortIcon :field="'ip'" :sort="store.sort" />
            </th>
            <th v-if="store.visibleColumns.mac" @click="store.toggleSort('mac')">
              MAC <SortIcon :field="'mac'" :sort="store.sort" />
            </th>
            <th v-if="store.visibleColumns.type" @click="store.toggleSort('type')">
              Type <SortIcon :field="'type'" :sort="store.sort" />
            </th>
            <th v-if="store.visibleColumns.status" @click="store.toggleSort('status')">
              Status <SortIcon :field="'status'" :sort="store.sort" />
            </th>
            <th v-if="store.visibleColumns.last_seen" @click="store.toggleSort('last_seen')">
              Last Seen <SortIcon :field="'last_seen'" :sort="store.sort" />
            </th>
            <th v-if="store.visibleColumns.firmware">Firmware</th>
            <th v-if="store.visibleColumns.actions">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in store.sortedItems" :key="d.id" data-testid="device-row">
            <td v-if="store.visibleColumns.name">
              <router-link :to="`/devices/${d.id}`" class="rowlink" data-testid="device-link">{{ d.name || '-' }}</router-link>
            </td>
            <td v-if="store.visibleColumns.ip">{{ d.ip || '-' }}</td>
            <td v-if="store.visibleColumns.mac" class="mono">{{ d.mac }}</td>
            <td v-if="store.visibleColumns.type">{{ d.type }}</td>
            <td v-if="store.visibleColumns.status">
              <span :class="['chip', d.status]" data-testid="device-status">{{ d.status || 'unknown' }}</span>
            </td>
            <td v-if="store.visibleColumns.last_seen">{{ formatDate(d.last_seen) }}</td>
            <td v-if="store.visibleColumns.firmware" class="mono small">{{ d.firmware || '-' }}</td>
            <td v-if="store.visibleColumns.actions">
              <div class="actions">
                <button class="action-btn" @click="handleControl(d.id, 'on')" title="Turn On">ON</button>
                <button class="action-btn" @click="handleControl(d.id, 'off')" title="Turn Off">OFF</button>
                <button class="action-btn" @click="handleEdit(d)" title="Edit">✏️</button>
                <button class="action-btn danger" @click="handleDelete(d.id)" title="Delete">🗑️</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pagination" data-testid="pagination">
      <button class="btn" :disabled="store.page <= 1" @click="store.prevPage()" data-testid="prev-page">Prev</button>
      <span>Page {{ store.page }} / {{ store.totalPages || 1 }}</span>
      <button class="btn" :disabled="!store.hasNext" @click="store.nextPage()" data-testid="next-page">Next</button>
    </div>

    <!-- Create/Edit Dialog -->
    <div v-if="showCreateDialog || showEditDialog" class="modal-overlay" @click.self="closeDialogs">
      <div class="modal">
        <h2>{{ showEditDialog ? 'Edit Device' : 'Add New Device' }}</h2>
        <DeviceForm
          ref="deviceFormRef"
          :device="editingDevice"
          :isEdit="showEditDialog"
          @submit="handleFormSubmit"
          @cancel="closeDialogs"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { createDevice, updateDevice, deleteDevice, controlDevice } from '../api/devices'
import type { Device } from '../api/types'
import { useDevicesStore } from '../stores/devices'
import DeviceForm from '../components/devices/DeviceForm.vue'
import ErrorState from '../components/shared/ErrorState.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ColumnToggle from '../components/shared/ColumnToggle.vue'

const store = useDevicesStore()
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const editingDevice = ref<Device | null>(null)
const deviceFormRef = ref<InstanceType<typeof DeviceForm> | null>(null)

// Column configuration
const columnOptions = [
  { key: 'name', label: 'Name' },
  { key: 'ip', label: 'IP Address' },
  { key: 'mac', label: 'MAC Address' },
  { key: 'type', label: 'Type' },
  { key: 'status', label: 'Status' },
  { key: 'last_seen', label: 'Last Seen' },
  { key: 'firmware', label: 'Firmware' },
  { key: 'actions', label: 'Actions' }
]

onMounted(() => {
  store.loadPreferences()
  store.fetchDevices()
})

// Watch for page/pageSize changes and refetch
watch(() => store.page, () => store.fetchDevices())
watch(() => store.pageSize, () => {
  store.setPageSize(store.pageSize)
  store.fetchDevices()
})

function formatDate(iso?: string) {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

function handleRetry() {
  store.clearError()
  store.fetchDevices()
}

function handleColumnToggle(columnKey: string) {
  store.toggleColumn(columnKey as any)
}

function handleSelectAllColumns() {
  Object.keys(store.visibleColumns).forEach(key => {
    if (!store.visibleColumns[key as keyof typeof store.visibleColumns]) {
      store.toggleColumn(key as any)
    }
  })
}

function handleSelectNoneColumns() {
  Object.keys(store.visibleColumns).forEach(key => {
    if (store.visibleColumns[key as keyof typeof store.visibleColumns]) {
      store.toggleColumn(key as any)
    }
  })
}

function closeDialogs() {
  showCreateDialog.value = false
  showEditDialog.value = false
  editingDevice.value = null
  deviceFormRef.value?.reset()
}

function handleEdit(device: Device) {
  editingDevice.value = device
  showEditDialog.value = true
}

async function handleFormSubmit(data: Partial<Device>) {
  try {
    if (showEditDialog.value && editingDevice.value) {
      await updateDevice(editingDevice.value.id, data)
    } else {
      await createDevice(data)
    }
    closeDialogs()
    await store.fetchDevices()
  } catch (e: any) {
    deviceFormRef.value?.setError(e?.message || 'Operation failed')
  }
}

async function handleDelete(id: number) {
  if (!confirm('Are you sure you want to delete this device?')) return
  try {
    await deleteDevice(id)
    await store.fetchDevices()
  } catch (e: any) {
    alert('Failed to delete device: ' + (e?.message || 'Unknown error'))
  }
}

async function handleControl(id: number, action: string) {
  try {
    await controlDevice(id, action)
    // Optionally refresh device status
    setTimeout(() => store.fetchDevices(), 500)
  } catch (e: any) {
    alert('Failed to control device: ' + (e?.message || 'Unknown error'))
  }
}

</script>

<script lang="ts">
// Local presentational helper (inline component)
export default {
  components: {
    SortIcon: {
      props: { field: { type: String, required: true }, sort: { type: Object, required: false } },
      template: `<span class="sort" v-if="sort && sort.field === field">{{ sort.dir === 'asc' ? '▲' : '▼' }}</span>`
    }
  }
}
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }
.search { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px; min-width: 260px; }
.select { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; }
.state { padding: 16px; text-align: center; color: #64748b; }
.state.error { color: #b91c1c; }
.table { width: 100%; border-collapse: collapse; }
.table th, .table td { text-align: left; padding: 10px 12px; border-bottom: 1px solid #f1f5f9; }
.table th { background: #f8fafc; cursor: pointer; user-select: none; white-space: nowrap; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.small { font-size: 12px; color: #475569; }
.chip { padding: 2px 8px; border-radius: 999px; font-size: 12px; background: #e2e8f0; color: #334155; }
.chip.online { background: #dcfce7; color: #065f46; }
.chip.offline { background: #fee2e2; color: #991b1b; }
.sort { margin-left: 4px; font-size: 12px; color: #64748b; }
.pagination { display: flex; align-items: center; gap: 8px; justify-content: center; padding: 8px; color: #334155; }
.btn { padding: 6px 10px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.rowlink { color: #2563eb; text-decoration: none; }
.rowlink:hover { text-decoration: underline; }
.primary-button { padding: 8px 16px; background: #2563eb; color: white; border: none; border-radius: 6px; cursor: pointer; font-weight: 500; }
.actions { display: flex; gap: 4px; }
.action-btn { padding: 4px 8px; font-size: 12px; background: #e5e7eb; border: none; border-radius: 4px; cursor: pointer; }
.action-btn.danger { background: #fee2e2; color: #991b1b; }
.action-btn:hover { opacity: 0.8; }
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; padding: 24px; border-radius: 8px; max-width: 600px; width: 90%; max-height: 90vh; overflow-y: auto; }
.modal h2 { margin-top: 0; }
</style>
