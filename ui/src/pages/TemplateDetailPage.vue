<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="back-link" to="/templates">← Back to Templates</router-link>
      <h1 class="title">{{ template?.name || 'Loading...' }}</h1>
      <div class="spacer" />
      <button v-if="!editMode" class="btn primary" @click="editMode = true">Edit</button>
      <button v-if="editMode" class="btn" @click="cancelEdit">Cancel</button>
      <button v-if="editMode" class="btn primary" @click="handleSave">Save Changes</button>
    </div>

    <!-- Loading/Error states -->
    <div v-if="store.loading && !template" class="card state">Loading...</div>
    <div v-else-if="store.error" class="card state error">{{ store.error }}</div>

    <!-- Template Details -->
    <div v-else-if="template" class="content">
      <!-- Info Card -->
      <div class="card">
        <div class="card-header">
          <h2>Template Information</h2>
        </div>
        <div class="info-grid">
          <div class="info-item">
            <label>Name</label>
            <input v-if="editMode" v-model="editData.name" type="text" class="form-input" />
            <span v-else>{{ template.name }}</span>
          </div>
          <div class="info-item">
            <label>Device Type</label>
            <select v-if="editMode" v-model="editData.deviceType" class="form-input">
              <option value="shelly1">Shelly 1</option>
              <option value="shelly1pm">Shelly 1PM</option>
              <option value="shelly25">Shelly 2.5</option>
              <option value="shellyplug">Shelly Plug</option>
              <option value="shellyem">Shelly EM</option>
              <option value="shelly3em">Shelly 3EM</option>
            </select>
            <span v-else class="device-type">{{ template.deviceType }}</span>
          </div>
          <div class="info-item full-width">
            <label>Description</label>
            <textarea v-if="editMode" v-model="editData.description" class="form-input" rows="2" />
            <span v-else>{{ template.description || '-' }}</span>
          </div>
          <div class="info-item">
            <label>Created</label>
            <span>{{ formatDate(template.createdAt) }}</span>
          </div>
          <div class="info-item">
            <label>Updated</label>
            <span>{{ formatDate(template.updatedAt) }}</span>
          </div>
        </div>
      </div>

      <!-- Template Content -->
      <div class="card">
        <div class="card-header">
          <h2>Template Content</h2>
          <div class="spacer" />
          <button class="btn small" @click="showValidateDialog = true">Validate</button>
          <button class="btn small primary" @click="showPreviewDialog = true">Preview</button>
        </div>
        <textarea
          v-model="editData.templateContent"
          class="template-editor"
          :readonly="!editMode"
          spellcheck="false"
          rows="20"
        />
      </div>

      <!-- Actions -->
      <div class="card">
        <div class="card-header">
          <h2>Actions</h2>
        </div>
        <div class="actions-grid">
          <button class="action-card" @click="showApplyDialog = true">
            <div class="action-icon">📱</div>
            <div class="action-title">Apply to Device</div>
            <div class="action-desc">Configure a device using this template</div>
          </button>
          <button class="action-card" @click="showPreviewDialog = true">
            <div class="action-icon">👁️</div>
            <div class="action-title">Preview Template</div>
            <div class="action-desc">See rendered configuration</div>
          </button>
          <button class="action-card" @click="showValidateDialog = true">
            <div class="action-icon">✓</div>
            <div class="action-title">Validate Syntax</div>
            <div class="action-desc">Check template for errors</div>
          </button>
        </div>
      </div>
    </div>

    <!-- Preview Dialog -->
    <div v-if="showPreviewDialog" class="modal-overlay" @click.self="showPreviewDialog = false">
      <div class="modal">
        <h2>Preview Template</h2>
        <div class="form-group">
          <label>Variables (JSON)</label>
          <textarea v-model="previewVariables" class="form-input monospace" rows="8" placeholder="{}" />
        </div>
        <button class="btn primary" @click="handlePreview">Generate Preview</button>

        <div v-if="store.previewResult" class="preview-result">
          <h3>Rendered Configuration</h3>
          <pre class="code-block">{{ JSON.stringify(store.previewResult.renderedConfig, null, 2) }}</pre>
          <div v-if="store.previewResult.errors?.length" class="errors">
            <h4>Errors:</h4>
            <ul>
              <li v-for="(err, i) in store.previewResult.errors" :key="i">{{ err }}</li>
            </ul>
          </div>
        </div>

        <div class="modal-actions">
          <button class="btn" @click="closePreview">Close</button>
        </div>
      </div>
    </div>

    <!-- Validate Dialog -->
    <div v-if="showValidateDialog" class="modal-overlay" @click.self="showValidateDialog = false">
      <div class="modal">
        <h2>Validate Template</h2>
        <button class="btn primary" @click="handleValidate">Run Validation</button>

        <div v-if="store.validationResult" class="validation-result">
          <div v-if="store.validationResult.valid" class="success">
            ✓ Template is valid
          </div>
          <div v-else class="errors">
            <h4>Errors:</h4>
            <ul>
              <li v-for="(err, i) in store.validationResult.errors" :key="i">{{ err }}</li>
            </ul>
          </div>
          <div v-if="store.validationResult.warnings?.length" class="warnings">
            <h4>Warnings:</h4>
            <ul>
              <li v-for="(warn, i) in store.validationResult.warnings" :key="i">{{ warn }}</li>
            </ul>
          </div>
        </div>

        <div class="modal-actions">
          <button class="btn" @click="closeValidate">Close</button>
        </div>
      </div>
    </div>

    <!-- Apply to Device Dialog -->
    <div v-if="showApplyDialog" class="modal-overlay" @click.self="showApplyDialog = false">
      <div class="modal">
        <h2>Apply Template to Device</h2>
        <div class="form-group">
          <label>Device ID</label>
          <input v-model.number="applyDeviceId" type="number" class="form-input" placeholder="Enter device ID" />
        </div>
        <div class="form-group">
          <label>Variables (JSON)</label>
          <textarea v-model="applyVariables" class="form-input monospace" rows="8" placeholder="{}" />
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showApplyDialog = false">Cancel</button>
          <button class="btn primary" @click="handleApply">Apply</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTemplatesStore } from '@/stores/templates'
