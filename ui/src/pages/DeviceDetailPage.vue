<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" to="/" aria-label="Back to devices">← Back</router-link>
      <h1 class="title">Device Details</h1>
      <div class="spacer" />

      <button class="btn" :disabled="actionBusy || !d.id" @click="doControl('on')">Power On</button>
      <button class="btn" :disabled="actionBusy || !d.id" @click="doControl('off')">Power Off</button>
      <button class="btn warn" :disabled="actionBusy || !d.id" @click="doControl('restart')">Restart</button>
      <button class="btn" :disabled="editOpen || !d.id" @click="openEdit()">Edit</button>
      <router-link class="btn" v-if="d.id" :to="`/devices/${d.id}/config`" aria-label="View device configuration">Config</router-link>
    </div>

    <div class="card" v-if="loading">
      <div class="state">Loading...</div>
    </div>
    <div class="card" v-else-if="error">
      <div class="state error">{{ error }}</div>
    </div>

    <div v-else class="grid">
      <div class="card details">
        <div class="row"><div class="k">Name</div><div class="v">{{ d.name || '-' }}</div></div>
        <div class="row"><div class="k">Type</div><div class="v">{{ d.type || '-' }}</div></div>
        <div class="row"><div class="k">IP</div><div class="v">{{ d.ip || '-' }}</div></div>
        <div class="row"><div class="k">MAC</div><div class="v mono">{{ d.mac }}</div></div>
        <div class="row"><div class="k">Status</div><div class="v">
          <span :class="['chip', liveStatus.status]">{{ liveStatus.status || d.status || 'unknown' }}</span>
          <span class="muted" v-if="liveStatus.last_seen">(last {{ formatDate(liveStatus.last_seen) }})</span>
        </div></div>
        <div class="row"><div class="k">Firmware</div><div class="v mono small">{{ d.firmware || '-' }}</div></div>
        <div class="row"><div class="k">Created</div><div class="v">{{ formatDate(d.created_at) }}</div></div>
        <div class="row"><div class="k">Updated</div><div class="v">{{ formatDate(d.updated_at) }}</div></div>
      </div>

      <div class="card metrics">
        <div class="card-title">Energy Metrics</div>
        <div class="metrics-grid" v-if="energy">
          <div class="metric"><div class="label">Power</div><div class="value">{{ formatNum(energy.power_w) }} W</div></div>
          <div class="metric"><div class="label">Energy</div><div class="value">{{ formatNum(energy.energy_wh) }} Wh</div></div>
          <div class="metric"><div class="label">Voltage</div><div class="value">{{ formatNum(energy.voltage_v) }} V</div></div>
          <div class="metric"><div class="label">Current</div><div class="value">{{ formatNum(energy.current_a) }} A</div></div>
        </div>
        <div class="state" v-else>Awaiting metrics…</div>
      </div>

      <div class="card caps">
        <div class="card-title">Capabilities</div>
        <div class="caps-grid" v-if="capabilities && Object.keys(capabilities).length">
          <div v-for="(enabled, key) in capabilities" :key="key" class="cap" :class="{ on: enabled }">
            <span class="dot" /> <span class="name">{{ formatCap(key) }}</span>
          </div>
        </div>
        <div class="state" v-else>No capabilities reported</div>
      </div>

      <div class="card config">
        <div class="card-title">Configuration (Read-only)</div>
        <div class="config-actions">
          <button class="btn" :disabled="configBusy" @click="loadConfig('stored')">Stored</button>
          <button class="btn" :disabled="configBusy" @click="loadConfig('live')">Live</button>
          <button class="btn" :disabled="configBusy" @click="loadConfig('normalized')">Live (Normalized)</button>
          <button class="btn" :disabled="configBusy" @click="loadConfig('typed')">Typed (Normalized)</button>
        </div>
        <pre class="config-view" v-if="configPayload">{{ pretty(configPayload) }}</pre>
        <div class="state" v-else>Choose a view to load configuration…</div>
      </div>
    </div>
  </div>
  
  <!-- Edit Dialog -->
  <div class="dialog-mask" v-if="editOpen">
    <div class="dialog">
      <div class="dialog-title">Edit Device</div>
      <div class="dialog-body">
        <label class="field">
          <div class="label">Name</div>
          <input class="input" v-model="editName" placeholder="Device name" />
        </label>
      </div>
      <div class="dialog-actions">
        <button class="btn" @click="closeEdit" :disabled="saving">Cancel</button>
        <button class="btn primary" @click="saveEdit" :disabled="saving || !editName.trim()">Save</button>
      </div>
    </div>
  </div>
  
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getDevice, getDeviceStatus, getDeviceEnergy, controlDevice, updateDevice, getDeviceCapabilities, type DeviceAction } from '../api/devices'
import { getStoredConfig, getLiveConfig, getLiveConfigNormalized, getTypedNormalizedConfig } from '../api/deviceConfig'
import type { Device } from '../api/types'
import { formatDate } from '@/utils/format'

