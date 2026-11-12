<template>
  <div class="config-editor">
    <div v-if="loading" class="loading">Loading schema...</div>

    <div v-else class="editor-content">
      <div class="editor-mode">
        <button 
          class="mode-btn" 
          :class="{ active: mode === 'form' }"
          @click="mode = 'form'"
        >
          Form View
        </button>
        <button 
          class="mode-btn" 
          :class="{ active: mode === 'json' }"
          @click="mode = 'json'"
        >
          JSON View
        </button>
      </div>

      <div v-if="mode === 'json'" class="json-editor">
        <textarea 
          v-model="jsonText" 
          class="code-textarea" 
          rows="15" 
          spellcheck="false"
          placeholder="{}"
          @input="handleJsonChange"
        />
        <div v-if="jsonError" class="json-error">{{ jsonError }}</div>
      </div>

      <div v-else class="form-editor">
        <div v-if="!schema" class="no-schema">
          {{ loadError ? `Schema not available (${loadError}).` : 'Schema not available.' }} Changes are not validated here. Use JSON view.
        </div>

        <template v-else>
          <div v-if="schema['x-apply-supported'] === false" class="apply-note">
            Apply does not support this device model yet. The fields are shown read-only.
          </div>
          <div class="sections">
            <div
              v-for="(sectionSchema, sectionKey) in schema.properties"
              :key="sectionKey"
              class="section"
              :data-section="sectionKey"
            >
              <div class="section-header" @click="toggleSection(String(sectionKey))">
                <span class="section-icon">{{ getSectionIcon(String(sectionKey)) }}</span>
                <span class="section-title">{{ sectionSchema.title || formatLabel(String(sectionKey)) }}</span>
                <span v-if="countOverrides(localConfig, [String(sectionKey)]) > 0" class="section-count">
                  {{ countOverrides(localConfig, [String(sectionKey)]) }} override(s)
                </span>
                <span v-if="sectionErrorCount(String(sectionKey)) > 0" class="section-errors">
                  {{ sectionErrorCount(String(sectionKey)) }} error(s)
                </span>
                <span class="expand-icon">{{ expandedSections[sectionKey] ? '−' : '+' }}</span>
              </div>

              <div v-if="expandedSections[sectionKey]" class="section-content">
                <div
                  v-if="schema['x-apply-supported'] !== false && !subtreeApplicable(sectionSchema)"
                  class="apply-note"
                  data-testid="section-not-applied"
                >
                  Not applied to the device: this section is stored here and apply never writes it.
                </div>
                <template v-if="sectionSchema.type === 'object' && sectionSchema.properties">
                  <ConfigSchemaField
                    v-for="(propSchema, propKey) in sectionSchema.properties"
                    :key="propKey"
                    :name="String(propKey)"
                    :schema="propSchema"
                    :path="[String(sectionKey), String(propKey)]"
                    :overrides="localConfig"
                    :inherited="baseline"
                    :sources="sources"
                    :layer="layer"
                    :errors="errors"
                    @set="setField"
                    @reset="resetField"
                  />
                </template>
                <ConfigSchemaField
                  v-else
                  :name="String(sectionKey)"
                  :schema="sectionSchema"
                  :path="[String(sectionKey)]"
                  :overrides="localConfig"
                  :inherited="baseline"
                  :sources="sources"
                  :layer="layer"
                  :errors="errors"
                  @set="setField"
                  @reset="resetField"
                />
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// ConfigEditor edits an override body: only the fields the user changed. Each field shows its
// inherited value (from the templates) as the baseline. Reset removes a field from the
// body; "use default" sets the schema default. Schema validation runs on every change, and the
// editor reports validity through update:valid so the page can block save.
import { ref, watch, computed, onMounted } from 'vue'
import { useTypedConfigStore } from '@/stores/typedConfig'
import type { ConfigSchema } from '@/api/typedConfig'
import ConfigSchemaField from './ConfigSchemaField.vue'
import {
  clone,
  countOverrides,
  deleteAt,
  getAt,
  hasAt,
  sameValue,
  setAt,
  subtreeApplicable,
  validateOverrides,
  type ConfigBody,
  type ConfigPath,
} from '@/utils/config-form'

