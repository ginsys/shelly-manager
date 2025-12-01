<template>
  <div class="page" data-testid="devices-page">
    <div class="toolbar">
      <h1 class="title" data-testid="page-title">Devices</h1>
      <div class="spacer" />
      <button class="primary-button" @click="showCreateDialog = true" data-testid="add-device-btn">
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
            <th @click="toggleSort('name')">Name <SortIcon :field="'name'" :sort="sort" /></th>
            <th @click="toggleSort('ip')">IP <SortIcon :field="'ip'" :sort="sort" /></th>
            <th @click="toggleSort('mac')">MAC <SortIcon :field="'mac'" :sort="sort" /></th>
            <th @click="toggleSort('type')">Type <SortIcon :field="'type'" :sort="sort" /></th>
            <th @click="toggleSort('status')">Status <SortIcon :field="'status'" :sort="sort" /></th>
            <th @click="toggleSort('last_seen')">Last Seen <SortIcon :field="'last_seen'" :sort="sort" /></th>
            <th>Firmware</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in pagedSortedFiltered" :key="d.id" data-testid="device-row">
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
            <td>
              <div class="actions">
                <button class="action-btn" @click="handleControl(d.id, 'on')" title="Turn On">ON</button>
                <button class="action-btn" @click="handleControl(d.id, 'off')" title="Turn Off">OFF</button>
                <button class="action-btn" @click="handleEdit(d)" title="Edit">✏️</button>
                <button class="action-btn danger" @click="handleDelete(d.id)" title="Delete">🗑️</button>
              </div>
            </td>
          </tr>
          <tr v-if="pagedSortedFiltered.length === 0">
            <td colspan="8" class="state" data-testid="empty-state">No devices found</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pagination" data-testid="pagination">
      <button class="btn" :disabled="page <= 1" @click="prevPage" data-testid="prev-page">Prev</button>
      <span>Page {{ page }} / {{ totalPages || 1 }}</span>
      <button class="btn" :disabled="!hasNext" @click="nextPage" data-testid="next-page">Next</button>
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
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { listDevices, createDevice, updateDevice, deleteDevice, controlDevice } from '../api/devices'
import type { Device } from '../api/types'
import DeviceForm from '../components/devices/DeviceForm.vue'

// Local sort descriptor
type Sort = { field: keyof Device | 'last_seen'; dir: 'asc' | 'desc' } | null

const loading = ref(false)
const error = ref<string | null>(null)
const items = ref<Device[]>([])
const page = ref(1)
const pageSize = ref(25)
const totalPages = ref<number | null>(null)
const hasNext = ref(false)
const search = ref('')
const sort = ref<Sort>(null)
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const editingDevice = ref<Device | null>(null)
const deviceFormRef = ref<InstanceType<typeof DeviceForm> | null>(null)

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

// The backend already returns a page slice; but we keep local guard in case of future changes
const pagedSortedFiltered = computed(() => sorted.value)

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
    await fetchData()
  } catch (e: any) {
    deviceFormRef.value?.setError(e?.message || 'Operation failed')
  }
}

async function handleDelete(id: number) {
  if (!confirm('Are you sure you want to delete this device?')) return
  try {
    await deleteDevice(id)
    await fetchData()
  } catch (e: any) {
    alert('Failed to delete device: ' + (e?.message || 'Unknown error'))
  }
}

async function handleControl(id: number, action: string) {
  try {
    await controlDevice(id, action)
    // Optionally refresh device status
    setTimeout(fetchData, 500)
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