import { getTemplate } from '@/api/templates'
import type { ConfigTemplate } from '@/api/templates'

const route = useRoute()
const router = useRouter()
const store = useTemplatesStore()

const template = ref<ConfigTemplate | null>(null)
const editMode = ref(false)
const editData = ref({
  name: '',
  deviceType: '',
  description: '',
  templateContent: ''
})

const showPreviewDialog = ref(false)
const showValidateDialog = ref(false)
const showApplyDialog = ref(false)
const previewVariables = ref('{}')
const applyVariables = ref('{}')
const applyDeviceId = ref<number | null>(null)

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

async function loadTemplate() {
  const id = route.params.id as string
  try {
    template.value = await getTemplate(id)
    editData.value = {
      name: template.value.name,
      deviceType: template.value.deviceType,
      description: template.value.description || '',
      templateContent: template.value.templateContent
    }
  } catch (e: any) {
    store.error = e?.message || 'Failed to load template'
  }
}

async function handleSave() {
  if (!template.value) return
  try {
    await store.update(template.value.id, editData.value)
    await loadTemplate()
    editMode.value = false
  } catch (e: any) {
    alert('Failed to save: ' + (e?.message || 'Unknown error'))
  }
}

function cancelEdit() {
  if (template.value) {
    editData.value = {
      name: template.value.name,
      deviceType: template.value.deviceType,
      description: template.value.description || '',
      templateContent: template.value.templateContent
    }
  }
  editMode.value = false
}

async function handlePreview() {
  try {
    const vars = JSON.parse(previewVariables.value)
    await store.preview(editData.value.templateContent, vars)
  } catch (e: any) {
    alert('Failed to preview: ' + (e?.message || 'Invalid JSON variables'))
  }
}

