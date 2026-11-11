<template>
  <div class="page">
    <div class="toolbar">
      <h1 class="title">Configuration Templates</h1>
      <div class="spacer" />
      <button class="btn primary" :disabled="busy" @click="openCreate">+ Create Template</button>
    </div>

    <div class="filters">
      <select v-model="store.scopeFilter" class="select" :disabled="busy" @change="changeScope">
        <option value="">All Scopes</option>
        <option value="global">Global</option>
        <option value="group">Group</option>
        <option value="device_type">Device Type</option>
      </select>

      <input
        v-model="store.deviceTypeFilter"
        type="text"
        class="search"
        placeholder="Filter device type (e.g. SHPLG-S)"
      />

      <input v-model="store.searchFilter" type="text" class="search" placeholder="Search by name..." />
    </div>

    <div v-if="store.loading" class="card state">Loading...</div>
    <div v-else-if="store.error" class="card state error">{{ store.error }}</div>

    <div v-else class="card">
      <table v-if="store.filteredTemplates.length > 0" class="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Scope</th>
            <th>Device Type</th>
            <th>Updated</th>
            <th>Secrets</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="template in store.filteredTemplates" :key="template.id">
            <td>
              <router-link :to="`/templates/${template.id}`" class="link">{{ template.name }}</router-link>
            </td>
            <td><span class="pill">{{ template.scope }}</span></td>
            <td><span class="pill">{{ template.device_type || '-' }}</span></td>
            <td>{{ formatDate(template.updated_at) }}</td>
            <td>
              <span v-if="hasSecrets(template)" class="pill warning">has secrets</span>
              <span v-else class="pill">none</span>
            </td>
            <td>
              <div class="actions">
                <button class="action-btn" :disabled="busy" @click="openEdit(template)">Edit</button>
                <button class="action-btn danger" :disabled="busy" @click="handleDelete(template.id)">Delete</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="state">No templates found</div>
    </div>

    <div v-if="showDialog" class="modal-overlay" @click.self="closeDialog">
      <div class="modal">
        <h2>{{ editing ? 'Edit Template' : 'Create Template' }}</h2>

        <div class="form-grid">
          <div class="form-group">
            <label>Name *</label>
            <input v-model="form.name" type="text" class="form-input" required />
          </div>

          <div class="form-group">
            <label>Scope *</label>
            <select v-model="form.scope" class="form-input" data-testid="template-scope" :disabled="!!editing">
              <option value="global">global</option>
              <option value="group">group</option>
              <option value="device_type">device_type</option>
            </select>
          </div>

          <div class="form-group" :class="{ disabled: form.scope !== 'device_type' }">
            <label>Device Type {{ form.scope === 'device_type' ? '*' : '' }}</label>
            <!-- The type of a saved template is read-only, and may be one the list no longer has. -->
            <input
              v-if="editing"
              :value="form.device_type"
              type="text"
              class="form-input"
              data-testid="template-device-type"
              disabled
            />
            <select
              v-else
              v-model="form.device_type"
              class="form-input"
              data-testid="template-device-type"
              :disabled="form.scope !== 'device_type' || deviceTypes.length === 0"
              @change="schemaDeviceType = form.device_type || ''"
            >
              <option value="" disabled>Select a device type</option>
              <option v-for="t in deviceTypes" :key="t.device_type" :value="t.device_type">
                {{ t.name }} ({{ t.device_type }})
              </option>
            </select>
            <div v-if="!editing && form.scope === 'device_type' && deviceTypesError" class="hint" data-testid="device-types-error">
              {{ deviceTypesError }}
              <button class="btn" data-testid="retry-device-types" @click="loadDeviceTypes">Retry</button>
            </div>
          </div>

          <div class="form-group full">
            <label>Description</label>
            <textarea v-model="form.description" class="form-input" rows="2" />
          </div>

          <div class="form-group full">
            <label>Config</label>
            <div v-if="schemaTarget === null" class="hint" data-testid="template-needs-type">
              Pick the device type to get the form for that model.
            </div>
            <ConfigEditor
              v-else
              v-model="configBody"
              v-model:valid="configValid"
              :device-type="schemaTarget.deviceType"
              :scope="schemaTarget.scope"
              :inherited="savedSecrets"
              layer
            />
            <div class="hint">
              Set only the fields this template must set. Password fields are accepted, but returned redacted.
            </div>
            <ul v-if="saveErrors.length > 0" class="save-errors" data-testid="template-save-errors">
              <li v-for="err in saveErrors" :key="err.path"><code>{{ err.path }}</code>: {{ err.message }}</li>
            </ul>
          </div>
        </div>

        <div class="modal-actions">
          <button class="btn" @click="closeDialog">Cancel</button>
          <button
            class="btn primary"
            data-testid="save-template"
            :disabled="!configValid || schemaTarget === null || typeMissing || busy"
            @click="handleSave"
          >
            Save
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useTemplatesStore } from '@/stores/templates'
import {
  listDeviceTypes,
  TemplateSaveError,
  type ConfigTemplate,
  type DeviceTypeInfo,
  type FieldError,
  type TemplateScope,
} from '@/api/templates'
import ConfigEditor from '@/components/config/ConfigEditor.vue'
import { clone, sameValue, savedSecretBaseline, templateSchemaTarget, type ConfigBody } from '@/utils/config-form'

