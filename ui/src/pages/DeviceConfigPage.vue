<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" :to="`/devices/${deviceId}`">← Back to Device</router-link>
      <h1 class="title">Device Configuration</h1>
      <div class="spacer" />
      <button class="btn" @click="showImportDialog = true">Import</button>
      <button class="btn" @click="handleExport">Export</button>
      <button class="btn" @click="checkDrift">Check Drift</button>
      <button class="btn primary" @click="handleRefresh">Refresh</button>
    </div>

    <!-- Loading/Error states -->
    <div v-if="store.loading" class="card state">Loading...</div>
    <div v-else-if="store.error" class="card state error">{{ store.error }}</div>

    <!-- Drift Alert -->
    <div v-if="store.drift?.hasDrift" class="card drift-alert">
      <h3>⚠️ Configuration Drift Detected</h3>
      <p>The stored configuration differs from the live device configuration.</p>
      <div v-if="store.drift.driftFields" class="drift-fields">
        <strong>Changed fields:</strong> {{ store.drift.driftFields.join(', ') }}
      </div>
      <button class="btn" @click="syncFromLive">Sync from Device</button>
    </div>

    <!-- Configuration Viewer -->
    <div v-if="store.storedConfig || store.liveConfig" class="card">
      <div class="config-tabs">
        <button
          :class="['tab', { active: activeTab === 'stored' }]"
          @click="activeTab = 'stored'"
        >
          Stored Config
        </button>
        <button
          :class="['tab', { active: activeTab === 'live' }]"
          @click="activeTab = 'live'"
        >
          Live Config
        </button>
        <button
          :class="['tab', { active: activeTab === 'diff' }]"
          @click="activeTab = 'diff'"
        >
          Comparison
        </button>
      </div>

      <!-- Stored Configuration Tab -->
      <div v-if="activeTab === 'stored' && store.storedConfig" class="config-content">
        <div class="config-actions">
          <button class="btn" @click="editMode = !editMode">
            {{ editMode ? 'Cancel Edit' : 'Edit' }}
          </button>
          <button v-if="editMode" class="btn primary" @click="handleSave">
            Save Changes
          </button>
        </div>
        <div v-if="editMode" class="config-editor">
          <textarea
            v-model="editedConfig"
            class="config-textarea"
            spellcheck="false"
          />
        </div>
        <pre v-else class="config-view">{{ JSON.stringify(store.storedConfig, null, 2) }}</pre>
      </div>

      <!-- Live Configuration Tab -->
      <div v-if="activeTab === 'live' && store.liveConfig" class="config-content">
        <pre class="config-view">{{ JSON.stringify(store.liveConfig, null, 2) }}</pre>
      </div>

      <!-- Comparison Tab -->
      <div v-if="activeTab === 'diff'" class="config-content">
        <div class="diff-view">
          <div class="diff-column">
            <h4>Stored</h4>
            <pre class="config-view">{{ JSON.stringify(store.storedConfig, null, 2) }}</pre>
          </div>
          <div class="diff-column">
            <h4>Live</h4>
            <pre class="config-view">{{ JSON.stringify(store.liveConfig, null, 2) }}</pre>
          </div>
        </div>
      </div>
    </div>

    <!-- Import Dialog -->
    <div v-if="showImportDialog" class="modal-overlay" @click.self="showImportDialog = false">
      <div class="modal">
        <h2>Import Configuration</h2>
        <textarea
          v-model="importConfigText"
          class="config-textarea"
          placeholder="Paste JSON configuration here..."
          spellcheck="false"
        />
        <div class="modal-actions">
          <button class="btn" @click="showImportDialog = false">Cancel</button>
          <button class="btn primary" @click="handleImport">Import</button>
        </div>
      </div>
    </div>

    <!-- Import Status -->
    <div v-if="store.importStatus" class="card import-status">
      <h3>Import Status</h3>
      <div class="status-row">
        <span>Status:</span>
        <span :class="['status-badge', store.importStatus.status]">
          {{ store.importStatus.status }}
        </span>
      </div>
      <div v-if="store.importStatus.progress" class="progress-bar">
        <div class="progress-fill" :style="{ width: store.importStatus.progress + '%' }" />
      </div>
      <div v-if="store.importStatus.message" class="status-message">
        {{ store.importStatus.message }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useDeviceConfigStore } from '@/stores/deviceConfig'

const route = useRoute()
const store = useDeviceConfigStore()

const deviceId = ref<number | string>(route.params.id as string)
const activeTab = ref<'stored' | 'live' | 'diff'>('stored')
const editMode = ref(false)
const editedConfig = ref('')
const showImportDialog = ref(false)
const importConfigText = ref('')

