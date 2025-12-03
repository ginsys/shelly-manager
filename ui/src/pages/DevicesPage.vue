<template>
  <div class="page" data-testid="devices-page">
    <div class="toolbar">
      <h1 class="title" data-testid="page-title">Devices</h1>
      <div class="spacer" />
      <button class="btn" data-testid="add-device" @click="openAdd">Add Device</button>
      <input
        class="search"
        v-model="store.search"
        type="text"
        placeholder="Search (name, IP, MAC, type)"
        data-testid="device-search"
      />
      <ColumnToggle :columns="availableColumns" v-model:modelValue="columnsModel" />
      <select v-model.number="pageSizeModel" class="select" data-testid="page-size-select">
        <option :value="10">10</option>
        <option :value="25">25</option>
        <option :value="50">50</option>
        <option :value="100">100</option>
      </select>
    </div>

    <div class="card" data-testid="device-list">
      <div v-if="loading" class="state" data-testid="loading-state">Loading...</div>
      <ErrorState v-else-if="error" data-testid="error-state" title="Failed to load devices" :message="error" :retryable="true" @retry="fetchData" />

      <table v-else class="table" data-testid="devices-table">
        <thead>
          <tr>
            <th v-if="store.columns.name" @click="toggleSort('name')">Name <SortIcon :field="'name'" :sort="sort" /></th>
            <th v-if="store.columns.ip" @click="toggleSort('ip')">IP <SortIcon :field="'ip'" :sort="sort" /></th>
            <th v-if="store.columns.mac" @click="toggleSort('mac')">MAC <SortIcon :field="'mac'" :sort="sort" /></th>
            <th v-if="store.columns.type" @click="toggleSort('type')">Type <SortIcon :field="'type'" :sort="sort" /></th>
            <th v-if="store.columns.status" @click="toggleSort('status')">Status <SortIcon :field="'status'" :sort="sort" /></th>
            <th v-if="store.columns.last_seen" @click="toggleSort('last_seen')">Last Seen <SortIcon :field="'last_seen'" :sort="sort" /></th>
            <th v-if="store.columns.firmware">Firmware</th>
            <th class="actions">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in pagedSortedFiltered" :key="d.id" data-testid="device-row">
            <td v-if="store.columns.name">
              <router-link :to="`/devices/${d.id}`" class="rowlink" data-testid="device-link" :aria-label="`View details for ${d.name || d.ip || d.mac}`">{{ d.name || '-' }}</router-link>
            </td>
            <td v-if="store.columns.ip">{{ d.ip || '-' }}</td>
            <td v-if="store.columns.mac" class="mono">{{ d.mac }}</td>
            <td v-if="store.columns.type">{{ d.type }}</td>
            <td v-if="store.columns.status">
              <span :class="['chip', d.status]" data-testid="device-status">{{ d.status || 'unknown' }}</span>
            </td>
            <td v-if="store.columns.last_seen">{{ formatDate(d.last_seen) }}</td>
            <td v-if="store.columns.firmware" class="mono small">{{ d.firmware || '-' }}</td>
            <td class="actions">
              <button class="btn sm" @click="openEdit(d)" data-testid="edit-device">Edit</button>
              <button class="btn sm warn" @click="confirmDelete(d)" :disabled="busy" data-testid="delete-device">Delete</button>
            </td>
          </tr>
          <tr v-if="pagedSortedFiltered.length === 0">
            <td :colspan="visibleColumnCount" class="state" data-testid="empty-state">
              <EmptyState title="No devices found" message="Try adjusting your filters or search." />
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pagination" data-testid="pagination">
      <button class="btn" :disabled="store.page <= 1" @click="prevPage" data-testid="prev-page">Prev</button>
      <span>Page {{ store.page }} / {{ totalPages || 1 }}</span>
      <button class="btn" :disabled="!hasNext" @click="nextPage" data-testid="next-page">Next</button>
    </div>
  </div>

  <!-- Add Device Dialog -->
  <div class="dialog-mask" v-if="addOpen">
    <div class="dialog">
      <div class="dialog-title">Add Device</div>
      <div class="dialog-body">
        <label class="field">
          <div class="label">IP</div>
          <input class="input" v-model="form.ip" placeholder="192.168.1.10" data-testid="add-ip" />
        </label>
        <label class="field">
          <div class="label">MAC</div>
          <input class="input" v-model="form.mac" placeholder="68C63A123456" data-testid="add-mac" />
        </label>
        <label class="field">
          <div class="label">Name</div>
          <input class="input" v-model="form.name" placeholder="Kitchen Light" data-testid="add-name" />
        </label>
        <label class="field">
          <div class="label">Type</div>
          <input class="input" v-model="form.type" placeholder="SHSW-1" data-testid="add-type" />
        </label>
        <div v-if="formError" class="form-error">{{ formError }}</div>
      </div>
      <div class="dialog-actions">
        <button class="btn" @click="closeAdd" :disabled="busy">Cancel</button>
        <button class="btn primary" @click="saveAdd" :disabled="busy || !formValid">Save</button>
      </div>
    </div>
  </div>

  <!-- Edit Device Dialog -->
  <div class="dialog-mask" v-if="editOpen">
    <div class="dialog">
      <div class="dialog-title">Edit Device</div>
      <div class="dialog-body">
        <label class="field">
          <div class="label">Name</div>
          <input class="input" v-model="editName" placeholder="Device name" data-testid="edit-name" />
        </label>
        <div v-if="formError" class="form-error">{{ formError }}</div>
      </div>
      <div class="dialog-actions">
        <button class="btn" @click="closeEdit" :disabled="busy">Cancel</button>
        <button class="btn primary" @click="saveEdit" :disabled="busy || !editName.trim()">Save</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { formatDate } from '@/utils/format'
