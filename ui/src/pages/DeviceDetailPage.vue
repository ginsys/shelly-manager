<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" to="/devices">← Back</router-link>
      <h1 class="title">Device Details</h1>
      <div class="spacer" />
      <button v-if="!formVisible" @click="showEditForm" class="btn-primary">✏️ Edit</button>
      <button v-if="!formVisible" @click="confirmDelete" class="btn-danger">🗑️ Delete</button>
    </div>

    <div class="card" v-if="loading && !d.id">
      <div class="state">Loading...</div>
    </div>
    <div class="card" v-else-if="error && !d.id">
      <div class="state error">{{ error }}</div>
    </div>

    <template v-else>
      <!-- Device Info Card -->
      <div class="card details">
        <div class="row"><div class="k">Name</div><div class="v">{{ d.name || '-' }}</div></div>
        <div class="row"><div class="k">Type</div><div class="v">{{ d.type || '-' }}</div></div>
        <div class="row"><div class="k">IP</div><div class="v">{{ d.ip || '-' }}</div></div>
        <div class="row"><div class="k">MAC</div><div class="v mono">{{ d.mac }}</div></div>
        <div class="row"><div class="k">Status</div><div class="v">
          <span :class="['chip', d.status]">{{ d.status || 'unknown' }}</span>
        </div></div>
        <div class="row"><div class="k">Last Seen</div><div class="v">{{ formatDate(d.last_seen) }}</div></div>
        <div class="row"><div class="k">Firmware</div><div class="v mono small">{{ d.firmware || '-' }}</div></div>
        <div class="row"><div class="k">Created</div><div class="v">{{ formatDate(d.created_at) }}</div></div>
        <div class="row"><div class="k">Updated</div><div class="v">{{ formatDate(d.updated_at) }}</div></div>
      </div>

      <!-- Device Control Card -->
      <div class="card section">
        <h2 class="section-title">Device Control</h2>
        <div class="control-buttons">
          <button @click="handleControl('on')" :disabled="controlLoading" class="control-btn on">
            {{ controlLoading && lastAction === 'on' ? 'Turning On...' : 'Turn On' }}
          </button>
          <button @click="handleControl('off')" :disabled="controlLoading" class="control-btn off">
            {{ controlLoading && lastAction === 'off' ? 'Turning Off...' : 'Turn Off' }}
          </button>
          <button @click="handleControl('toggle')" :disabled="controlLoading" class="control-btn toggle">
            {{ controlLoading && lastAction === 'toggle' ? 'Toggling...' : 'Toggle' }}
          </button>
          <button @click="handleControl('reboot')" :disabled="controlLoading" class="control-btn reboot">
            {{ controlLoading && lastAction === 'reboot' ? 'Rebooting...' : 'Reboot' }}
          </button>
        </div>
      </div>

      <!-- Device Status Card -->
      <div class="card section">
        <div class="section-header">
          <h2 class="section-title">Live Status</h2>
          <button @click="refreshStatus" :disabled="statusLoading" class="refresh-btn">
            {{ statusLoading ? '↻ Refreshing...' : '↻ Refresh' }}
          </button>
        </div>
        <div v-if="statusLoading && !status" class="state">Loading status...</div>
        <div v-else-if="statusError" class="state error">{{ statusError }}</div>
        <div v-else-if="status" class="status-grid">
          <div class="status-item" v-if="status.temperature">
            <div class="status-label">Temperature</div>
            <div class="status-value">{{ status.temperature }}°C</div>
          </div>
          <div class="status-item" v-if="status.uptime !== undefined">
            <div class="status-label">Uptime</div>
            <div class="status-value">{{ formatUptime(status.uptime) }}</div>
          </div>
          <div class="status-item" v-if="status.wifi">
            <div class="status-label">WiFi RSSI</div>
            <div class="status-value">{{ status.wifi.rssi }} dBm</div>
          </div>
          <div class="status-item" v-if="status.wifi">
            <div class="status-label">WiFi SSID</div>
            <div class="status-value">{{ status.wifi.ssid || '-' }}</div>
          </div>
          <div class="status-item full-width" v-if="status.switches && status.switches.length > 0">
            <div class="status-label">Switches</div>
            <div class="switches-list">
              <div v-for="sw in status.switches" :key="sw.id" class="switch-item">
                <span class="switch-id">Switch {{ sw.id }}:</span>
                <span :class="['switch-state', sw.output ? 'on' : 'off']">
                  {{ sw.output ? 'ON' : 'OFF' }}
                </span>
                <span v-if="sw.apower !== undefined" class="switch-power">{{ sw.apower }}W</span>
              </div>
            </div>
          </div>
        </div>
        <div v-else class="state">No status data available</div>
        <div class="polling-info">Auto-refreshing every 10 seconds</div>
      </div>

      <!-- Device Energy Card -->
      <div class="card section" v-if="supportsEnergyMetering">
        <h2 class="section-title">Energy Metrics</h2>
        <div v-if="energyLoading && !energy" class="state">Loading energy data...</div>
        <div v-else-if="energyError" class="state error">{{ energyError }}</div>
        <div v-else-if="energy" class="energy-grid">
          <div class="energy-item">
            <div class="energy-label">Current Power</div>
            <div class="energy-value">{{ energy.power }} W</div>
          </div>
          <div class="energy-item">
            <div class="energy-label">Voltage</div>
            <div class="energy-value">{{ energy.voltage }} V</div>
          </div>
          <div class="energy-item">
            <div class="energy-label">Current</div>
            <div class="energy-value">{{ energy.current }} A</div>
          </div>
          <div class="energy-item">
            <div class="energy-label">Total Energy</div>
            <div class="energy-value">{{ (energy.total / 1000).toFixed(2) }} kWh</div>
          </div>
          <div class="energy-item" v-if="energy.pf !== undefined">
            <div class="energy-label">Power Factor</div>
            <div class="energy-value">{{ energy.pf.toFixed(2) }}</div>
          </div>
        </div>
        <div v-else class="state">No energy data available</div>
      </div>
    </template>

    <!-- Edit Form Overlay -->
    <div v-if="formVisible" class="overlay" @click.self="closeForm">
      <DeviceForm
        :existing-device="d"
        :loading="formLoading"
        :error="formError"
        @submit="handleFormSubmit"
        @cancel="closeForm"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuasar } from 'quasar'