async function handleValidate() {
  try {
    await store.validate(editData.value.templateContent, editData.value.deviceType)
  } catch (e: any) {
    alert('Failed to validate: ' + (e?.message || 'Unknown error'))
  }
}

async function handleApply() {
  if (!applyDeviceId.value) {
    alert('Please enter a device ID')
    return
  }
  try {
    const vars = JSON.parse(applyVariables.value)
    // TODO: Integrate with device configuration API when available
    alert('Apply template functionality will be integrated with device configuration API')
    showApplyDialog.value = false
  } catch (e: any) {
    alert('Failed to apply: ' + (e?.message || 'Invalid JSON variables'))
  }
}

function closePreview() {
  showPreviewDialog.value = false
  store.clearPreview()
}

function closeValidate() {
  showValidateDialog.value = false
  store.clearValidation()
}

onMounted(() => {
  loadTemplate()
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.back-link { color: #2563eb; text-decoration: none; }
.back-link:hover { text-decoration: underline; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; font-size: 14px; }
.btn:hover { background: #f8fafc; }
.btn.primary { background: #2563eb; color: white; border-color: #2563eb; }
.btn.primary:hover { background: #1d4ed8; }
.btn.small { padding: 4px 8px; font-size: 12px; }

.content { display: flex; flex-direction: column; gap: 12px; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; }
.card-header { display: flex; align-items: center; gap: 8px; padding: 12px 16px; background: #f8fafc; border-bottom: 1px solid #e5e7eb; }
.card-header h2 { font-size: 16px; margin: 0; }
.state { padding: 32px; text-align: center; color: #64748b; }
.state.error { color: #b91c1c; }

.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; padding: 16px; }
.info-item { display: flex; flex-direction: column; gap: 4px; }
.info-item.full-width { grid-column: 1 / -1; }
.info-item label { font-size: 12px; font-weight: 600; color: #64748b; text-transform: uppercase; }
.info-item span { font-size: 14px; }
.device-type { padding: 2px 8px; background: #e0e7ff; color: #3730a3; border-radius: 999px; font-size: 12px; font-weight: 600; display: inline-block; }

.form-input { padding: 6px 10px; border: 1px solid #d1d5db; border-radius: 6px; font-family: inherit; font-size: 14px; }
.monospace { font-family: ui-monospace, monospace; }

.template-editor { width: 100%; padding: 12px; font-family: ui-monospace, monospace; font-size: 13px; border: none; resize: vertical; }
.template-editor:focus { outline: none; }

.actions-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; padding: 16px; }
.action-card { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 20px; background: #f8fafc; border: 1px solid #e5e7eb; border-radius: 8px; cursor: pointer; text-align: center; }
.action-card:hover { background: #f1f5f9; }
.action-icon { font-size: 32px; }
.action-title { font-weight: 600; font-size: 14px; }
.action-desc { font-size: 12px; color: #64748b; }

.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; padding: 24px; border-radius: 8px; max-width: 800px; width: 90%; max-height: 90vh; overflow-y: auto; }
.modal h2 { margin-top: 0; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-weight: 500; color: #374151; margin-bottom: 6px; }
.modal-actions { display: flex; gap: 12px; justify-content: flex-end; margin-top: 20px; }

.preview-result, .validation-result { margin-top: 16px; padding: 16px; background: #f8fafc; border-radius: 6px; }
.code-block { background: #1e293b; color: #e2e8f0; padding: 12px; border-radius: 4px; overflow-x: auto; font-size: 12px; }
.success { color: #16a34a; font-weight: 600; padding: 12px; background: #dcfce7; border-radius: 6px; }
.errors, .warnings { margin-top: 12px; }
.errors h4, .warnings h4 { margin: 0 0 8px 0; font-size: 14px; }
.errors { color: #b91c1c; }
.warnings { color: #ca8a04; }
.errors ul, .warnings ul { margin: 0; padding-left: 20px; }
</style>
