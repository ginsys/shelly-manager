<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" :to="`/devices/${id}`" aria-label="Back to device">← Back</router-link>
      <h1 class="title">Device Configuration</h1>
      <div class="spacer" />
      <button class="btn" @click="load('stored')" :disabled="busy">Stored</button>
      <button class="btn" @click="load('live')" :disabled="busy">Live</button>
      <button class="btn" @click="load('normalized')" :disabled="busy">Live (Normalized)</button>
      <button class="btn" @click="load('typed')" :disabled="busy">Typed (Normalized)</button>
    </div>

    <div class="grid">
      <div class="card">
        <div class="card-title">Viewer</div>
        <div v-if="error" class="state error">{{ error }}</div>
        <div v-else-if="busy" class="state">Loading…</div>
        <pre v-else-if="payload" class="viewer">{{ pretty(payload) }}</pre>
        <div v-else class="state">Choose a source to view configuration</div>
      </div>

      <div class="card actions">
        <div class="card-title">Actions</div>
        <div class="section">
          <div class="section-title">Import</div>
          <textarea v-model="importText" class="textarea" placeholder="Paste JSON configuration…"></textarea>
          <div class="row">
            <button class="btn primary" @click="doImport" :disabled="busy || !importText.trim()">Import</button>
            <button class="btn" @click="checkImportStatus" :disabled="busy">Check Status</button>
            <div class="muted" v-if="importStatus">Status: {{ importStatus.status }}</div>
          </div>
        </div>
        <div class="section">
          <div class="section-title">Edit Stored Config</div>
          <textarea v-model="editText" class="textarea" placeholder="JSON editor for stored config"></textarea>
          <div class="row">
            <button class="btn" @click="loadEditor" :disabled="busy">Load Stored</button>
            <button class="btn primary" @click="saveEditor" :disabled="busy || !editText.trim()">Save</button>
            <div class="muted" v-if="saveMsg">{{ saveMsg }}</div>
          </div>
        </div>
        <div class="section">
          <div class="section-title">Export</div>
          <button class="btn" @click="doExport" :disabled="busy">Export Config</button>
          <div class="muted" v-if="exportId">Queued export id: {{ exportId }}</div>
        </div>
        <div class="section">
          <div class="section-title">Drift</div>
          <button class="btn" @click="doDrift" :disabled="busy">Detect Drift</button>
          <div class="muted" v-if="drift">Drift: {{ drift.has_drift ? 'Yes' : 'No' }}</div>
        </div>
        <div class="section">
          <div class="section-title">Apply Template</div>
          <div class="row">
            <input class="input" v-model="templateId" placeholder="Template ID" />
          </div>
          <textarea v-model="templateVars" class="textarea" placeholder='Template variables JSON, e.g. { "room": "kitchen" }'></textarea>
          <div class="row">
            <button class="btn primary" @click="doApplyTemplate" :disabled="busy || !templateId.trim()">Apply</button>
            <div class="muted" v-if="applyMsg">{{ applyMsg }}</div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-title">History</div>
        <div v-if="historyError" class="state error">{{ historyError }}</div>
        <div class="history" v-else>
          <button class="btn" @click="loadHistory" :disabled="busy">Refresh</button>
          <ul class="list" v-if="history.length">
            <li v-for="h in history" :key="h.id">
              <span class="mono small">{{ h.id }}</span> — {{ formatDate(h.timestamp) }} <span v-if="h.user" class="muted">by {{ h.user }}</span>
            </li>
          </ul>
          <div class="state" v-else>No history</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { getStoredConfig, updateStoredConfig, getLiveConfig, getLiveConfigNormalized, getTypedNormalizedConfig, importConfig, getImportStatus, exportConfig, detectDrift, getConfigHistory, applyTemplate, type ImportStatus, type DriftStatus, type ConfigHistoryItem } from '@/api/deviceConfig'

const route = useRoute()
const id = route.params.id as string

const busy = ref(false)
const error = ref<string | null>(null)
const payload = ref<any | null>(null)

const importText = ref('')
const importStatus = ref<ImportStatus | null>(null)
const exportId = ref<string | null>(null)
const drift = ref<DriftStatus | null>(null)

const history = ref<ConfigHistoryItem[]>([])
const historyError = ref<string | null>(null)
const editText = ref('')
const saveMsg = ref('')
const templateId = ref('')
const templateVars = ref('')
const applyMsg = ref('')