import {
  getDevice,
  updateDevice,
  deleteDevice,
  controlDevice,
  getDeviceStatus,
  getDeviceEnergy,
} from '../api/devices'
import type { Device, DeviceStatus, DeviceEnergy, UpdateDeviceRequest } from '../api/types'
import DeviceForm from '../components/devices/DeviceForm.vue'

const route = useRoute()
const router = useRouter()
const $q = useQuasar()

const loading = ref(false)
const error = ref<string | null>(null)
const d = ref<Device>({
  id: 0,
  ip: '',
  mac: '',
  type: '',
  name: '',
  firmware: '',
  status: '',
  last_seen: '',
})

// Control state
const controlLoading = ref(false)
const lastAction = ref<string>('')

// Status state
const status = ref<DeviceStatus | null>(null)
const statusLoading = ref(false)
const statusError = ref<string | null>(null)
const statusInterval = ref<number | null>(null)

// Energy state
const energy = ref<DeviceEnergy | null>(null)
const energyLoading = ref(false)
const energyError = ref<string | null>(null)

// Form state
const formVisible = ref(false)
const formLoading = ref(false)
const formError = ref<string | null>(null)

// Check if device supports energy metering based on type
const supportsEnergyMetering = computed(() => {
  const type = d.value.type?.toUpperCase() || ''
  return type.includes('SHSW') || type.includes('SHPLG') || type.includes('SHDM')
})

function formatDate(iso?: string) {
  if (!iso) return '-'
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    const id = route.params.id as string
    d.value = await getDevice(id)
  } catch (e: any) {
    error.value = e?.message || 'Failed to load device'
  } finally {
    loading.value = false
  }
}

async function refreshStatus() {
  if (!d.value.id) return

  statusLoading.value = true
  statusError.value = null
  try {
    status.value = await getDeviceStatus(d.value.id)
  } catch (e: any) {
    statusError.value = e?.message || 'Failed to load device status'
  } finally {
    statusLoading.value = false
  }
}

async function refreshEnergy() {
  if (!d.value.id || !supportsEnergyMetering.value) return

  energyLoading.value = true
  energyError.value = null
  try {
    energy.value = await getDeviceEnergy(d.value.id)
  } catch (e: any) {
    energyError.value = e?.message || 'Failed to load energy data'
  } finally {
    energyLoading.value = false
  }
}

async function handleControl(action: 'on' | 'off' | 'toggle' | 'reboot') {
  if (!d.value.id) return

  controlLoading.value = true
  lastAction.value = action
  try {
    await controlDevice(d.value.id, { action })
    $q.notify({ type: 'positive', message: `Device ${action} command sent successfully`, position: 'top' })
    // Refresh status after control action
    setTimeout(refreshStatus, 1000)
  } catch (e: any) {
    $q.notify({ type: 'negative', message: e?.message || `Failed to ${action} device`, position: 'top' })
  } finally {
    controlLoading.value = false
  }
}

// Form handlers
function showEditForm() {
  formError.value = null
  formVisible.value = true
}

function closeForm() {
  formVisible.value = false
  formLoading.value = false
  formError.value = null
}

