<template>
  <div class="modal-overlay" data-testid="from-import-dialog" @click.self="emit('close')">
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
              <option v-if="modelListed" value="device_type">Only {{ deviceType }}</option>
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
        <button class="btn" data-testid="from-import-cancel" @click="emit('close')">Cancel</button>
        <button class="btn primary" data-testid="from-import-save" :disabled="!canSave" @click="save">Save</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getTypedDeviceConfig } from '@/api/deviceConfig'
import { getConfigSchema, type ConfigSchema } from '@/api/typedConfig'
import { createTemplate, listDeviceTypes, TemplateSaveError, type FieldError } from '@/api/templates'
import { PER_DEVICE_NOTE, buildTemplateBody, defaultTicked, importSections } from '@/utils/templateFromImport'

const props = defineProps<{
  deviceId: number | string
  // The device's model code (Device.type). A device-type template can name it only when the server lists it.
  deviceType: string
}>()
const emit = defineEmits<{ close: [] }>()

const router = useRouter()

const loading = ref(true)
const loadError = ref('')
const typed = ref<Record<string, any>>({})
const schema = ref<ConfigSchema | null>(null)
const sections = ref<string[]>([])
const ticked = ref<string[]>([])
const modelListed = ref(false)

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
    modelListed.value = !!props.deviceType && types.some((t) => t.device_type === props.deviceType)
    scope.value = modelListed.value ? 'device_type' : 'global'
  } catch (e: any) {
    if (!alive) return
    loadError.value = 'Could not read the imported configuration: ' + (e?.message || 'Unknown error')
  } finally {
    if (alive) loading.value = false
  }
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
      device_type: scope.value === 'device_type' ? props.deviceType : undefined,
      config: body.value.config,
    })
    if (!alive) return
    emit('close')
    await router.push({ name: 'template-detail', params: { id: template.id } })
  } catch (e: any) {
    if (!alive) return
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