function pretty(v: unknown) { try { return JSON.stringify(v, null, 2) } catch { return String(v) } }
function formatDate(iso?: string) { if (!iso) return '-'; try { return new Date(iso).toLocaleString() } catch { return iso } }

async function load(which: 'stored' | 'live' | 'normalized' | 'typed') {
  busy.value = true
  error.value = null
  try {
    if (which === 'stored') payload.value = await getStoredConfig(id)
    else if (which === 'live') payload.value = await getLiveConfig(id)
    else if (which === 'normalized') payload.value = await getLiveConfigNormalized(id)
    else payload.value = await getTypedNormalizedConfig(id)
  } catch (e: any) {
    error.value = e?.message || 'Failed to load configuration'
  } finally {
    busy.value = false
  }
}

async function doImport() {
  busy.value = true
  error.value = null
  try {
    const parsed = JSON.parse(importText.value)
    await importConfig(id, parsed)
    importStatus.value = await getImportStatus(id)
  } catch (e: any) {
    error.value = e?.message || 'Import failed'
  } finally {
    busy.value = false
  }
}

async function checkImportStatus() {
  try { importStatus.value = await getImportStatus(id) } catch {}
}

async function doExport() {
  busy.value = true
  error.value = null
  try {
    const r = await exportConfig(id)
    exportId.value = r.export_id
  } catch (e: any) {
    error.value = e?.message || 'Export failed'
  } finally {
    busy.value = false
  }
}

async function doDrift() {
  busy.value = true
  error.value = null
  try {
    drift.value = await detectDrift(id)
  } catch (e: any) {
    error.value = e?.message || 'Drift detection failed'
  } finally {
    busy.value = false
  }
}

async function loadHistory() {
  historyError.value = null
  try {
    const r = await getConfigHistory(id)
    history.value = r.items
  } catch (e: any) {
    historyError.value = e?.message || 'Failed to load config history'
  }
}

async function loadEditor() {
  busy.value = true
  try {
    const c = await getStoredConfig(id)
    editText.value = JSON.stringify(c, null, 2)
    saveMsg.value = ''
  } catch (e: any) {
    error.value = e?.message || 'Failed to load stored config'
  } finally {
    busy.value = false
  }
}

async function saveEditor() {
  busy.value = true
  try {
    const parsed = JSON.parse(editText.value)
    await updateStoredConfig(id, parsed)
    saveMsg.value = 'Saved'
  } catch (e: any) {
    error.value = e?.message || 'Failed to save config'
  } finally {
    busy.value = false
    setTimeout(() => { saveMsg.value = '' }, 2500)
  }
}

async function doApplyTemplate() {
  busy.value = true
  try {
    const vars = templateVars.value.trim() ? JSON.parse(templateVars.value) : {}
    const tid = isNaN(Number(templateId.value)) ? templateId.value : Number(templateId.value)
    const r = await applyTemplate(id, tid as any, vars)
    applyMsg.value = r.applied ? 'Template applied' : 'No changes'
  } catch (e: any) {
    error.value = e?.message || 'Failed to apply template'
  } finally {
    busy.value = false
    setTimeout(() => { applyMsg.value = '' }, 2500)
  }
}
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; }
.toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }
.btn { padding: 6px 10px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; }
.btn.primary { background: #0ea5e9; color: #fff; border-color: #0ea5e9; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; }
.card-title { padding: 10px 12px; border-bottom: 1px solid #f1f5f9; font-weight: 600; background: #f8fafc; }
.state { padding: 16px; text-align: center; color: #64748b; }
.state.error { color: #b91c1c; }
.grid { display: grid; grid-template-columns: 1.2fr 1fr; gap: 12px; }
.viewer { margin: 0; padding: 12px; font-size: 12px; background: #0b1020; color: #d1e7ff; overflow: auto; min-height: 200px; }
.actions { display: flex; flex-direction: column; gap: 12px; }
.section { padding: 12px; border-bottom: 1px solid #f1f5f9; display: grid; gap: 8px; }
.section:last-child { border-bottom: 0; }
.section-title { font-weight: 600; color: #334155; }
.textarea { width: 100%; min-height: 100px; padding: 8px 10px; border: 1px solid #cbd5e1; border-radius: 8px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 12px; }
.input { width: 100%; padding: 8px 10px; border: 1px solid #cbd5e1; border-radius: 8px; }
.muted { color: #64748b; font-size: 12px; }
.history { padding: 12px; }
.list { margin: 8px 0 0; padding: 0 0 0 16px; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.small { font-size: 12px; }
</style>