interface Props {
  modelValue: Record<string, any>
  deviceType?: string
  // A device id asks the server for the schema of that device's model, resolved the way apply
  // resolves it. It wins over deviceType.
  deviceId?: number | string
  // A global or group template scope asks for the template schema: every stored section. It is
  // used only when neither deviceId nor deviceType is given.
  scope?: string
  inherited?: ConfigBody | null
  // True when the body is one layer of the configuration (a template): a field another layer can
  // supply is not required (see validateOverrides).
  layer?: boolean
  sources?: Record<string, string>
  // The overrides as they were saved: the page sets it when it loads them (see `held` below). The device page always gives it; template pages (layer) do not.
  held?: ConfigBody
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: Record<string, any>]
  'update:valid': [value: boolean]
}>()

const typedConfigStore = useTypedConfigStore()

const mode = ref<'form' | 'json'>('form')
const schema = ref<ConfigSchema | null>(null)
// True from the first render: the schema request starts on mount, and nothing is validated before it ends.
const loading = ref(true)
const loadError = ref<string | null>(null)
const jsonText = ref('{}')
const jsonError = ref<string | null>(null)
const expandedSections = ref<Record<string, boolean>>({})
const localConfig = ref<ConfigBody>({})

// The overrides as they were saved. Reset can empty a section, and an emptied section looks
// untouched, yet saving it removes what was saved, so the required check keeps looking at a
// section that was saved with an override (see validateOverrides). The memory is the saved body
// and nothing the editor sees: no edit, typed in the form or in the JSON view, can change it, so
// no sequence of edits can lose it. Only the page knows when the overrides were loaded again
// (another device, a reload after Save, Cancel), so the page gives it (`held` prop). Without the
// prop nothing is remembered: the editor cannot tell a reload from an edit by itself.
const held = computed<ConfigBody>(() => props.held ?? {})

watch(() => props.modelValue, (newValue) => {
  const incoming = newValue || {}
  localConfig.value = clone(incoming)
  jsonText.value = JSON.stringify(incoming, null, 2)
}, { immediate: true, deep: true })

// The page passes the inherited configuration: the template layers alone, which is what a field
// falls back to when its override is removed. A secret that a template sets arrives as a mask.
const baseline = computed<ConfigBody>(() => props.inherited || {})

const errors = computed<Record<string, string>>(() =>
  schema.value?.properties
    ? validateOverrides(schema.value.properties, localConfig.value, baseline.value, held.value,
        props.layer)
    : {},
)

// Valid means checked and clean. While the schema request is pending nothing is checked yet, so
// Save stays off. After a failed request the form says it cannot validate, and the server's
// validation on save is the check left.
const valid = computed(() =>
  jsonError.value === null &&
  !loading.value &&
  Object.keys(errors.value).length === 0,
)

watch(valid, (value) => emit('update:valid', value), { immediate: true })

function sectionErrorCount(section: string): number {
  return Object.keys(errors.value).filter(key => key === section || key.startsWith(`${section}.`)).length
}

// The number of the newest schema request. A template page changes the scope or the model while
// an older request may still be pending, and requests can finish in any order. Only the newest
// one may set the schema, the error and the loading state.
let schemaRequest = 0

async function loadSchema() {
  const request = ++schemaRequest
  loading.value = true
  loadError.value = null
  try {
    const loaded = await typedConfigStore.fetchSchema(props.deviceType, props.deviceId, props.scope)
    if (request !== schemaRequest) return
    schema.value = loaded
    if (loaded?.properties) {
      Object.keys(loaded.properties).forEach(key => {
        // Open the sections that already hold overrides.
        expandedSections.value[key] = countOverrides(localConfig.value, [key]) > 0
      })
    }
  } catch (e: any) {
    if (request !== schemaRequest) return
    loadError.value = e?.message || 'unknown error'
  } finally {
    if (request === schemaRequest) loading.value = false
  }
}

function emitChange() {
  jsonText.value = JSON.stringify(localConfig.value, null, 2)
  // The form body is valid JSON again, so an earlier JSON syntax error no longer applies.
  jsonError.value = null
  emit('update:modelValue', clone(localConfig.value))
}

// A value equal to the inherited one is not an override: it would pin the field and stop it
// following later template changes. Setting it back removes the override instead.
function setField(path: ConfigPath, value: any) {
  if (hasAt(baseline.value, path) && sameValue(getAt(baseline.value, path), value)) {
    localConfig.value = deleteAt(localConfig.value, path)
  } else {
    localConfig.value = setAt(localConfig.value, path, value)
  }
  emitChange()
}

