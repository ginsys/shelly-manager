<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" :to="`/devices/${deviceId}`">← Back to Device</router-link>
      <h1 class="title">Device Configuration</h1>
      <div class="spacer" />
      <button class="btn" @click="handleRefresh" :disabled="store.loading">Refresh</button>
      <button 
        v-if="store.status?.pending_changes" 
        class="btn primary" 
        @click="handleApply"
        :disabled="store.loading"
      >
        Apply Config
      </button>
    </div>

    <div v-if="store.loading" class="card state">Loading...</div>
    <div v-else-if="store.error" class="card state error">{{ store.error }}</div>

    <template v-else>
      <div v-if="store.status" class="card status-card">
        <div class="status-row">
          <span class="status-label">Status:</span>
          <span v-if="store.status.config_applied && !store.status.pending_changes" class="badge success">✓ Applied</span>
          <span v-else-if="store.status.pending_changes" class="badge warning">⏳ Pending</span>
          <span v-else class="badge">Not configured</span>
        </div>
        <div v-if="store.status.last_applied" class="status-row">
          <span class="status-label">Last Applied:</span>
          <span>{{ formatDate(store.status.last_applied) }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">Templates:</span>
          <span>{{ store.status.template_count }} template(s)</span>
        </div>
        <div class="status-row">
          <span class="status-label">Overrides:</span>
          <span>{{ store.status.has_overrides ? 'Yes' : 'No' }}</span>
        </div>
        <div class="status-actions">
          <button class="btn" @click="handleVerify" :disabled="store.loading">Verify Device</button>
        </div>
      </div>

      <div v-if="store.lastVerify" class="card verify-card">
        <h3>Verification Result</h3>
        <div v-if="store.lastVerify.match" class="verify-success">
          ✓ Device configuration matches desired state
        </div>
        <div v-else class="verify-failed">
          ⚠️ Configuration drift detected
          <div v-if="store.lastVerify.differences" class="differences">
            <div v-for="(diff, idx) in store.lastVerify.differences" :key="idx" class="diff-item">
              <strong>{{ diff.path }}:</strong> expected <code>{{ JSON.stringify(diff.expected) }}</code>, 
              actual <code>{{ JSON.stringify(diff.actual) }}</code>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="header-row">
          <h2>Templates ({{ store.templates.length }})</h2>
          <div class="spacer" />
          <button class="btn" @click="showAddTemplate = true">+ Add Template</button>
        </div>
        <div v-if="store.templates.length === 0" class="empty-state">
          No templates assigned. Add a template to configure this device.
        </div>
        <div v-else class="templates-list">
          <div v-for="(template, idx) in store.templates" :key="template.id" class="template-item">
            <span class="template-order">{{ idx + 1 }}.</span>
            <div class="template-info">
              <div class="template-name">{{ template.name }}</div>
              <div class="template-meta">
                <span class="pill">{{ template.scope }}</span>
                <span v-if="template.device_type" class="pill">{{ template.device_type }}</span>
              </div>
            </div>
            <button class="btn-icon" @click="handleRemoveTemplate(template.id)" title="Remove">×</button>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="header-row">
          <h2>Device Overrides</h2>
          <div class="spacer" />
          <button v-if="store.overrides" class="btn" @click="handleClearOverrides">Clear All</button>
          <button class="btn" @click="editOverrides = !editOverrides">
            {{ editOverrides ? 'Cancel' : 'Edit' }}
          </button>
          <button v-if="editOverrides" class="btn primary" @click="handleSaveOverrides">Save</button>
        </div>
        <div v-if="!store.overrides && !editOverrides" class="empty-state">
          No overrides set. Click Edit to add device-specific overrides.
        </div>
        <div v-else-if="editOverrides" class="editor">
          <textarea 
            v-model="overridesJson" 
            class="code-textarea" 
            rows="12" 
            spellcheck="false"
            placeholder="{}"
          />
        </div>
        <pre v-else class="code-view">{{ JSON.stringify(store.overrides, null, 2) }}</pre>
      </div>

      <div class="card">
        <div class="header-row">
          <h2>Desired Configuration</h2>
          <div class="spacer" />
          <button class="btn" @click="showSources = !showSources">
            {{ showSources ? 'Hide Sources' : 'Show Sources' }}
          </button>
        </div>
        <div v-if="!store.desiredConfig" class="empty-state">
          No configuration computed. Add templates or overrides above.
        </div>
        <div v-else-if="showSources && store.sources" class="sources-view">
          <div v-for="(source, path) in store.sources" :key="path" class="source-item">
            <span class="source-path">{{ path }}:</span>
            <span class="source-badge">{{ getSourceIcon(source) }} {{ source }}</span>
          </div>
        </div>
        <pre v-else class="code-view">{{ JSON.stringify(store.desiredConfig, null, 2) }}</pre>
      </div>
    </template>

    <div v-if="showAddTemplate" class="modal-overlay" @click.self="showAddTemplate = false">
      <div class="modal">
        <h2>Add Template</h2>
        <div class="form-group">
          <label>Template ID *</label>
          <input v-model.number="newTemplateId" type="number" class="form-input" placeholder="Enter template ID" />
        </div>
        <div class="form-group">
          <label>Position (optional)</label>
          <input v-model.number="newTemplatePosition" type="number" class="form-input" placeholder="Leave empty to append" />
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showAddTemplate = false">Cancel</button>
          <button class="btn primary" @click="handleAddTemplate">Add</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useDeviceConfigNewStore } from '@/stores/deviceConfigNew'

