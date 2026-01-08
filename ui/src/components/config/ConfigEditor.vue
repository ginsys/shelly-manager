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
          Schema not available. Use JSON view.
        </div>

        <div v-else class="sections">
          <div 
            v-for="(sectionSchema, sectionKey) in schema.properties" 
            :key="sectionKey"
            class="section"
          >
            <div 
              class="section-header"
              @click="toggleSection(sectionKey as string)"
            >
              <input 
                type="checkbox" 
                :checked="hasSection(sectionKey as string)"
                @click.stop="toggleSectionEnabled(sectionKey as string)"
              />
              <span class="section-icon">{{ getSectionIcon(sectionKey as string) }}</span>
              <span class="section-title">{{ sectionSchema.title || formatLabel(sectionKey as string) }}</span>
              <span class="expand-icon">{{ expandedSections[sectionKey] ? '−' : '+' }}</span>
            </div>

            <div v-if="expandedSections[sectionKey] && hasSection(sectionKey as string)" class="section-content">
              <div 
                v-for="(propSchema, propKey) in sectionSchema.properties" 
                :key="propKey"
                class="field"
              >
                <label :for="`${sectionKey}-${propKey}`">
                  {{ propSchema.title || formatLabel(propKey as string) }}
                </label>
                <p v-if="propSchema.description" class="field-hint">{{ propSchema.description }}</p>

                <input
                  v-if="propSchema.type === 'boolean'"
                  :id="`${sectionKey}-${propKey}`"
                  type="checkbox"
                  :checked="getFieldValue(sectionKey as string, propKey as string)"
                  @change="setFieldValue(sectionKey as string, propKey as string, ($event.target as HTMLInputElement).checked)"
                />

                <input
                  v-else-if="propSchema.type === 'integer' || propSchema.type === 'number'"
                  :id="`${sectionKey}-${propKey}`"
                  type="number"
                  class="form-input"
                  :value="getFieldValue(sectionKey as string, propKey as string)"
                  :min="propSchema.minimum"
                  :max="propSchema.maximum"
                  :placeholder="propSchema.default?.toString()"
                  @input="setFieldValue(sectionKey as string, propKey as string, parseNumber(($event.target as HTMLInputElement).value))"
                />

                <select
                  v-else-if="propSchema.enum"
                  :id="`${sectionKey}-${propKey}`"
                  class="form-select"
                  :value="getFieldValue(sectionKey as string, propKey as string)"
                  @change="setFieldValue(sectionKey as string, propKey as string, ($event.target as HTMLSelectElement).value)"
                >
                  <option value="">Select...</option>
                  <option v-for="opt in propSchema.enum" :key="opt" :value="opt">{{ opt }}</option>
                </select>

                <input
                  v-else-if="propSchema.format === 'password'"
                  :id="`${sectionKey}-${propKey}`"
                  type="password"
                  class="form-input"
                  :value="getFieldValue(sectionKey as string, propKey as string)"
                  :maxlength="propSchema.maxLength"
                  :placeholder="propSchema.default?.toString()"
                  @input="setFieldValue(sectionKey as string, propKey as string, ($event.target as HTMLInputElement).value)"
                />

                <input
                  v-else-if="propSchema.type === 'string'"
                  :id="`${sectionKey}-${propKey}`"
                  type="text"
                  class="form-input"
                  :value="getFieldValue(sectionKey as string, propKey as string)"
                  :maxlength="propSchema.maxLength"
                  :pattern="propSchema.pattern"
                  :placeholder="propSchema.default?.toString()"
                  @input="setFieldValue(sectionKey as string, propKey as string, ($event.target as HTMLInputElement).value)"
                />

                <textarea
                  v-else-if="propSchema.type === 'object' || propSchema.type === 'array'"
                  :id="`${sectionKey}-${propKey}`"
                  class="form-textarea"
                  rows="3"
                  :value="JSON.stringify(getFieldValue(sectionKey as string, propKey as string) || (propSchema.type === 'array' ? [] : {}), null, 2)"
                  @input="setFieldValueJson(sectionKey as string, propKey as string, ($event.target as HTMLTextAreaElement).value)"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useTypedConfigStore } from '@/stores/typedConfig'
import type { ConfigSchema } from '@/api/typedConfig'

interface Props {
  modelValue: Record<string, any>
  deviceType?: string
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: Record<string, any>]
}>()

const typedConfigStore = useTypedConfigStore()

const mode = ref<'form' | 'json'>('form')
const schema = ref<ConfigSchema | null>(null)
const loading = ref(false)
const jsonText = ref('{}')
const jsonError = ref<string | null>(null)
const expandedSections = ref<Record<string, boolean>>({})
const localConfig = ref<Record<string, any>>({})

watch(() => props.modelValue, (newValue) => {
  localConfig.value = JSON.parse(JSON.stringify(newValue || {}))
  jsonText.value = JSON.stringify(newValue || {}, null, 2)
}, { immediate: true, deep: true })

async function loadSchema() {
  loading.value = true
  try {
    schema.value = await typedConfigStore.fetchSchema(props.deviceType)
    if (schema.value?.properties) {
      Object.keys(schema.value.properties).forEach(key => {
        expandedSections.value[key] = false
      })
    }
  } catch (e) {
    console.error('Failed to load schema:', e)
  } finally {
    loading.value = false
  }
}

function emitChange() {
  emit('update:modelValue', JSON.parse(JSON.stringify(localConfig.value)))
}

function handleJsonChange() {
  try {
    localConfig.value = JSON.parse(jsonText.value || '{}')
    jsonError.value = null
    emitChange()
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

function hasSection(sectionKey: string): boolean {
  return sectionKey in localConfig.value
}

function toggleSectionEnabled(sectionKey: string) {
  if (hasSection(sectionKey)) {
    delete localConfig.value[sectionKey]
  } else {
    localConfig.value[sectionKey] = {}
    expandedSections.value[sectionKey] = true
  }
  jsonText.value = JSON.stringify(localConfig.value, null, 2)
  emitChange()
}

function getFieldValue(section: string, field: string): any {
  return localConfig.value[section]?.[field]
}

function setFieldValue(section: string, field: string, value: any) {
  if (!localConfig.value[section]) {
    localConfig.value[section] = {}
  }
  if (value === '' || value === undefined) {
    delete localConfig.value[section][field]
  } else {
    localConfig.value[section][field] = value
  }
  jsonText.value = JSON.stringify(localConfig.value, null, 2)
  emitChange()
}

function setFieldValueJson(section: string, field: string, jsonValue: string) {
  try {
    const parsed = JSON.parse(jsonValue)
    setFieldValue(section, field, parsed)
  } catch {
    // Invalid JSON, ignore
  }
}

function parseNumber(value: string): number | undefined {
  if (value === '') return undefined
  const num = Number(value)
  return isNaN(num) ? undefined : num
}

onMounted(() => {
  loadSchema()
})
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
