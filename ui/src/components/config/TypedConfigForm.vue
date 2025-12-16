<template>
  <div class="typed-config-form">
    <div v-if="store.loading" class="loading">Loading schema...</div>
    <div v-else-if="store.error" class="error">{{ store.error }}</div>

    <form v-else @submit.prevent="handleSubmit">
      <div v-if="schema" class="schema-form">
        <div
          v-for="(property, key) in schema.properties"
          :key="key"
          class="form-field"
          :class="{ required: schema.required?.includes(key as string) }"
        >
          <label :for="`field-${key}`">
            {{ formatLabel(key as string) }}
            <span v-if="schema.required?.includes(key as string)" class="required-mark">*</span>
          </label>
          <p v-if="property.description" class="field-description">{{ property.description }}</p>

          <!-- String input -->
          <input
            v-if="property.type === 'string' && !property.enum"
            :id="`field-${key}`"
            v-model="formData[key as string]"
            type="text"
            class="form-input"
            :required="schema.required?.includes(key as string)"
            :pattern="property.pattern"
          />

          <!-- Number input -->
          <input
            v-else-if="property.type === 'number' || property.type === 'integer'"
            :id="`field-${key}`"
            v-model.number="formData[key as string]"
            type="number"
            class="form-input"
            :required="schema.required?.includes(key as string)"
            :min="property.minimum"
            :max="property.maximum"
          />

          <!-- Boolean checkbox -->
          <input
            v-else-if="property.type === 'boolean'"
            :id="`field-${key}`"
            v-model="formData[key as string]"
            type="checkbox"
            class="form-checkbox"
          />

          <!-- Enum select -->
          <select
            v-else-if="property.enum"
            :id="`field-${key}`"
            v-model="formData[key as string]"
            class="form-select"
            :required="schema.required?.includes(key as string)"
          >
            <option value="">Select...</option>
            <option v-for="option in property.enum" :key="option" :value="option">
              {{ option }}
            </option>
          </select>

          <!-- Object (nested) - show as JSON textarea -->
          <textarea
            v-else-if="property.type === 'object'"
            :id="`field-${key}`"
            v-model="formData[key as string]"
            class="form-textarea"
            :required="schema.required?.includes(key as string)"
            rows="4"
            placeholder="{}"
          />

          <!-- Array - show as JSON textarea -->
          <textarea
            v-else-if="property.type === 'array'"
            :id="`field-${key}`"
            v-model="formData[key as string]"
            class="form-textarea"
            :required="schema.required?.includes(key as string)"
            rows="3"
            placeholder="[]"
          />
        </div>
      </div>

      <div v-else class="no-schema">
        <p>No schema available. Using raw JSON editor:</p>
        <textarea
          v-model="rawJson"
          class="form-textarea raw-json"
          rows="15"
          placeholder="{}"
          spellcheck="false"
        />
      </div>

      <div class="form-actions">
        <button type="button" class="btn" @click="handleValidate">Validate</button>
        <button type="button" class="btn" @click="handleConvert">Convert to Raw</button>
        <button type="submit" class="btn primary">{{ submitLabel }}</button>
      </div>
    </form>

    <!-- Validation Results -->
    <ValidationResults v-if="store.validationResult" :result="store.validationResult" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useTypedConfigStore } from '@/stores/typedConfig'
import ValidationResults from './ValidationResults.vue'
import type { ConfigSchema } from '@/api/typedConfig'

interface Props {
  deviceType: string
  initialData?: Record<string, any>
  submitLabel?: string
}

const props = withDefaults(defineProps<Props>(), {
  submitLabel: 'Save Configuration'
})

const emit = defineEmits<{
  submit: [config: Record<string, any>]
  convert: [rawConfig: Record<string, any>]
}>()

const store = useTypedConfigStore()
const schema = ref<ConfigSchema | null>(null)
const formData = ref<Record<string, any>>({})
const rawJson = ref('')

function formatLabel(key: string): string {
  return key
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, str => str.toUpperCase())
    .trim()
}

function initializeFormData() {
  if (props.initialData) {
    formData.value = { ...props.initialData }
    rawJson.value = JSON.stringify(props.initialData, null, 2)
  } else if (schema.value) {
    // Initialize with default values from schema
    const defaults: Record<string, any> = {}
    Object.entries(schema.value.properties).forEach(([key, prop]) => {
      if (prop.default !== undefined) {
        defaults[key] = prop.default
      }
    })
    formData.value = defaults
  }
}

async function loadSchema() {
  try {
    schema.value = await store.fetchSchema(props.deviceType)
    initializeFormData()
  } catch (e) {
    console.error('Failed to load schema:', e)
  }
}

async function handleValidate() {
  try {
    const config = schema.value ? formData.value : JSON.parse(rawJson.value)
    await store.validate(config, props.deviceType)
  } catch (e: any) {
    alert('Validation failed: ' + (e?.message || 'Invalid JSON'))
  }
}

async function handleConvert() {
  try {
    const config = schema.value ? formData.value : JSON.parse(rawJson.value)
    const rawConfig = await store.toRaw(config, props.deviceType)
    emit('convert', rawConfig)
  } catch (e: any) {
    alert('Conversion failed: ' + (e?.message || 'Unknown error'))
  }
}

function handleSubmit() {
  try {
    const config = schema.value ? formData.value : JSON.parse(rawJson.value)
    emit('submit', config)
  } catch (e: any) {
    alert('Invalid configuration: ' + (e?.message || 'Invalid JSON'))
  }
}

watch(() => props.initialData, () => {
  initializeFormData()
}, { deep: true })

onMounted(() => {
  loadSchema()
})
</script>

<style scoped>
.typed-config-form { display: flex; flex-direction: column; gap: 16px; }
.loading, .error, .no-schema { padding: 16px; text-align: center; color: #64748b; }
.error { color: #b91c1c; background: #fee2e2; border-radius: 6px; }
.no-schema { background: #f8fafc; border: 1px solid #e5e7eb; border-radius: 6px; }
.no-schema p { margin: 0 0 12px 0; font-weight: 500; }

.schema-form { display: flex; flex-direction: column; gap: 16px; }
.form-field { display: flex; flex-direction: column; gap: 6px; }
.form-field.required label { font-weight: 600; }
.form-field label { font-size: 14px; color: #374151; }
.required-mark { color: #dc2626; }
.field-description { margin: 0; font-size: 12px; color: #64748b; }

.form-input, .form-select, .form-textarea {
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-family: inherit;
  font-size: 14px;
}
.form-textarea { resize: vertical; }
.form-textarea.raw-json { font-family: ui-monospace, monospace; font-size: 13px; }
.form-checkbox { width: 20px; height: 20px; }

.form-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  padding-top: 8px;
  border-top: 1px solid #e5e7eb;
}
.btn {
  padding: 8px 16px;
  border: 1px solid #cbd5e1;
  background: #fff;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
}
.btn:hover { background: #f8fafc; }
.btn.primary { background: #2563eb; color: white; border-color: #2563eb; }
.btn.primary:hover { background: #1d4ed8; }
</style>
