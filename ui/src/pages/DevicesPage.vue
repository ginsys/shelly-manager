<template>
  <div class="page" data-testid="devices-page">
    <div class="toolbar">
      <h1 class="title" data-testid="page-title">Devices</h1>
      <div class="spacer" />
      <button
        class="btn"
        :disabled="store.loading"
        title="Refresh the list (R)"
        aria-keyshortcuts="r"
        @click="handleRefresh"
        data-testid="refresh-btn"
      >
        {{ store.loading ? 'Refreshing...' : 'Refresh' }}
      </button>
      <ColumnToggle
        :columns="columnOptions"
        :visibleColumns="store.visibleColumns"
        @toggle="handleColumnToggle"
        @selectAll="handleSelectAllColumns"
        @selectNone="handleSelectNoneColumns"
      />
      <button
        class="btn"
        :disabled="store.selectedCount === 0"
        @click="openBulkDialog"
        data-testid="bulk-ops-btn"
      >
        {{ bulkButtonLabel }}
      </button>
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

    <ErrorDisplay
      :error="actionError"
      :show-retry="false"
      data-testid="devices-action-error"
      @dismiss="clearActionError"
    />

    <div class="card" data-testid="device-list">
      <!-- A refresh keeps the rows on screen until the new answer lands; only a page with nothing yet says Loading. -->
      <div v-if="store.loading && store.items.length === 0" class="state" data-testid="loading-state">Loading...</div>

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

      <table v-else class="table" :class="{ busy: store.loading }" :aria-busy="store.loading" data-testid="devices-table">
        <thead>
          <tr>
            <th v-if="store.visibleColumns.select" class="select-col">
              <input
                type="checkbox"
                :checked="allVisibleSelected"
                :indeterminate="someVisibleSelected && !allVisibleSelected"
                aria-label="Select all devices on this page"
                data-testid="select-all-devices"
                @change="handleToggleVisible"
              />
            </th>
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
            <td v-if="store.visibleColumns.select" class="select-col">
              <input
                type="checkbox"
                :checked="store.isSelected(d.id)"
                :aria-label="`Select ${d.name || d.ip}`"
                data-testid="select-device"
                @change="store.toggleSelect(d)"
              />
            </td>
            <td v-if="store.visibleColumns.name">
              <router-link :to="`/devices/${d.id}`" class="rowlink" data-testid="device-link">{{ d.name || '-' }}</router-link>
            </td>
            <td v-if="store.visibleColumns.ip">{{ d.ip || '-' }}</td>
            <td v-if="store.visibleColumns.mac" class="mono">{{ d.mac }}</td>
            <td v-if="store.visibleColumns.type">{{ d.type }}</td>
            <td v-if="store.visibleColumns.status">
              <span :class="['chip', shownDeviceStatus(d).cssClass]" :title="shownDeviceStatus(d).title" data-testid="device-status">{{ shownDeviceStatus(d).label }}</span>
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
      <div class="dialog-card">
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

    <BulkOperationsDialog
      :isOpen="showBulkDialog"
      :selectedDevices="store.selectedDevices"
      :checking="store.checkingSelection"
      :removedCount="store.removedFromSelection"
      @close="showBulkDialog = false"
      @complete="store.fetchDevices()"
      @remove="store.deselect([$event])"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { createDevice, updateDevice, deleteDevice, controlDevice, isAbortError } from '../api/devices'
import type { Device } from '../api/types'
import { useDevicesStore } from '../stores/devices'
import DeviceForm from '../components/devices/DeviceForm.vue'
import ErrorState from '../components/shared/ErrorState.vue'
import ErrorDisplay from '../components/shared/ErrorDisplay.vue'
import EmptyState from '../components/shared/EmptyState.vue'
import ColumnToggle from '../components/shared/ColumnToggle.vue'
import BulkOperationsDialog from '../components/bulk/BulkOperationsDialog.vue'
import { usePageError } from '../composables/useNotice'
import { shownDeviceStatus } from '../utils/deviceStatus'

const store = useDevicesStore()
// A failed delete or control command. The list load keeps its own ErrorState above.
const showBulkDialog = ref(false)
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
// A failure shows in the app notice when the page was left, or when a dialog covers the panel.
const { error: actionError, setError: setActionError, clearError: clearActionError } = usePageError(
  () => showCreateDialog.value || showEditDialog.value || showBulkDialog.value
)
const editingDevice = ref<Device | null>(null)
const deviceFormRef = ref<InstanceType<typeof DeviceForm> | null>(null)

// Column configuration
const columnOptions = [
  { key: 'select', label: 'Select' },
  { key: 'name', label: 'Name' },
  { key: 'ip', label: 'IP Address' },
  { key: 'mac', label: 'MAC Address' },
  { key: 'type', label: 'Type' },
  { key: 'status', label: 'Status' },
  { key: 'last_seen', label: 'Last Seen' },
  { key: 'firmware', label: 'Firmware' },
  { key: 'actions', label: 'Actions' }
]

