<template>
  <div class="page" data-testid="devices-page">
    <div class="toolbar">
      <h1 class="title" data-testid="page-title">Devices</h1>
      <div class="spacer" />
      <button v-if="selectedDevices.length > 0" @click="bulkDelete" class="btn-danger" data-testid="bulk-delete">
        Delete Selected ({{ selectedDevices.length }})
      </button>
      <button @click="showCreateForm" class="btn-primary" data-testid="add-device">
        + Add Device
      </button>
      <input
        class="search"
        v-model="search"
        type="text"
        placeholder="Search (name, IP, MAC, type)"
        data-testid="device-search"
      />
      <select v-model.number="pageSize" class="select" data-testid="page-size-select">
        <option :value="10">10</option>
        <option :value="25">25</option>
        <option :value="50">50</option>
      </select>
    </div>

    <div class="card" data-testid="device-list">
      <div v-if="loading" class="state" data-testid="loading-state">Loading...</div>
      <div v-else-if="error" class="state error" data-testid="error-state">{{ error }}</div>

      <table v-else class="table" data-testid="devices-table">
        <thead>
          <tr>
            <th class="checkbox-col">
              <input type="checkbox" v-model="selectAll" @change="toggleSelectAll" data-testid="select-all" />
            </th>
            <th @click="toggleSort('name')">Name <SortIcon :field="'name'" :sort="sort" /></th>
            <th @click="toggleSort('ip')">IP <SortIcon :field="'ip'" :sort="sort" /></th>
            <th @click="toggleSort('mac')">MAC <SortIcon :field="'mac'" :sort="sort" /></th>
            <th @click="toggleSort('type')">Type <SortIcon :field="'type'" :sort="sort" /></th>
            <th @click="toggleSort('status')">Status <SortIcon :field="'status'" :sort="sort" /></th>
            <th @click="toggleSort('last_seen')">Last Seen <SortIcon :field="'last_seen'" :sort="sort" /></th>
            <th>Firmware</th>
            <th class="actions-col">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in pagedSortedFiltered" :key="d.id" data-testid="device-row">
            <td class="checkbox-col">
              <input type="checkbox" :value="d.id" v-model="selectedDevices" data-testid="device-checkbox" />
            </td>
            <td>
              <router-link :to="`/devices/${d.id}`" class="rowlink" data-testid="device-link">{{ d.name || '-' }}</router-link>
            </td>
            <td>{{ d.ip || '-' }}</td>
            <td class="mono">{{ d.mac }}</td>
            <td>{{ d.type }}</td>
            <td>
              <span :class="['chip', d.status]" data-testid="device-status">{{ d.status || 'unknown' }}</span>
            </td>
            <td>{{ formatDate(d.last_seen) }}</td>
            <td class="mono small">{{ d.firmware || '-' }}</td>
            <td class="actions-col">
              <button @click="quickControl(d, 'on')" class="action-btn" title="Turn On" data-testid="quick-on">⚡</button>
              <button @click="quickControl(d, 'off')" class="action-btn" title="Turn Off" data-testid="quick-off">⏸</button>
              <button @click="showEditForm(d)" class="action-btn" title="Edit" data-testid="edit-device">✏️</button>
              <button @click="confirmDelete(d)" class="action-btn" title="Delete" data-testid="delete-device">🗑️</button>
            </td>
          </tr>
          <tr v-if="pagedSortedFiltered.length === 0">
            <td colspan="9" class="state" data-testid="empty-state">No devices found</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pagination" data-testid="pagination">
      <button class="btn" :disabled="page <= 1" @click="prevPage" data-testid="prev-page">Prev</button>
      <span>Page {{ page }} / {{ totalPages || 1 }}</span>
      <button class="btn" :disabled="!hasNext" @click="nextPage" data-testid="next-page">Next</button>
    </div>

    <!-- Device Form Overlay -->
    <div v-if="formVisible" class="overlay" @click.self="closeForm">
      <DeviceForm
        :existing-device="editingDevice"
        :loading="formLoading"
        :error="formError"
        @submit="handleFormSubmit"
        @cancel="closeForm"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useQuasar } from 'quasar'
import { listDevices, createDevice, updateDevice, deleteDevice, controlDevice } from '../api/devices'
import type { Device, CreateDeviceRequest, UpdateDeviceRequest } from '../api/types'
import DeviceForm from '../components/devices/DeviceForm.vue'

// Local sort descriptor
type Sort = { field: keyof Device | 'last_seen'; dir: 'asc' | 'desc' } | null

const $q = useQuasar()

const loading = ref(false)
const error = ref<string | null>(null)
const items = ref<Device[]>([])
const page = ref(1)
const pageSize = ref(25)
const totalPages = ref<number | null>(null)
const hasNext = ref(false)
const search = ref('')
const sort = ref<Sort>(null)

// Form state
const formVisible = ref(false)
const formLoading = ref(false)
const formError = ref<string | null>(null)
const editingDevice = ref<Device | null>(null)

// Bulk selection
const selectedDevices = ref<number[]>([])
const selectAll = ref(false)

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    const { items: list, meta } = await listDevices({ page: page.value, pageSize: pageSize.value })
    items.value = list
    const p = meta?.pagination
    totalPages.value = p?.total_pages ?? null
    hasNext.value = !!p?.has_next
  } catch (e: any) {
    error.value = e?.message || 'Failed to load devices'
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
watch([page, pageSize], fetchData)

function prevPage() { if (page.value > 1) page.value -= 1 }
function nextPage() { if (hasNext.value) page.value += 1 }

function toggleSort(field: Sort['field']) {
  if (!sort.value || sort.value.field !== field) {
    sort.value = { field, dir: 'asc' }
  } else if (sort.value.dir === 'asc') {
    sort.value.dir = 'desc'
  } else {
    sort.value = null
  }
}

function formatDate(iso?: string) {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter(d => {
    return (
      (d.name || '').toLowerCase().includes(q) ||
      (d.ip || '').toLowerCase().includes(q) ||
      (d.mac || '').toLowerCase().includes(q) ||
      (d.type || '').toLowerCase().includes(q)
    )
  })
})