async function handleFormSubmit(data: UpdateDeviceRequest) {
  formLoading.value = true
  formError.value = null

  try {
    await updateDevice(d.value.id, data)
    $q.notify({ type: 'positive', message: 'Device updated successfully', position: 'top' })
    closeForm()
    await fetchData()
  } catch (e: any) {
    formError.value = e?.message || 'Failed to update device'
  } finally {
    formLoading.value = false
  }
}

// Delete handler
function confirmDelete() {
  $q.dialog({
    title: 'Delete Device',
    message: `Are you sure you want to delete "${d.value.name || d.value.ip}"? This action cannot be undone.`,
    cancel: true,
    persistent: true,
    color: 'negative',
  }).onOk(async () => {
    try {
      await deleteDevice(d.value.id)
      $q.notify({ type: 'positive', message: 'Device deleted successfully', position: 'top' })
      router.push('/devices')
    } catch (e: any) {
      $q.notify({ type: 'negative', message: e?.message || 'Failed to delete device', position: 'top' })
    }
  })
}

// Setup polling
function startPolling() {
  // Initial fetch
  refreshStatus()
  if (supportsEnergyMetering.value) {
    refreshEnergy()
  }

  // Poll every 10 seconds
  statusInterval.value = window.setInterval(() => {
    refreshStatus()
    if (supportsEnergyMetering.value) {
      refreshEnergy()
    }
  }, 10000)
}

function stopPolling() {
  if (statusInterval.value) {
    clearInterval(statusInterval.value)
    statusInterval.value = null
  }
}

onMounted(async () => {
  await fetchData()
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
}
.title {
  font-size: 20px;
  margin: 0;
}
.spacer {
  flex: 1;
}
.btn {
  padding: 6px 10px;
  border: 1px solid #cbd5e1;
  background: #fff;
  border-radius: 6px;
  cursor: pointer;
  text-decoration: none;
  color: inherit;
}
.btn-primary {
  padding: 6px 12px;
  border: none;
  background: #3b82f6;
  color: white;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 500;
}
.btn-primary:hover {
  background: #2563eb;
}
.btn-danger {
  padding: 6px 12px;
  border: none;
  background: #ef4444;
  color: white;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 500;
}
.btn-danger:hover {
  background: #dc2626;
}
.card {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  overflow: hidden;
}
.state {
  padding: 16px;
  text-align: center;
  color: #64748b;
}
.state.error {
  color: #b91c1c;
}
.details {
  display: grid;
  grid-template-columns: 240px 1fr;
  gap: 0;
}
.row {
  display: contents;
}
.k {
  padding: 10px 12px;
  background: #f8fafc;
  border-bottom: 1px solid #f1f5f9;
  font-weight: 600;
}
.v {
  padding: 10px 12px;
  border-bottom: 1px solid #f1f5f9;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.small {
  font-size: 12px;
  color: #475569;
}
.chip {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  background: #e2e8f0;
  color: #334155;
}
.chip.online {
  background: #dcfce7;
  color: #065f46;
}
.chip.offline {
  background: #fee2e2;
  color: #991b1b;
}

.section {
  padding: 20px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.section-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 16px 0;
}
.control-buttons {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
.control-btn {
  padding: 10px 20px;
  border: none;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.2s;
}
.control-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.control-btn.on {
  background: #22c55e;
  color: white;
}
.control-btn.on:hover:not(:disabled) {
  background: #16a34a;
}
.control-btn.off {
  background: #ef4444;
  color: white;
}
.control-btn.off:hover:not(:disabled) {
  background: #dc2626;
}
.control-btn.toggle {
  background: #3b82f6;
  color: white;
}
.control-btn.toggle:hover:not(:disabled) {
  background: #2563eb;
}
.control-btn.reboot {
  background: #f59e0b;
  color: white;
}
.control-btn.reboot:hover:not(:disabled) {
  background: #d97706;
}

.refresh-btn {
  padding: 6px 12px;
  border: 1px solid #cbd5e1;
  background: #fff;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
}
.refresh-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.status-grid,
.energy-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
}
.status-item,
.energy-item {
  padding: 12px;
  background: #f8fafc;
  border-radius: 6px;
}
.status-item.full-width {
  grid-column: 1 / -1;
}
.status-label,
.energy-label {
  font-size: 12px;
  color: #64748b;
  margin-bottom: 4px;
}
.status-value,
.energy-value {
  font-size: 18px;
  font-weight: 600;
  color: #111827;
}
.polling-info {
  margin-top: 12px;
  font-size: 12px;
  color: #64748b;
  text-align: center;
}

.switches-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.switch-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.switch-id {
  font-weight: 500;
}
.switch-state {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}
.switch-state.on {
  background: #dcfce7;
  color: #065f46;
}
.switch-state.off {
  background: #fee2e2;
  color: #991b1b;
}
.switch-power {
  font-size: 12px;
  color: #64748b;
}

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