const route = useRoute()
const loading = ref(false)
const error = ref<string | null>(null)

const d = ref<Device>({
  id: 0, ip: '', mac: '', type: '', name: '', firmware: '', status: '', last_seen: ''
})

// Live status + energy
const liveStatus = ref<{ status?: string; last_seen?: string }>({})
const energy = ref<{ power_w?: number; energy_wh?: number; voltage_v?: number; current_a?: number; timestamp?: string } | null>(null)
const capabilities = ref<Record<string, boolean> | null>(null)
let statusTimer: any = 0
let energyTimer: any = 0

// Actions state
const actionBusy = ref(false)
const configBusy = ref(false)

// Config view
const configPayload = ref<any | null>(null)

// Edit dialog state
const editOpen = ref(false)
const editName = ref('')
const saving = ref(false)

// Use shared formatter from utils/format
function formatNum(v?: number) {
  if (v === undefined || v === null) return '-'
  try { return Number(v).toFixed(2) } catch { return String(v) }
}
function pretty(v: unknown) {
  try { return JSON.stringify(v, null, 2) } catch { return String(v) }
}

async function fetchDevice() {
  loading.value = true
  error.value = null
  try {
    const id = route.params.id as string
    d.value = await getDevice(id)
    editName.value = d.value.name || ''
  } catch (e: any) {
    error.value = e?.message || 'Failed to load device'
  } finally {
    loading.value = false
  }
}

async function pollStatus() {
  if (!d.value?.id) return
  try {
    liveStatus.value = await getDeviceStatus(d.value.id)
  } catch {}
}
async function pollEnergy() {
  if (!d.value?.id) return
  try {
    energy.value = await getDeviceEnergy(d.value.id)
  } catch {}
}

async function pollCapabilities() {
  if (!d.value?.id) return
  try {
    capabilities.value = await getDeviceCapabilities(d.value.id)
  } catch {}
}

async function doControl(action: DeviceAction) {
  if (!d.value?.id) return
  actionBusy.value = true
  try {
    await controlDevice(d.value.id, action)
    // Refresh status shortly after control
    setTimeout(pollStatus, 800)
  } catch (e: any) {
    error.value = e?.message || `Failed to ${action} device`
  } finally {
    actionBusy.value = false
  }
}

async function loadConfig(which: 'stored' | 'live' | 'normalized' | 'typed') {
  if (!d.value?.id) return
  configBusy.value = true
  error.value = null
  try {
    if (which === 'stored') configPayload.value = await getStoredConfig(d.value.id)
    else if (which === 'live') configPayload.value = await getLiveConfig(d.value.id)
    else if (which === 'normalized') configPayload.value = await getLiveConfigNormalized(d.value.id)
    else configPayload.value = await getTypedNormalizedConfig(d.value.id)
  } catch (e: any) {
    error.value = e?.message || 'Failed to load configuration'
  } finally {
    configBusy.value = false
  }
}

