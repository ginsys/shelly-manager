<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" to="/templates">← Back</router-link>
      <h1 class="title">{{ template?.name || 'Template' }}</h1>
      <div class="spacer" />
      <button v-if="!editMode" class="btn primary" :disabled="saving || error !== null" @click="enterEdit">Edit</button>
      <button v-else class="btn" @click="cancelEdit">Cancel</button>
      <button v-if="editMode" class="btn primary" data-testid="save-template" :disabled="!configValid || saving" @click="handleSave">
        Save
      </button>
    </div>

    <div v-if="loading" class="card state">Loading...</div>
    <div v-else-if="error" class="card state error">{{ error }}</div>

    <div v-else-if="template" class="content">
      <div class="card">
        <div class="header-row">
          <h2 class="h2">Template</h2>
          <div class="spacer" />
          <button class="btn danger" :disabled="saving" @click="handleDelete">Delete</button>
        </div>

        <div class="info-grid">
          <div class="info-item">
            <label>Name</label>
            <input v-if="editMode" v-model="edit.name" class="form-input" />
            <span v-else>{{ template.name }}</span>
          </div>
          <!-- The update API keeps the scope and the device type, so they are not editable here. -->
          <div class="info-item">
            <label>Scope</label>
            <span class="pill">{{ template.scope }}</span>
          </div>
          <div class="info-item">
            <label>Device Type</label>
            <span class="pill">{{ template.device_type || '-' }}</span>
          </div>
          <div class="info-item full">
            <label>Description</label>
            <textarea v-if="editMode" v-model="edit.description" class="form-input" rows="2" />
            <span v-else>{{ template.description || '-' }}</span>
          </div>
          <div class="info-item">
            <label>Updated</label>
            <span>{{ formatDate(template.updated_at) }}</span>
          </div>
          <div class="info-item">
            <label>Secrets</label>
            <span v-if="hasSecrets(template)" class="pill warning">has secrets</span>
            <span v-else class="pill">none</span>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="header-row">
          <h2 class="h2">{{ editMode ? 'Config' : 'Config (JSON)' }}</h2>
          <div class="spacer" />
          <button class="btn" @click="copyConfig">Copy</button>
        </div>

        <template v-if="editMode">
          <div v-if="schemaTarget === null" class="hint">
            This device_type template has no device type, so no form exists for it.
          </div>
          <ConfigEditor
            v-else
            v-model="configBody"
            v-model:valid="configValid"
            :device-type="schemaTarget?.deviceType"
            :scope="schemaTarget?.scope"
            :inherited="savedSecrets"
            layer
          />
          <ul v-if="saveErrors.length > 0" class="save-errors" data-testid="template-save-errors">
            <li v-for="err in saveErrors" :key="err.path"><code>{{ err.path }}</code>: {{ err.message }}</li>
          </ul>
        </template>
        <textarea v-else :value="configJson" class="code" rows="18" readonly spellcheck="false" />
        <div v-if="lastAffectedDevices !== null" class="hint">
          Update impacts ~{{ lastAffectedDevices }} devices.
        </div>
      </div>

      <div class="card">
        <div class="header-row">
          <h2 class="h2">Assign To Device</h2>
          <div class="spacer" />
          <button class="btn" @click="showAssignDialog = true">Assign</button>
        </div>
        <div class="hint">
          This adds the template to a device’s template chain via the new config system.
        </div>
      </div>
    </div>

    <div v-if="showAssignDialog" class="modal-overlay" @click.self="showAssignDialog = false">
      <div class="modal">
        <h2>Assign Template To Device</h2>

        <div class="form-group">
          <label>Device ID *</label>
          <input v-model="assign.deviceId" type="number" class="form-input" />
        </div>

        <div class="form-group">
          <label>Position (optional)</label>
          <input v-model="assign.position" type="number" class="form-input" placeholder="-1" />
          <div class="hint">Leave empty to append; set 0 to insert first.</div>
        </div>

        <div class="modal-actions">
          <button class="btn" @click="showAssignDialog = false">Cancel</button>
          <button class="btn primary" @click="handleAssign">Assign</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  assignTemplateToDevice,
  deleteTemplate,
  getTemplate,
  updateTemplate,
  TemplateSaveError,
  type ConfigTemplate,
  type FieldError,
  type TemplateScope,
} from '@/api/templates'
import ConfigEditor from '@/components/config/ConfigEditor.vue'
import { clone, sameValue, savedSecretBaseline, templateSchemaTarget, type ConfigBody } from '@/utils/config-form'

const route = useRoute()
const router = useRouter()

const template = ref<ConfigTemplate | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

const editMode = ref(false)
const edit = ref<{ name: string; description?: string; scope: TemplateScope; device_type?: string }>({
  name: '',
  description: '',
  scope: 'global',
  device_type: '',
})

const configJson = ref<string>('{}')
const configBody = ref<ConfigBody>({})
const configValid = ref(true)
const saveErrors = ref<FieldError[]>([])
const schemaTarget = computed(() => templateSchemaTarget(template.value?.scope, template.value?.device_type))
const lastAffectedDevices = ref<number | null>(null)
// The body the template had when Edit opened. It is the same until the form changes it.
const configUntouched = computed(() =>
  template.value !== null && sameValue(configBody.value, template.value.config || {}))
// The saved passwords show as a mask. While the body is untouched the update leaves the config
// out; a changed body that leaves a password out keeps it, because the server keeps a password the
// body omits. A template is a layer: no field is required for being incomplete.
const savedSecrets = computed(() => (template.value ? savedSecretBaseline(template.value) : null))

const showAssignDialog = ref(false)
const assign = ref<{ deviceId: number | null; position: number | null }>({ deviceId: null, position: null })

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function hasSecrets(t: ConfigTemplate) {
  return Boolean(t.has_wifi_password || t.has_mqtt_password || t.has_auth_password)
}