// The button and the R key share this: a load that is already running is not started twice.
function handleRefresh() {
  if (store.loading) return
  store.fetchDevices()
}

function isTypingTarget(target: EventTarget | null) {
  const el = target as HTMLElement | null
  if (!el || !el.tagName) return false
  return ['INPUT', 'SELECT', 'TEXTAREA'].includes(el.tagName) || el.isContentEditable
}

// R refreshes, unless the key is for something else: a modifier, a held key, a field being
// typed in, or a dialog of this page.
function handleRefreshKey(event: KeyboardEvent) {
  if (event.key.toLowerCase() !== 'r') return
  if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return
  if (event.repeat || event.defaultPrevented) return
  if (isTypingTarget(event.target)) return
  if (showCreateDialog.value || showEditDialog.value || showBulkDialog.value) return
  handleRefresh()
}

onMounted(() => {
  store.loadPreferences()
  store.fetchDevices()
  window.addEventListener('keydown', handleRefreshKey)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleRefreshKey)
})

// Watch for page/pageSize changes and refetch
watch(() => store.page, () => store.fetchDevices())
watch(() => store.pageSize, () => {
  store.setPageSize(store.pageSize)
  store.fetchDevices()
})

// The dialog is the one place the selection is sent, so it is checked against the server here: a
// device deleted elsewhere is dropped before an operation can be started on it.
function openBulkDialog() {
  showBulkDialog.value = true
  void store.checkSelection()
}

// Selection covers the devices ticked so far, also those on other pages. The header checkbox
// acts on the rows shown, never on the whole fleet.
const bulkButtonLabel = computed(() =>
  store.selectedCount > 0 ? `Bulk Operations (${store.selectedCount})` : 'Bulk Operations'
)
const allVisibleSelected = computed(
  () => store.sortedItems.length > 0 && store.sortedItems.every((d) => store.isSelected(d.id))
)
const someVisibleSelected = computed(() => store.sortedItems.some((d) => store.isSelected(d.id)))

function handleToggleVisible(event: Event) {
  if ((event.target as HTMLInputElement).checked) {
    store.selectAll(store.sortedItems)
  } else {
    store.deselect(store.sortedItems.map((d) => d.id))
  }
}

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
      await updateDevice(editingDevice.value.id, { ...data, updated_at: editingDevice.value.updated_at })
    } else {
      await createDevice(data)
    }
    closeDialogs()
    await store.fetchDevices()
  } catch (e: any) {
    // The dialog shows its own failure. With the dialog gone (cancelled, or the page left) there is
    // no form to tell, so the failure goes to the page panel, or to the notice once the page is gone.
    if (deviceFormRef.value) {
      deviceFormRef.value.setError(e?.message || 'Operation failed')
    } else {
      setActionError(e, { action: 'Save device', resource: 'Device' })
    }
  }
}

async function handleDelete(id: number) {
  if (!confirm('Are you sure you want to delete this device?')) return
  clearActionError()
  try {
    await deleteDevice(id)
    store.deselect([id])
    await store.fetchDevices()
  } catch (e: any) {
    if (isAbortError(e)) return
    setActionError(e, { action: 'Delete device', resource: 'Device', resourceId: id })
  }
}

async function handleControl(id: number, action: string) {
  clearActionError()
  try {
    await controlDevice(id, { action: action as any })
    // Optionally refresh device status
    setTimeout(() => store.fetchDevices(), 500)
  } catch (e: any) {
    setActionError(e, { action: 'Control device', resource: 'Device', resourceId: id })
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
.table.busy { opacity: 0.6; }
.table th, .table td { text-align: left; padding: 10px 12px; border-bottom: 1px solid #f1f5f9; }
.table th { background: #f8fafc; cursor: pointer; user-select: none; white-space: nowrap; }
.table th.select-col, .table td.select-col { width: 32px; cursor: default; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.small { font-size: 12px; color: #475569; }
.chip { padding: 2px 8px; border-radius: 999px; font-size: 12px; background: #e2e8f0; color: #334155; }
.chip.online { background: #dcfce7; color: #065f46; }
.chip.offline { background: #fee2e2; color: #991b1b; }
.chip.stale { background: #fef3c7; color: #92400e; }
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
/* Not `.modal`: a scoped rule reaches the root element of a child component, and the bulk dialog's backdrop is `.modal`. */
.dialog-card { background: white; padding: 24px; border-radius: 8px; max-width: 600px; width: 90%; max-height: 90vh; overflow-y: auto; }
.dialog-card h2 { margin-top: 0; }
</style>