const route = useRoute()
const store = useDeviceConfigNewStore()

const deviceId = ref<number | string>(route.params.id as string)
const editOverrides = ref(false)
const overridesJson = ref('{}')
const showSources = ref(false)
const showAddTemplate = ref(false)
const newTemplateId = ref<number | null>(null)
const newTemplatePosition = ref<number | null>(null)

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function getSourceIcon(source: string) {
  if (source.includes('Global')) return '🌍'
  if (source.includes('Group')) return '🏷️'
  if (source.includes('device-type') || source.includes('Device-type')) return '📦'
  if (source.includes('override') || source.includes('Override')) return '✏️'
  return '⚙️'
}

async function handleRefresh() {
  try {
    await store.refresh(deviceId.value)
    overridesJson.value = JSON.stringify(store.overrides || {}, null, 2)
  } catch (e: any) {
    console.error('Refresh failed:', e)
  }
}

async function handleAddTemplate() {
  if (!newTemplateId.value) {
    alert('Template ID is required')
    return
  }

  try {
    await store.addTemplate(
      deviceId.value, 
      newTemplateId.value, 
      newTemplatePosition.value === null ? undefined : newTemplatePosition.value
    )
    await handleRefresh()
    showAddTemplate.value = false
    newTemplateId.value = null
    newTemplatePosition.value = null
  } catch (e: any) {
    alert('Failed to add template: ' + (e?.message || 'Unknown error'))
  }
}

async function handleRemoveTemplate(templateId: number) {
  if (!confirm('Remove this template from the device?')) return

  try {
    await store.removeTemplate(deviceId.value, templateId)
    await handleRefresh()
  } catch (e: any) {
    alert('Failed to remove template: ' + (e?.message || 'Unknown error'))
  }
}

async function handleSaveOverrides() {
  try {
    const parsed = JSON.parse(overridesJson.value || '{}')
    await store.saveOverrides(deviceId.value, parsed)
    editOverrides.value = false
    await handleRefresh()
  } catch (e: any) {
    alert('Failed to save overrides: ' + (e?.message || 'Invalid JSON'))
  }
}

async function handleClearOverrides() {
  if (!confirm('Clear all device overrides?')) return

  try {
    await store.clearOverrides(deviceId.value)
    await handleRefresh()
  } catch (e: any) {
    alert('Failed to clear overrides: ' + (e?.message || 'Unknown error'))
  }
}

