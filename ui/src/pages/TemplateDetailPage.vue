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

const showApplyDialog = ref(false)
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


</style>
