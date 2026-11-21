<template>
  <div class="modal-overlay" data-testid="from-import-dialog" @click.self="close">
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="from-import-title">
      <h2 id="from-import-title">Create template from this import</h2>

      <p v-if="loading" class="hint" role="status" data-testid="from-import-loading">
        Reading the imported configuration...
      </p>
      <p v-else-if="loadError" class="save-error" role="alert" data-testid="from-import-load-error">
        {{ loadError }}
      </p>

      <template v-else>
        <p v-if="sections.length === 0" class="hint" data-testid="from-import-empty">
          Nothing in this import can go into a template for this model.
        </p>
        <fieldset v-else class="sections">
          <legend>Sections to keep</legend>
          <label v-for="s in sections" :key="s" class="check">
            <input v-model="ticked" type="checkbox" :value="s" :data-testid="`section-${s}`" />
            {{ s }}
          </label>
        </fieldset>

        <p class="hint" data-testid="from-import-left-out">
          Left out: {{ PER_DEVICE_NOTE }}<template v-if="body.omittedSecrets.length > 0">;
            secrets ({{ body.omittedSecrets.join(', ') }}). Set a secret on the template page after saving.</template>
        </p>

        <div class="form-grid">
          <div class="form-group">
            <label for="from-import-name">Name</label>
            <input id="from-import-name" v-model="name" type="text" class="form-input" data-testid="from-import-name" />
          </div>
          <div class="form-group">
            <label for="from-import-scope">Scope</label>
            <select id="from-import-scope" v-model="scope" class="form-input" data-testid="from-import-scope">
              <option value="global">All devices</option>
              <option v-if="model" value="device_type">Only {{ model }}</option>
            </select>
          </div>
          <div class="form-group full">
            <label for="from-import-description">Description</label>
            <textarea id="from-import-description" v-model="description" class="form-input" rows="2" />
          </div>
        </div>
      </template>

      <ul v-if="fieldErrors.length > 0" class="save-errors" data-testid="from-import-field-errors">
        <li v-for="err in fieldErrors" :key="err.path"><code>{{ err.path }}</code>: {{ err.message }}</li>
      </ul>
      <p v-if="saveError" class="save-error" role="alert" data-testid="from-import-save-error">{{ saveError }}</p>

      <div class="modal-actions">
        <button class="btn" data-testid="from-import-cancel" :disabled="saving" @click="close">Cancel</button>
        <button class="btn primary" data-testid="from-import-save" :disabled="!canSave" @click="save">Save</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useLateFailure, useLateResult } from '@/composables/useNotice'
import { getTypedDeviceConfig } from '@/api/deviceConfig'
import { getConfigSchema, type ConfigSchema } from '@/api/typedConfig'
import { createTemplate, listDeviceTypes, TemplateSaveError, type FieldError } from '@/api/templates'
import { PER_DEVICE_NOTE, buildTemplateBody, defaultTicked, importSections } from '@/utils/templateFromImport'

const props = defineProps<{ deviceId: number | string }>()
const emit = defineEmits<{ close: [] }>()

const router = useRouter()
const reportLate = useLateFailure()
const reportLateResult = useLateResult()

const loading = ref(true)
const loadError = ref('')
const typed = ref<Record<string, any>>({})
const schema = ref<ConfigSchema | null>(null)
const sections = ref<string[]>([])
const ticked = ref<string[]>([])
// The model a device-type template can name: the one the server built the device's schema for, when
// the server lists it as a supported type. Empty gives only an all-devices template.
const model = ref('')

const name = ref('')
const description = ref('')
const scope = ref<'global' | 'device_type'>('global')
const saving = ref(false)
const saveError = ref('')
const fieldErrors = ref<FieldError[]>([])

// An answer that arrives after the dialog closed belongs to nobody.
let alive = true
onUnmounted(() => {
  alive = false
})

const body = computed(() =>
  schema.value
    ? buildTemplateBody(typed.value, schema.value, ticked.value)
    : { config: {}, omittedSecrets: [], leftOut: [] },
)

const canSave = computed(
  () =>
    !loading.value &&
    !loadError.value &&
    !saving.value &&
    name.value.trim() !== '' &&
    Object.keys(body.value.config).length > 0,
)

async function load() {
  try {
    // The typed read and the schema must both answer. The device types are optional: without them
    // only an all-devices template is offered.
    const [typedConfig, deviceSchema, types] = await Promise.all([
      getTypedDeviceConfig(props.deviceId),
      getConfigSchema(undefined, props.deviceId),
      listDeviceTypes().catch(() => []),
    ])
    if (!alive) return
    typed.value = typedConfig
    schema.value = deviceSchema
    sections.value = importSections(typedConfig, deviceSchema)
    ticked.value = sections.value.filter(defaultTicked)
    const named = deviceSchema['x-device-type'] || ''
    model.value = types.some((t) => t.device_type === named) ? named : ''
    scope.value = model.value ? 'device_type' : 'global'
  } catch (e: any) {
    if (!alive) return
    loadError.value = 'Could not read the imported configuration: ' + (e?.message || 'Unknown error')
  } finally {
    if (alive) loading.value = false
  }
}

// Not while a save is pending: its outcome, an error above all, would reach nobody.
function close() {
  if (!saving.value) emit('close')
}

async function save() {
  if (!canSave.value) return
  saving.value = true
  saveError.value = ''
  fieldErrors.value = []
  try {
    const template = await createTemplate({
      name: name.value.trim(),
      description: description.value.trim() || undefined,
      scope: scope.value,
      device_type: scope.value === 'device_type' ? model.value : undefined,
      config: body.value.config,
    })
    if (!alive) {
      reportLateResult('info', `Template "${template.name}" was created.`, { action: 'Create template', resource: 'Template' })
      return
    }
    emit('close')
    await router.push({ name: 'template-detail', params: { id: template.id } })
  } catch (e: any) {
    if (!alive) {
      // The user left the page while the save ran: the dialog is gone, so the app notice is the only place left.
      // The notice keeps only the message, so the field reasons go into it.
      const reasons = e instanceof TemplateSaveError ? e.fields.map((f) => `${f.path}: ${f.message}`).join('; ') : ''
      reportLate(reasons ? new Error(`${e.message} (${reasons})`) : e, { action: 'Create template', resource: 'Template' })
      return
    }
    if (e instanceof TemplateSaveError) fieldErrors.value = e.fields
    saveError.value = e?.message || 'Failed to create the template'
  } finally {
    if (alive) saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.35); display: flex; align-items: center; justify-content: center; padding: 16px; z-index: 50; }
.modal { width: min(640px, 100%); max-height: calc(100vh - 32px); overflow-y: auto; background: #fff; border-radius: 12px; border: 1px solid #e5e7eb; padding: 14px; }
.sections { border: 1px solid #e5e7eb; border-radius: 8px; padding: 8px 12px; margin: 8px 0; display: flex; flex-wrap: wrap; gap: 4px 16px; }
.check { display: inline-flex; align-items: center; gap: 6px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 8px; }
.form-group { display: flex; flex-direction: column; gap: 6px; }
.form-group.full { grid-column: 1 / -1; }
.hint { color: #475569; font-size: 13px; margin: 6px 0; }
.save-error { color: #991b1b; margin: 8px 0 0; }
.save-errors { margin: 4px 0 0; padding: 8px 12px 8px 28px; background: #fee2e2; color: #991b1b; border-radius: 6px; font-size: 13px; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 12px; border-top: 1px solid #e5e7eb; margin-top: 12px; }
</style>