const store = useTemplatesStore()

const showDialog = ref(false)
const editing = ref<ConfigTemplate | null>(null)

const form = ref<{ name: string; description?: string; scope: TemplateScope; device_type?: string }>({
  name: '',
  description: '',
  scope: 'global',
  device_type: '',
})

const configBody = ref<ConfigBody>({})
const configValid = ref(true)
const saveErrors = ref<FieldError[]>([])
// The device type the form schema follows. It changes when the input is committed (change
// event), not on each key, so a half-typed model does not load a schema.
const schemaDeviceType = ref('')
const schemaTarget = computed(() => templateSchemaTarget(form.value.scope, schemaDeviceType.value))
// The body the template had when the dialog opened. It is the same until the form changes it.
const configUntouched = computed(() =>
  editing.value !== null && sameValue(configBody.value, editing.value.config || {}))
// The saved passwords show as a mask. While the body is untouched the update leaves the config
// out; a changed body that leaves a password out keeps it, because the server keeps a password the
// body omits. A template is a layer: no field is required for being incomplete.
const savedSecrets = computed(() => (editing.value ? savedSecretBaseline(editing.value) : null))

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

// The number of the dialog that is open now. It changes when a dialog opens or closes. A save
// keeps the number it started with and writes dialog state (the error list, closing) only if the
// number is still the same, because the answer of a closed dialog belongs to nobody.
let dialogSession = 0

// True from the Save click until the save and the list refresh after it have settled.
const saving = ref(false)

// Busy while a save runs or any request of the store is in flight. Nothing that opens, sends or
// reloads a template starts then: a list request that ends must not bring back an action while
// another request is still pending. Cancel is the one way out and stays.
const busy = computed(() => saving.value || store.loading)

function changeScope() {
  if (busy.value) return
  store.fetchTemplates()
}

// The models the create dialog offers. A failed load leaves the list empty, so no type can be
// chosen and a device-type template cannot be saved; there is no free-text fallback.
const deviceTypes = ref<DeviceTypeInfo[]>([])
const deviceTypesError = ref('')

// The number of the newest list request. A response whose request is no longer the newest (the
// dialog was closed or reopened, or Retry started another) belongs to nobody and is dropped.
let typesRequest = 0

// A device-type template needs a type from the loaded list. The list is cleared when a load starts
// and when it fails, so a type chosen from an earlier list never outlives it.
const typeMissing = computed(() =>
  !editing.value && form.value.scope === 'device_type' &&
  !deviceTypes.value.some((t) => t.device_type === form.value.device_type))

async function loadDeviceTypes() {
  const mine = ++typesRequest
  deviceTypesError.value = ''
  deviceTypes.value = []
  form.value.device_type = ''
  schemaDeviceType.value = ''
  try {
    const list = await listDeviceTypes()
    if (mine === typesRequest) deviceTypes.value = list
  } catch (e: any) {
    if (mine === typesRequest) {
      deviceTypesError.value = 'Could not load the device types: ' + (e?.message || 'Unknown error')
    }
  }
}