function resetField(path: ConfigPath) {
  localConfig.value = deleteAt(localConfig.value, path)
  emitChange()
}

function handleJsonChange() {
  try {
    localConfig.value = JSON.parse(jsonText.value || '{}')
    jsonError.value = null
    emit('update:modelValue', clone(localConfig.value))
  } catch (e: any) {
    jsonError.value = e.message
  }
}

function getSectionIcon(key: string): string {
  const icons: Record<string, string> = {
    wifi: '📶', mqtt: '📡', auth: '🔐', system: '⚙️', cloud: '☁️',
    location: '📍', relay: '🔌', led: '💡', power_metering: '⚡',
    input: '🔘', coiot: '🔗', dimming: '🌗', roller: '🪟', color: '🎨',
    temp_protection: '🌡️', schedule: '📅', energy_meter: '📊',
    motion: '👁️', sensor: '🌡️'
  }
  return icons[key] || '📋'
}

function formatLabel(key: string): string {
  return key
    .replace(/_/g, ' ')
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, str => str.toUpperCase())
    .trim()
}

function toggleSection(key: string) {
  expandedSections.value[key] = !expandedSections.value[key]
}

onMounted(() => {
  loadSchema()
})

// A template page changes the scope or the model while the editor is open.
watch(() => [props.deviceType, props.deviceId, props.scope], () => {
  schema.value = null
  loadSchema()
})

// The device page reads the schema to find the password fields when it saves.
defineExpose({ schema })
</script>

<style scoped>
.config-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.loading {
  padding: 24px;
  text-align: center;
  color: #64748b;
}

.editor-mode {
  display: flex;
  gap: 4px;
  padding: 4px;
  background: #f1f5f9;
  border-radius: 6px;
  width: fit-content;
}

.mode-btn {
  padding: 6px 12px;
  border: none;
  background: transparent;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
  color: #64748b;
}

.mode-btn:hover {
  background: #e2e8f0;
}

.mode-btn.active {
  background: white;
  color: #1e293b;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}

.json-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.code-textarea {
  width: 100%;
  padding: 12px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  font-family: ui-monospace, monospace;
  font-size: 13px;
  resize: vertical;
}

.json-error {
  padding: 8px 12px;
  background: #fee2e2;
  color: #991b1b;
  border-radius: 4px;
  font-size: 13px;
}

.apply-note {
  padding: 8px 12px;
  margin-bottom: 8px;
  background: #fef3c7;
  color: #92400e;
  border-radius: 4px;
  font-size: 13px;
}

.section-count,
.section-errors {
  font-size: 12px;
  font-weight: 600;
  padding: 1px 8px;
  border-radius: 999px;
}

.section-count {
  background: #dbeafe;
  color: #1e40af;
}

.section-errors {
  background: #fee2e2;
  color: #991b1b;
}

.no-schema {
  padding: 16px;
  text-align: center;
  color: #64748b;
  background: #f9fafb;
  border-radius: 6px;
}

.sections {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  overflow: hidden;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: #f9fafb;
  cursor: pointer;
  user-select: none;
}

.section-header:hover {
  background: #f1f5f9;
}

.section-header input[type="checkbox"] {
  width: 16px;
  height: 16px;
}

.section-icon {
  font-size: 16px;
}

.section-title {
  font-weight: 600;
  flex: 1;
}

.expand-icon {
  color: #64748b;
  font-weight: 600;
  font-size: 18px;
}

.section-content {
  padding: 12px;
  background: white;
  border-top: 1px solid #e5e7eb;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.field label {
  font-size: 13px;
  font-weight: 500;
  color: #374151;
}

.field-hint {
  font-size: 12px;
  color: #64748b;
  margin: 0;
}

.form-input,
.form-select,
.form-textarea {
  padding: 8px 10px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  font-size: 14px;
}

.form-input:focus,
.form-select:focus,
.form-textarea:focus {
  outline: none;
  border-color: #2563eb;
  box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.1);
}

.form-textarea {
  font-family: ui-monospace, monospace;
  font-size: 13px;
  resize: vertical;
}

input[type="checkbox"] {
  width: 18px;
  height: 18px;
}
</style>