async function handleApply() {
  if (!confirm('Apply this configuration to the device? This will modify the physical device.')) return

  try {
    const result = await store.apply(deviceId.value)
    if (result.success) {
      alert(`Applied successfully. ${result.applied_count} field(s) updated.`)
    } else {
      alert(`Apply completed with errors. ${result.failed_count} field(s) failed.`)
    }
    await handleRefresh()
  } catch (e: any) {
    alert('Failed to apply config: ' + (e?.message || 'Unknown error'))
  }
}

async function handleVerify() {
  try {
    await store.verify(deviceId.value)
  } catch (e: any) {
    alert('Failed to verify: ' + (e?.message || 'Unknown error'))
  }
}

onMounted(() => {
  handleRefresh()
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }

.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; font-size: 14px; }
.btn:hover { background: #f8fafc; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn.primary { background: #2563eb; color: white; border-color: #2563eb; }
.btn.primary:hover { background: #1d4ed8; }

.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 16px; }
.state { text-align: center; color: #64748b; padding: 32px; }
.state.error { color: #b91c1c; }

.status-card { background: #f9fafb; }
.status-row { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
.status-label { font-weight: 600; min-width: 120px; }
.status-actions { margin-top: 12px; display: flex; gap: 8px; }

.badge { padding: 4px 12px; border-radius: 999px; font-size: 13px; font-weight: 600; background: #e2e8f0; color: #334155; }
.badge.success { background: #dcfce7; color: #065f46; }
.badge.warning { background: #fef3c7; color: #92400e; }

.verify-card h3 { margin: 0 0 12px 0; }
.verify-success { padding: 12px; background: #dcfce7; color: #065f46; border-radius: 6px; }
.verify-failed { padding: 12px; background: #fee2e2; color: #991b1b; border-radius: 6px; }
.differences { margin-top: 12px; }
.diff-item { padding: 8px; background: #fff; border: 1px solid #fecaca; border-radius: 4px; margin-bottom: 6px; font-size: 13px; }
.diff-item code { background: #fef2f2; padding: 2px 4px; border-radius: 3px; }

.header-row { display: flex; align-items: center; margin-bottom: 16px; }
.header-row h2 { margin: 0; font-size: 16px; }

.empty-state { padding: 24px; text-align: center; color: #64748b; background: #f9fafb; border-radius: 6px; }

.templates-list { display: flex; flex-direction: column; gap: 8px; }
.template-item { display: flex; align-items: center; gap: 12px; padding: 12px; background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; }
.template-order { font-weight: 600; color: #64748b; min-width: 24px; }
.template-info { flex: 1; }
.template-name { font-weight: 600; margin-bottom: 4px; }
.template-meta { display: flex; gap: 6px; }

.pill { padding: 2px 8px; border-radius: 999px; font-size: 12px; font-weight: 600; background: #e0e7ff; color: #3730a3; }

.btn-icon { padding: 4px 8px; border: none; background: #fee2e2; color: #991b1b; border-radius: 4px; cursor: pointer; font-size: 18px; line-height: 1; }
.btn-icon:hover { background: #fecaca; }

.editor { margin-top: 12px; }
.code-textarea { width: 100%; padding: 12px; border: 1px solid #cbd5e1; border-radius: 6px; font-family: ui-monospace, monospace; font-size: 13px; resize: vertical; }
.code-view { background: #f8fafc; padding: 12px; border-radius: 6px; overflow-x: auto; font-family: ui-monospace, monospace; font-size: 13px; margin: 0; }

.sources-view { display: flex; flex-direction: column; gap: 8px; }
.source-item { display: flex; align-items: center; gap: 12px; padding: 8px; background: #f9fafb; border-radius: 4px; }
.source-path { font-weight: 600; font-size: 13px; font-family: ui-monospace, monospace; }
.source-badge { padding: 2px 8px; border-radius: 999px; font-size: 12px; background: #dbeafe; color: #1e40af; }

.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; padding: 24px; border-radius: 8px; max-width: 500px; width: 90%; }
.modal h2 { margin-top: 0; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; margin-bottom: 6px; font-weight: 600; font-size: 14px; }
.form-input { width: 100%; padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 14px; }
.modal-actions { display: flex; gap: 12px; justify-content: flex-end; margin-top: 16px; }
</style>