function openCreate() {
  if (busy.value) return
  dialogSession++
  editing.value = null
  form.value = { name: '', description: '', scope: 'global', device_type: '' }
  // A new template starts empty; the form offers each schema default through "use default".
  configBody.value = {}
  schemaDeviceType.value = ''
  saveErrors.value = []
  showDialog.value = true
  void loadDeviceTypes()
}

function openEdit(template: ConfigTemplate) {
  if (busy.value) return
  dialogSession++
  editing.value = template
  form.value = {
    name: template.name,
    description: template.description || '',
    scope: (template.scope as TemplateScope) || 'global',
    device_type: template.device_type || '',
  }
  configBody.value = clone(template.config || {})
  schemaDeviceType.value = template.device_type || ''
  saveErrors.value = []
  showDialog.value = true
}

function closeDialog() {
  dialogSession++
  typesRequest++
  showDialog.value = false
  editing.value = null
}

async function handleSave() {
  if (!configValid.value || schemaTarget.value === null || typeMissing.value || busy.value) return
  const parsedConfig = clone(configBody.value)
  const session = dialogSession
  saveErrors.value = []
  saving.value = true

  try {
    if (editing.value) {
      // An untouched config is left out of the update: the server keeps the stored one, and the
      // body shown here has no saved passwords in it.
      await store.update(editing.value.id, {
        name: form.value.name,
        description: form.value.description,
        ...(configUntouched.value ? {} : { config: parsedConfig }),
      })
    } else {
      await store.create({
        name: form.value.name,
        description: form.value.description,
        scope: form.value.scope,
        device_type: form.value.scope === 'device_type' ? form.value.device_type : undefined,
        config: parsedConfig,
      })
    }

    if (session === dialogSession) closeDialog()
    await store.fetchTemplates()
  } catch (e: any) {
    if (e instanceof TemplateSaveError) {
      if (session === dialogSession) saveErrors.value = e.fields
      return
    }
    alert('Failed to save template: ' + (e?.message || 'Unknown error'))
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: number) {
  if (busy.value) return
  if (!confirm('Delete this template? This will fail if assigned to devices.')) return

  try {
    await store.remove(id)
  } catch (e: any) {
    alert('Failed to delete: ' + (e?.message || 'Unknown error'))
  }
}

onMounted(() => {
  store.fetchTemplates()
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }

.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; }
.btn.primary { background: #2563eb; border-color: #2563eb; color: #fff; }

.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 10px; padding: 12px; }
.state { color: #64748b; }
.state.error { color: #b91c1c; background: #fee2e2; border-color: #fecaca; }

.filters { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.select, .search { padding: 8px 10px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px; }
.search { min-width: 240px; }

.table { width: 100%; border-collapse: collapse; }
.table th, .table td { padding: 10px; border-bottom: 1px solid #e5e7eb; text-align: left; }
.link { color: #2563eb; text-decoration: none; }
.link:hover { text-decoration: underline; }

.pill { display: inline-flex; padding: 2px 8px; border-radius: 999px; background: #f1f5f9; color: #334155; font-size: 12px; }
.pill.warning { background: #fef3c7; color: #92400e; }

.actions { display: flex; gap: 8px; }
.action-btn { padding: 6px 10px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; }
.action-btn.danger { border-color: #fecaca; color: #b91c1c; background: #fff; }

.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.35); display: flex; align-items: center; justify-content: center; padding: 16px; }
.modal { width: min(840px, 100%); background: #fff; border-radius: 12px; border: 1px solid #e5e7eb; padding: 14px; }

.form-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; }
.form-group { display: flex; flex-direction: column; gap: 6px; }
.form-group.full { grid-column: 1 / -1; }
.form-group.disabled { opacity: 0.6; }
.form-input { padding: 8px 10px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px; }
.save-errors { margin: 4px 0 0; padding: 8px 12px 8px 28px; background: #fee2e2; color: #991b1b; border-radius: 6px; font-size: 13px; }
.modal { max-height: calc(100vh - 32px); overflow-y: auto; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.hint { font-size: 12px; color: #64748b; }

.modal-actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 12px; border-top: 1px solid #e5e7eb; margin-top: 12px; }
</style>