async function load() {
  loading.value = true
  error.value = null

  try {
    const id = route.params.id as string
    template.value = await getTemplate(id)
    configJson.value = JSON.stringify(template.value.config || {}, null, 2)

    edit.value = {
      name: template.value.name,
      description: template.value.description || '',
      scope: (template.value.scope as TemplateScope) || 'global',
      device_type: template.value.device_type || '',
    }
  } catch (e: any) {
    error.value = e?.message || 'Failed to load template'
  } finally {
    loading.value = false
  }
}

// The number of the edit that is open now. It changes when an edit starts or is cancelled. A save
// keeps the number it started with and writes edit state (the error list, leaving edit mode) only
// if the number is still the same, because the answer of a cancelled edit belongs to nobody.
let editSession = 0

// True from the Save click until the save and the refresh after it have settled. While it is set
// there is one save and nothing else that starts or replaces the template: Save, Edit and Delete are
// off, and Cancel is the only action left. A new edit would copy the config the page holds, and the
// refresh then replaces it under that edit.
const saving = ref(false)

function enterEdit() {
  // After a failed load or refresh the page shows an error and not the template, and holds a copy
  // that may be older than the server's, so no edit starts from it.
  if (!template.value || saving.value || error.value !== null) return
  editSession++
  configBody.value = clone(template.value.config || {})
  saveErrors.value = []
  editMode.value = true
  lastAffectedDevices.value = null
}

function cancelEdit() {
  editSession++
  if (!template.value) {
    editMode.value = false
    return
  }

  editMode.value = false
  edit.value = {
    name: template.value.name,
    description: template.value.description || '',
    scope: (template.value.scope as TemplateScope) || 'global',
    device_type: template.value.device_type || '',
  }
  configJson.value = JSON.stringify(template.value.config || {}, null, 2)
  lastAffectedDevices.value = null
}

async function handleSave() {
  if (!template.value || !configValid.value || saving.value) return
  const parsedConfig = clone(configBody.value)
  const session = editSession
  saveErrors.value = []
  saving.value = true

  try {
    // An untouched config is left out of the update: the server keeps the stored one, and the
    // body shown here has no saved passwords in it.
    const result = await updateTemplate(template.value.id, {
      name: edit.value.name,
      description: edit.value.description,
      ...(configUntouched.value ? {} : { config: parsedConfig }),
    })

    if (session !== editSession) {
      // The edit was cancelled while the save ran. No edit can have started since (Edit is off
      // while saving), so the page shows what the server holds now.
      await load()
      return
    }
    lastAffectedDevices.value = result.affected_devices ?? null
    editMode.value = false
    await load()
  } catch (e: any) {
    if (e instanceof TemplateSaveError) {
      if (session === editSession) saveErrors.value = e.fields
      return
    }
    alert('Save failed: ' + (e?.message || 'Unknown error'))
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  if (!template.value || saving.value) return
  if (!confirm('Delete this template?')) return

  try {
    await deleteTemplate(template.value.id)
    await router.push('/templates')
  } catch (e: any) {
    alert('Delete failed: ' + (e?.message || 'Unknown error'))
  }
}

async function copyConfig() {
  try {
    // In edit mode the body being edited is the config on screen, not the saved one.
    const text = editMode.value ? JSON.stringify(configBody.value, null, 2) : configJson.value
    await navigator.clipboard.writeText(text)
  } catch {
    alert('Copy failed')
  }
}

async function handleAssign() {
  if (!template.value) return
  if (!assign.value.deviceId) {
    alert('Device ID is required')
    return
  }

  try {
    await assignTemplateToDevice({
      deviceId: assign.value.deviceId,
      templateId: template.value.id,
      position: assign.value.position === null ? undefined : assign.value.position,
    })

    showAssignDialog.value = false
    assign.value = { deviceId: null, position: null }
    alert('Assigned')
  } catch (e: any) {
    alert('Assign failed: ' + (e?.message || 'Unknown error'))
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }

.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.btn.primary { background: #2563eb; border-color: #2563eb; color: #fff; }
.btn.danger { border-color: #fecaca; color: #b91c1c; }

.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 10px; padding: 12px; }
.state { color: #64748b; }
.state.error { color: #b91c1c; background: #fee2e2; border-color: #fecaca; }

.h2 { margin: 0; font-size: 16px; }
.header-row { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }

.info-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; }
.info-item { display: flex; flex-direction: column; gap: 6px; }
.info-item.full { grid-column: 1 / -1; }
.info-item.disabled { opacity: 0.6; }

label { font-size: 12px; color: #64748b; }
.form-input { padding: 8px 10px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px; }

.pill { display: inline-flex; padding: 2px 8px; border-radius: 999px; background: #f1f5f9; color: #334155; font-size: 12px; width: fit-content; }
.pill.warning { background: #fef3c7; color: #92400e; }

.code { width: 100%; padding: 10px; border: 1px solid #d1d5db; border-radius: 8px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; }

.hint { font-size: 12px; color: #64748b; margin-top: 8px; }
.save-errors { margin: 8px 0 0; padding: 8px 12px 8px 28px; background: #fee2e2; color: #991b1b; border-radius: 6px; font-size: 13px; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }

.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.35); display: flex; align-items: center; justify-content: center; padding: 16px; }
.modal { width: min(560px, 100%); background: #fff; border-radius: 12px; border: 1px solid #e5e7eb; padding: 14px; }
.form-group { display: flex; flex-direction: column; gap: 6px; margin-bottom: 10px; }

.modal-actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 12px; border-top: 1px solid #e5e7eb; margin-top: 12px; }
</style>
