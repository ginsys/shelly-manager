<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" to="/">← Back</router-link>
      <h1 class="title">Device Details</h1>
      <div class="spacer" />
      <button class="control-btn on" @click="handleControl('on')" title="Turn On">ON</button>
      <button class="control-btn off" @click="handleControl('off')" title="Turn Off">OFF</button>
      <button class="control-btn" @click="handleControl('restart')" title="Restart">↻</button>
      <button class="btn" @click="refreshAll">Refresh</button>
    </div>

    <div class="card" v-if="loading">
      <div class="state">Loading...</div>
    </div>
    <div class="card" v-else-if="error">
      <div class="state error">{{ error }}</div>
    </div>

    <template v-else>
      <!-- Device Info -->
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

      <!-- Device Status -->
      <div class="card" v-if="status">
        <h2 class="section-title">Device Status</h2>
        <div class="status-grid">
          <div class="status-item">
            <div class="status-label">Online</div>
            <div :class="['status-value', status.online ? 'online' : 'offline']">
              {{ status.online ? 'Yes' : 'No' }}
            </div>
          </div>
          <div class="status-item" v-if="status.uptime !== undefined">
            <div class="status-label">Uptime</div>
            <div class="status-value">{{ formatUptime(status.uptime) }}</div>
          </div>
          <div class="status-item" v-if="status.temperature !== undefined">
            <div class="status-label">Temperature</div>
            <div class="status-value">{{ status.temperature }}°C</div>
          </div>
          <div class="status-item" v-if="status.wifi">
            <div class="status-label">WiFi</div>
            <div class="status-value">{{ status.wifi.ssid }} ({{ status.wifi.rssi }} dBm)</div>
          </div>
          <div class="status-item" v-if="status.cloud">
            <div class="status-label">Cloud</div>
            <div class="status-value">{{ status.cloud.connected ? 'Connected' : 'Disconnected' }}</div>
          </div>
          <div class="status-item" v-if="status.mqtt">
            <div class="status-label">MQTT</div>
            <div class="status-value">{{ status.mqtt.connected ? 'Connected' : 'Disconnected' }}</div>
          </div>
        </div>
      </div>

      <!-- Energy Metrics -->
      <div class="card" v-if="energy && hasEnergyData">
        <h2 class="section-title">Energy Metrics</h2>
        <div class="energy-grid">
          <div class="energy-item" v-if="energy.power !== undefined">
            <div class="energy-label">Power</div>
            <div class="energy-value">{{ energy.power.toFixed(1) }} W</div>
          </div>
          <div class="energy-item" v-if="energy.voltage !== undefined">
            <div class="energy-label">Voltage</div>
            <div class="energy-value">{{ energy.voltage.toFixed(1) }} V</div>
          </div>
          <div class="energy-item" v-if="energy.current !== undefined">
            <div class="energy-label">Current</div>
            <div class="energy-value">{{ energy.current.toFixed(2) }} A</div>
          </div>
          <div class="energy-item" v-if="energy.total !== undefined">
            <div class="energy-label">Total Consumed</div>
            <div class="energy-value">{{ energy.total.toFixed(2) }} kWh</div>
          </div>
          <div class="energy-item" v-if="energy.totalReturned !== undefined">
            <div class="energy-label">Total Returned</div>
            <div class="energy-value">{{ energy.totalReturned.toFixed(2) }} kWh</div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { getDevice, getDeviceStatus, getDeviceEnergy, controlDevice } from '../api/devices'
import type { Device } from '../api/types'
import type { DeviceStatus, DeviceEnergy } from '../api/devices'

const route = useRoute()
const loading = ref(false)
const error = ref<string | null>(null)
const d = ref<Device>({
  id: 0, ip: '', mac: '', type: '', name: '', firmware: '', status: '', last_seen: ''
})
const status = ref<DeviceStatus | null>(null)
const energy = ref<DeviceEnergy | null>(null)
let refreshInterval: ReturnType<typeof setInterval> | null = null

const hasEnergyData = computed(() => {
  if (!energy.value) return false
  return energy.value.power !== undefined ||
         energy.value.voltage !== undefined ||
         energy.value.current !== undefined ||
         energy.value.total !== undefined
})

function formatDate(iso?: string) {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const mins = Math.floor((seconds % 3600) / 60)

  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

async function fetchDevice() {
  try {
    const id = route.params.id as string
    d.value = await getDevice(id)
  } catch (e: any) {
    console.error('Failed to load device:', e)
  }
}

async function fetchStatus() {
  try {
    const id = route.params.id as string
    status.value = await getDeviceStatus(id)
  } catch (e: any) {
    // Status might not be available for all devices
    console.warn('Failed to load status:', e)
  }
}

async function fetchEnergy() {
  try {
    const id = route.params.id as string
    energy.value = await getDeviceEnergy(id)
  } catch (e: any) {
    // Energy metrics not available for all devices
    console.warn('Failed to load energy metrics:', e)
  }
}

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    await Promise.all([fetchDevice(), fetchStatus(), fetchEnergy()])
  } catch (e: any) {
    error.value = e?.message || 'Failed to load device'
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  await fetchData()
}

async function handleControl(action: string) {
  try {
    const id = route.params.id as string
    await controlDevice(id, action)
    // Refresh status after control command
    setTimeout(() => {
      fetchStatus()
      fetchDevice()
    }, 1000)
  } catch (e: any) {
    alert('Failed to control device: ' + (e?.message || 'Unknown error'))
  }
}

onMounted(() => {
  fetchData()
  // Auto-refresh every 30 seconds
  refreshInterval = setInterval(() => {
    fetchStatus()
    fetchEnergy()
  }, 30000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.control-btn { padding: 6px 12px; border: none; background: #e5e7eb; border-radius: 6px; cursor: pointer; font-weight: 500; }
.control-btn.on { background: #dcfce7; color: #065f46; }
.control-btn.off { background: #fee2e2; color: #991b1b; }
.control-btn:hover { opacity: 0.8; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; padding: 16px; }
.section-title { margin: 0 0 16px 0; font-size: 16px; font-weight: 600; }
.state { padding: 16px; text-align: center; color: #64748b; }
.state.error { color: #b91c1c; }
.details { display: grid; grid-template-columns: 240px 1fr; gap: 0; padding: 0; }
.row { display: contents; }
.k { padding: 10px 12px; background: #f8fafc; border-bottom: 1px solid #f1f5f9; font-weight: 600; }
.v { padding: 10px 12px; border-bottom: 1px solid #f1f5f9; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.small { font-size: 12px; color: #475569; }
.chip { padding: 2px 8px; border-radius: 999px; font-size: 12px; background: #e2e8f0; color: #334155; }
.chip.online { background: #dcfce7; color: #065f46; }
.chip.offline { background: #fee2e2; color: #991b1b; }

.status-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; }
.status-item { display: flex; flex-direction: column; gap: 4px; }
.status-label { font-size: 12px; color: #64748b; font-weight: 500; }
.status-value { font-size: 16px; font-weight: 600; color: #111827; }
.status-value.online { color: #065f46; }
.status-value.offline { color: #991b1b; }

.energy-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 16px; }
.energy-item { display: flex; flex-direction: column; gap: 4px; padding: 12px; background: #f9fafb; border-radius: 6px; }
.energy-label { font-size: 12px; color: #64748b; font-weight: 500; }
.energy-value { font-size: 18px; font-weight: 700; color: #2563eb; }
</style>