import { computed, onMounted, ref, watch } from 'vue'
import { useDevicesStore } from '@/stores/devices'
import { createDevice, updateDevice, deleteDevice } from '@/api/devices'
import ErrorState from '@/components/shared/ErrorState.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import ColumnToggle from '@/components/shared/ColumnToggle.vue'
import type { Device } from '../api/types'

// Local sort descriptor
type Sort = { field: keyof Device | 'last_seen'; dir: 'asc' | 'desc' } | null

const loading = ref(false)
const error = ref<string | null>(null)
const totalPages = ref<number | null>(null)
const hasNext = ref(false)
const sort = ref<Sort>(null)
const store = useDevicesStore()
const busy = ref(false)
const addOpen = ref(false)
const editOpen = ref(false)
const editingId = ref<number | null>(null)
const editName = ref('')
const form = ref<{ ip: string; mac: string; name: string; type: string }>({ ip: '', mac: '', name: '', type: '' })
const formError = ref<string | null>(null)

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    await store.fetch()
    const p = store.meta?.pagination
    totalPages.value = p?.total_pages ?? null
    hasNext.value = !!p?.has_next
  } catch (e: any) {
    error.value = e?.message || 'Failed to load devices'
  } finally {
    loading.value = false
  }
}

onMounted(() => { store.initializeFromStorage(); fetchData() })
watch(() => store.page, fetchData)
watch(() => store.pageSize, fetchData)

function prevPage() { if (store.page > 1) store.page -= 1 }
function nextPage() { if (hasNext.value) store.page += 1 }

function toggleSort(field: Sort['field']) {
  if (!sort.value || sort.value.field !== field) {
    sort.value = { field, dir: 'asc' }
  } else if (sort.value.dir === 'asc') {
    sort.value.dir = 'desc'
  } else {
    sort.value = null
  }
}

// Use shared formatter from utils/format

const filtered = computed(() => {
  const q = store.search.trim().toLowerCase()
  if (!q) return store.items as Device[]
  return (store.items as Device[]).filter(d => {
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

// Column toggle wiring
const availableColumns = [
  { key: 'name', label: 'Name' },
  { key: 'ip', label: 'IP' },
  { key: 'mac', label: 'MAC' },
  { key: 'type', label: 'Type' },
  { key: 'status', label: 'Status' },
  { key: 'last_seen', label: 'Last Seen' },
  { key: 'firmware', label: 'Firmware' },
] as const

const columnsModel = computed({
  get: () => store.columns,
  set: (v) => store.setColumns(v),
})

const pageSizeModel = computed({
  get: () => store.pageSize,
  set: (v: number) => store.setPageSize(v),
})

const visibleColumnCount = computed(() => Object.values(store.columns).filter(Boolean).length || 1)

// CRUD helpers
function openAdd() { addOpen.value = true; form.value = { ip: '', mac: '', name: '', type: '' }; formError.value = null }
function closeAdd() { addOpen.value = false }
const formValid = computed(() => !!form.value.ip && !!form.value.mac)
async function saveAdd() {
  busy.value = true
  formError.value = null
  try {
    await createDevice({ ...form.value })
    closeAdd()
    await fetchData()
  } catch (e: any) {
    formError.value = e?.message || 'Failed to create device'
  } finally {
    busy.value = false
  }
}

function openEdit(d: Device) { editOpen.value = true; editingId.value = d.id; editName.value = d.name || ''; formError.value = null }
function closeEdit() { editOpen.value = false; editingId.value = null }
async function saveEdit() {
  if (!editingId.value) return
  busy.value = true
  formError.value = null
  try {
    await updateDevice(editingId.value, { name: editName.value.trim() })
    closeEdit()
    await fetchData()
  } catch (e: any) {
    formError.value = e?.message || 'Failed to update device'
  } finally {
    busy.value = false
  }
}

async function confirmDelete(d: Device) {
  if (!window.confirm(`Delete device "${d.name || d.ip}"?`)) return
  busy.value = true
  try {
    await deleteDevice(d.id)
    await fetchData()
  } catch (e: any) {
    alert(e?.message || 'Failed to delete device')
  } finally {
    busy.value = false
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
.toolbar :deep(.column-toggle) { position: relative }
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
.btn.sm { padding: 4px 8px; font-size: 12px; }
.btn.primary { background: #0ea5e9; color: #fff; border-color: #0ea5e9; }
.btn.warn { border-color: #fecaca; background: #fff7ed; }
.rowlink { color: #2563eb; text-decoration: none; }
.rowlink:hover { text-decoration: underline; }
.actions { white-space: nowrap; }

/* Dialog styles */
.dialog-mask { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.4); display: flex; align-items: center; justify-content: center; padding: 16px; }
.dialog { width: 460px; max-width: 100%; background: #fff; border: 1px solid #e5e7eb; border-radius: 10px; overflow: hidden; }
.dialog-title { padding: 12px; font-weight: 600; border-bottom: 1px solid #f1f5f9; background: #f8fafc; }
.dialog-body { padding: 12px; display: grid; gap: 10px; }
.field .label { font-size: 12px; color: #475569; margin-bottom: 4px; }
.input { width: 100%; padding: 8px 10px; border: 1px solid #cbd5e1; border-radius: 8px; }
.dialog-actions { display: flex; justify-content: flex-end; gap: 8px; padding: 10px 12px; border-top: 1px solid #f1f5f9; }
.form-error { color: #b91c1c; font-size: 12px; padding: 4px 0; }
</style>