async function handleRefresh() {
  await Promise.all([
    store.fetchStoredConfig(deviceId.value),
    store.fetchLiveConfig(deviceId.value)
  ])
}

async function checkDrift() {
  await store.checkDrift(deviceId.value)
}

async function syncFromLive() {
  if (store.liveConfig) {
    await store.saveStoredConfig(deviceId.value, store.liveConfig)
    await handleRefresh()
  }
}

async function handleSave() {
  try {
    const config = JSON.parse(editedConfig.value)
    await store.saveStoredConfig(deviceId.value, config)
    editMode.value = false
    await handleRefresh()
  } catch (e: any) {
    alert('Invalid JSON: ' + e.message)
  }
}

async function handleExport() {
  try {
    const config = await store.exportConfig(deviceId.value)
    const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `device-${deviceId.value}-config.json`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e: any) {
    alert('Export failed: ' + (e?.message || 'Unknown error'))
  }
}

async function handleImport() {
  try {
    const config = JSON.parse(importConfigText.value)
    await store.importConfig(deviceId.value, config)
    showImportDialog.value = false
    importConfigText.value = ''
    await handleRefresh()
  } catch (e: any) {
    alert('Import failed: ' + (e?.message || 'Invalid JSON'))
  }
}

watch(() => store.storedConfig, (newConfig) => {
  if (newConfig && editMode.value) {
    editedConfig.value = JSON.stringify(newConfig, null, 2)
  }
}, { immediate: true })

watch(editMode, (isEditing) => {
  if (isEditing && store.storedConfig) {
    editedConfig.value = JSON.stringify(store.storedConfig, null, 2)
  }
})

onMounted(() => {
  handleRefresh()
  checkDrift()
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.btn:hover { background: #f8fafc; }
.btn.primary { background: #2563eb; color: white; border-color: #2563eb; }
.btn.primary:hover { background: #1d4ed8; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 16px; }
.state { text-align: center; color: #64748b; padding: 32px; }
.state.error { color: #b91c1c; }

.drift-alert { background: #fef3c7; border-color: #fbbf24; }
.drift-alert h3 { margin: 0 0 8px 0; color: #92400e; }
.drift-alert p { margin: 0 0 8px 0; color: #78350f; }
.drift-fields { margin: 8px 0; font-size: 14px; color: #78350f; }

.config-tabs { display: flex; gap: 4px; border-bottom: 1px solid #e5e7eb; margin-bottom: 16px; }
.tab { padding: 8px 16px; border: none; background: none; cursor: pointer; border-bottom: 2px solid transparent; color: #64748b; }
.tab.active { color: #2563eb; border-bottom-color: #2563eb; font-weight: 600; }
.tab:hover { background: #f8fafc; }

.config-content { position: relative; }
.config-actions { display: flex; gap: 8px; margin-bottom: 12px; }
.config-view { background: #f8fafc; padding: 12px; border-radius: 6px; overflow-x: auto; font-family: ui-monospace, monospace; font-size: 13px; margin: 0; }
.config-textarea { width: 100%; min-height: 400px; font-family: ui-monospace, monospace; font-size: 13px; padding: 12px; border: 1px solid #cbd5e1; border-radius: 6px; resize: vertical; }
.config-editor { margin-top: 12px; }

.diff-view { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.diff-column h4 { margin: 0 0 8px 0; }

.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; padding: 24px; border-radius: 8px; max-width: 800px; width: 90%; max-height: 90vh; overflow-y: auto; }
.modal h2 { margin-top: 0; }
.modal-actions { display: flex; gap: 12px; justify-content: flex-end; margin-top: 16px; }

.import-status { background: #f0f9ff; border-color: #0ea5e9; }
.status-row { display: flex; justify-content: space-between; margin-bottom: 8px; }
.status-badge { padding: 2px 8px; border-radius: 999px; font-size: 12px; font-weight: 600; }
.status-badge.pending { background: #e0e7ff; color: #3730a3; }
.status-badge.in_progress { background: #fef3c7; color: #92400e; }
.status-badge.completed { background: #dcfce7; color: #065f46; }
.status-badge.failed { background: #fee2e2; color: #991b1b; }
.progress-bar { height: 8px; background: #e5e7eb; border-radius: 4px; overflow: hidden; margin: 8px 0; }
.progress-fill { height: 100%; background: #0ea5e9; transition: width 0.3s; }
.status-message { font-size: 14px; color: #64748b; margin-top: 8px; }
</style>