const sorted = computed(() => {
  if (!sort.value) return filtered.value
  const { field, dir } = sort.value
  const copy = filtered.value.slice()
  copy.sort((a: any, b: any) => {
    const av = a?.[field]
    const bv = b?.[field]
    if (av == null && bv == null) return 0
    if (av == null) return 1
    if (bv == null) return -1
    if (av < bv) return dir === 'asc' ? -1 : 1
    if (av > bv) return dir === 'asc' ? 1 : -1
    return 0
  })
  return copy
})

const pagedSortedFiltered = computed(() => sorted.value)

// Form handlers
function showCreateForm() {
  editingDevice.value = null
  formError.value = null
  formVisible.value = true
}

function showEditForm(device: Device) {
  editingDevice.value = device
  formError.value = null
  formVisible.value = true
}

function closeForm() {
  formVisible.value = false
  formLoading.value = false
  formError.value = null
  editingDevice.value = null
}

async function handleFormSubmit(data: CreateDeviceRequest | UpdateDeviceRequest) {
  formLoading.value = true
  formError.value = null

  try {
    if (editingDevice.value) {
      // Update existing device
      await updateDevice(editingDevice.value.id, data)
      $q.notify({ type: 'positive', message: 'Device updated successfully', position: 'top' })
    } else {
      // Create new device
      await createDevice(data as CreateDeviceRequest)
      $q.notify({ type: 'positive', message: 'Device created successfully', position: 'top' })
    }

    closeForm()
    await fetchData()
  } catch (e: any) {
    formError.value = e?.message || 'Operation failed'
  } finally {
    formLoading.value = false
  }
}

// Delete handlers
function confirmDelete(device: Device) {
  $q.dialog({
    title: 'Delete Device',
    message: `Are you sure you want to delete "${device.name || device.ip}"? This action cannot be undone.`,
    cancel: true,
    persistent: true,
    color: 'negative'
  }).onOk(async () => {
    try {
      await deleteDevice(device.id)
      $q.notify({ type: 'positive', message: 'Device deleted successfully', position: 'top' })
      await fetchData()
      selectedDevices.value = selectedDevices.value.filter(id => id !== device.id)
    } catch (e: any) {
      $q.notify({ type: 'negative', message: e?.message || 'Failed to delete device', position: 'top' })
    }
  })
}

function bulkDelete() {
  if (selectedDevices.value.length === 0) return

  $q.dialog({
    title: 'Delete Multiple Devices',
    message: `Are you sure you want to delete ${selectedDevices.value.length} device(s)? This action cannot be undone.`,
    cancel: true,
    persistent: true,
    color: 'negative'
  }).onOk(async () => {
    const ids = [...selectedDevices.value]
    let successCount = 0
    let failCount = 0

    for (const id of ids) {
      try {
        await deleteDevice(id)
        successCount++
      } catch {
        failCount++
      }
    }

    if (successCount > 0) {
      $q.notify({ type: 'positive', message: `Successfully deleted ${successCount} device(s)`, position: 'top' })
    }
    if (failCount > 0) {
      $q.notify({ type: 'negative', message: `Failed to delete ${failCount} device(s)`, position: 'top' })
    }

    selectedDevices.value = []
    selectAll.value = false
    await fetchData()
  })
}

function toggleSelectAll() {
  if (selectAll.value) {
    selectedDevices.value = pagedSortedFiltered.value.map(d => d.id)
  } else {
    selectedDevices.value = []
  }
}

// Quick control
async function quickControl(device: Device, action: 'on' | 'off') {
  try {
    await controlDevice(device.id, { action })
    $q.notify({ type: 'positive', message: `Device turned ${action}`, position: 'top' })
    // Optionally refresh status
  } catch (e: any) {
    $q.notify({ type: 'negative', message: e?.message || `Failed to turn ${action}`, position: 'top' })
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
.toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
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
.table th.checkbox-col,
.table td.checkbox-col {
  width: 40px;
  text-align: center;
  cursor: default;
}
.table th.actions-col,
.table td.actions-col {
  width: 160px;
  text-align: center;
  cursor: default;
}
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.small { font-size: 12px; color: #475569; }
.chip { padding: 2px 8px; border-radius: 999px; font-size: 12px; background: #e2e8f0; color: #334155; }
.chip.online { background: #dcfce7; color: #065f46; }
.chip.offline { background: #fee2e2; color: #991b1b; }
.sort { margin-left: 4px; font-size: 12px; color: #64748b; }
.pagination { display: flex; align-items: center; gap: 8px; justify-content: center; padding: 8px; color: #334155; }
.btn { padding: 6px 10px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-primary {
  padding: 6px 12px;
  border: none;
  background: #3b82f6;
  color: white;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 500;
}
.btn-primary:hover { background: #2563eb; }
.btn-danger {
  padding: 6px 12px;
  border: none;
  background: #ef4444;
  color: white;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 500;
}
.btn-danger:hover { background: #dc2626; }
.rowlink { color: #2563eb; text-decoration: none; }
.rowlink:hover { text-decoration: underline; }
.action-btn {
  background: none;
  border: none;
  font-size: 16px;
  cursor: pointer;
  padding: 4px 6px;
  opacity: 0.7;
  transition: opacity 0.2s;
}
.action-btn:hover { opacity: 1; }

.overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}
</style>
