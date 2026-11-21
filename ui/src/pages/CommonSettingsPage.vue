<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" to="/templates">← Back to Templates</router-link>
      <h1 class="title">Common settings</h1>
    </div>
    <p class="hint">
      Compare the last imported configuration of several devices and save what they share as a template. Left out:
      {{ PER_DEVICE_NOTE }}, and every secret. A device with no import is named and skipped.
    </p>

    <div v-if="loading" class="card state">Loading devices...</div>
    <div v-else-if="loadError" class="card state error" data-testid="devices-error">{{ loadError }}</div>
    <div v-else class="card">
      <h2>Devices</h2>
      <div class="mode">
        <label class="check">
          <input v-model="mode" type="radio" value="model" data-testid="mode-model" /> All devices of one model
        </label>
        <label class="check">
          <input v-model="mode" type="radio" value="pick" data-testid="mode-pick" /> A hand pick
        </label>
      </div>
      <select v-if="mode === 'model'" v-model="model" class="select" data-testid="model-select">
        <option value="">Choose a model</option>
        <option v-for="m in models" :key="m.name" :value="m.name">
          {{ m.name }} ({{ m.count }} {{ m.count === 1 ? 'device' : 'devices' }})
        </option>
      </select>
      <div v-else class="picks">
        <label v-for="d in devices" :key="d.id" class="check">
          <input v-model="picked" type="checkbox" :value="d.id" :data-testid="`device-${d.id}`" />
          {{ d.name || `Device ${d.id}` }} <span class="muted">{{ deviceModel(d) }}</span>
        </label>
      </div>
      <div class="actions">
        <button class="btn primary" data-testid="compare" :disabled="chosen.length === 0 || comparing" @click="compare">
          {{ comparing ? 'Comparing...' : 'Compare' }}
        </button>
      </div>
    </div>

    <p v-if="compareError" class="state error" role="alert" data-testid="common-error">{{ compareError }}</p>

    <div v-if="result" class="card" data-testid="common-result">
      <h2>Common settings</h2>
      <p data-testid="common-compared">
        <template v-if="result.compared.length === 0">No device could be compared.</template>
        <template v-else-if="result.compared.length === 1">Only one device was compared; this is not a comparison.</template>
        <template v-else>{{ result.compared.length }} devices compared.</template>
      </p>
      <div v-if="result.skipped.length > 0" data-testid="common-skipped">
        <p>Not compared:</p>
        <ul>
          <li v-for="s in result.skipped" :key="s.deviceId">{{ s.deviceName || `Device ${s.deviceId}` }}: {{ s.reason }}</li>
        </ul>
      </div>
      <p v-if="Object.keys(result.common).length === 0" class="hint" data-testid="common-none">
        The compared devices share no setting that a template can hold.
      </p>
      <template v-else>
        <ConfigView :config="result.common" :expand-all="true" />
        <button class="btn primary" data-testid="common-create" @click="showSave = true">
          Create template from these
        </button>
      </template>
    </div>

    <CreateTemplateFromImportDialog
      v-if="showSave && result"
      :common="{ typed: result.common, deviceType: result.deviceType, differing: result.differing }"
      title="Create template from the common settings"
      @close="showSave = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { listDevices } from '@/api/devices'
import type { Device } from '@/api/types'
import { getCommonSettings, type CommonSettings } from '@/api/commonSettings'
import ConfigView from '@/components/config/ConfigView.vue'
import CreateTemplateFromImportDialog from '@/components/config/CreateTemplateFromImportDialog.vue'
import { PER_DEVICE_NOTE } from '@/utils/templateFromImport'

const devices = ref<Device[]>([])
const loading = ref(true)
const loadError = ref('')
const mode = ref<'model' | 'pick'>('model')
const model = ref('')
const picked = ref<number[]>([])
const comparing = ref(false)
const compareError = ref('')
const result = ref<CommonSettings | null>(null)
const showSave = ref(false)

// An answer that arrives after the page was left belongs to nobody.
let alive = true
onUnmounted(() => {
  alive = false
})

// The model is in the device's settings text; the type is the human name a device falls back to.
function deviceModel(d: Device): string {
  try {
    const named = JSON.parse(d.settings || '{}').model
    if (typeof named === 'string' && named) return named
  } catch {
    // A settings text that is not JSON names no model.
  }
  return d.type
}

const models = computed(() => {
  const counts = new Map<string, number>()
  for (const d of devices.value) counts.set(deviceModel(d), (counts.get(deviceModel(d)) ?? 0) + 1)
  return [...counts].map(([name, count]) => ({ name, count })).sort((a, b) => a.name.localeCompare(b.name))
})

const chosen = computed(() =>
  mode.value === 'model'
    ? devices.value.filter((d) => model.value !== '' && deviceModel(d) === model.value).map((d) => d.id)
    : devices.value.filter((d) => picked.value.includes(d.id)).map((d) => d.id),
)

// A result belongs to the devices it was asked for.
watch(chosen, () => {
  result.value = null
  compareError.value = ''
  showSave.value = false
})

async function compare() {
  comparing.value = true
  compareError.value = ''
  result.value = null
  try {
    const answer = await getCommonSettings(chosen.value)
    if (alive) result.value = answer
  } catch (e: any) {
    if (alive) compareError.value = e?.response?.data?.error?.message || e?.message || 'Failed to compare the devices'
  } finally {
    if (alive) comparing.value = false
  }
}

onMounted(async () => {
  try {
    const list = await listDevices({ all: true })
    if (alive) devices.value = list.items
  } catch (e: any) {
    if (alive) loadError.value = e?.message || 'Failed to load devices'
  } finally {
    if (alive) loading.value = false
  }
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.btn.primary { background: #2563eb; border-color: #2563eb; color: #fff; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 16px; display: flex; flex-direction: column; gap: 8px; }
.state { color: #64748b; }
.state.error { color: #b91c1c; }
.hint { color: #475569; font-size: 13px; margin: 0; }
.muted { color: #64748b; font-size: 12px; }
.mode { display: flex; gap: 16px; }
.check { display: inline-flex; align-items: center; gap: 6px; }
.picks { display: flex; flex-direction: column; gap: 4px; max-height: 320px; overflow-y: auto; }
.select { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px; max-width: 320px; }
.actions { display: flex; gap: 8px; }
</style>