function openEdit() {
  editOpen.value = true
}
function closeEdit() {
  editOpen.value = false
}
async function saveEdit() {
  if (!d.value?.id) return
  saving.value = true
  error.value = null
  try {
    const updated = await updateDevice(d.value.id, { name: editName.value.trim() })
    d.value = { ...d.value, ...updated }
    closeEdit()
  } catch (e: any) {
    error.value = e?.message || 'Failed to save changes'
  } finally {
    saving.value = false
  }
}

function formatCap(key: string) {
  return key.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}

onMounted(async () => {
  await fetchDevice()
  // Start polling
  await pollStatus()
  await pollEnergy()
  await pollCapabilities()
  statusTimer = setInterval(pollStatus, 10000)
  energyTimer = setInterval(pollEnergy, 15000)
})
onUnmounted(() => {
  if (statusTimer) clearInterval(statusTimer)
  if (energyTimer) clearInterval(energyTimer)
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; }
.toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }
.btn { padding: 6px 10px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.btn:disabled { opacity: 0.6; cursor: not-allowed; }
.btn.warn { border-color: #fecaca; background: #fff7ed; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; }
.card-title { padding: 10px 12px; border-bottom: 1px solid #f1f5f9; font-weight: 600; background: #f8fafc; }
.grid { display: grid; grid-template-columns: 1fr; gap: 12px; }
@media (min-width: 1000px) { .grid { grid-template-columns: 1.2fr 1fr; } .config { grid-column: 1 / -1; } }
.state { padding: 16px; text-align: center; color: #64748b; }
.state.error { color: #b91c1c; }
.details { display: grid; grid-template-columns: 240px 1fr; gap: 0; }
.row { display: contents; }
.k { padding: 10px 12px; background: #f8fafc; border-bottom: 1px solid #f1f5f9; font-weight: 600; }
.v { padding: 10px 12px; border-bottom: 1px solid #f1f5f9; display: flex; align-items: center; gap: 8px; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.small { font-size: 12px; color: #475569; }
.muted { color: #64748b; font-size: 12px; }
.chip { padding: 2px 8px; border-radius: 999px; font-size: 12px; background: #e2e8f0; color: #334155; }
.chip.online { background: #dcfce7; color: #065f46; }
.chip.offline { background: #fee2e2; color: #991b1b; }
.metrics { display: flex; flex-direction: column; gap: 6px; }
.metrics-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; padding: 12px; }
.metric { border: 1px solid #f1f5f9; border-radius: 8px; padding: 10px; }
.metric .label { color: #64748b; font-size: 12px; }
.metric .value { font-weight: 600; font-size: 16px; }
.caps-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; padding: 12px; }
.cap { display: flex; align-items: center; gap: 8px; padding: 8px; border: 1px solid #f1f5f9; border-radius: 8px; }
.cap .dot { width: 8px; height: 8px; border-radius: 999px; background: #94a3b8; }
.cap.on .dot { background: #22c55e; }
.cap .name { font-size: 12px; color: #334155; }
.config-actions { display: flex; gap: 8px; padding: 10px 12px; border-bottom: 1px solid #f1f5f9; flex-wrap: wrap; }
.config-view { margin: 0; padding: 12px; font-size: 12px; background: #0b1020; color: #d1e7ff; overflow: auto; max-height: 360px; }

/* Dialog */
.dialog-mask { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.4); display: flex; align-items: center; justify-content: center; padding: 16px; }
.dialog { width: 420px; max-width: 100%; background: #fff; border: 1px solid #e5e7eb; border-radius: 10px; overflow: hidden; }
.dialog-title { padding: 12px; font-weight: 600; border-bottom: 1px solid #f1f5f9; background: #f8fafc; }
.dialog-body { padding: 12px; display: grid; gap: 10px; }
.field .label { font-size: 12px; color: #475569; margin-bottom: 4px; }
.input { width: 100%; padding: 8px 10px; border: 1px solid #cbd5e1; border-radius: 8px; }
.dialog-actions { display: flex; justify-content: flex-end; gap: 8px; padding: 10px 12px; border-top: 1px solid #f1f5f9; }
.btn.primary { background: #0ea5e9; color: #fff; border-color: #0ea5e9; }
</style>
