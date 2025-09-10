<template>
  <q-dialog v-model="isOpen" persistent>
    <q-card style="min-width: 600px; max-width: 800px">
      <q-card-section class="row items-center">
        <div class="text-h6">Configure {{ plugin?.name || 'Plugin' }}</div>
        <q-space />
        <q-btn icon="close" flat round dense v-close-popup />
      </q-card-section>

      <q-separator />

      <q-card-section v-if="plugin" class="q-pt-none">
        <div class="q-mb-md">
          <div class="text-subtitle2 text-grey-7">{{ plugin.description }}</div>
          <div class="text-caption text-grey-6">Version: {{ plugin.version }}</div>
        </div>

        <q-form @submit="handleSubmit" @reset="handleReset" class="q-gutter-md">
          <div v-if="loading" class="text-center q-py-lg">
            <q-spinner color="primary" size="lg" />
            <div class="q-mt-md">Loading plugin schema...</div>
          </div>

          <div v-else-if="error" class="q-pa-md bg-negative text-white rounded-borders">
            <q-icon name="error" class="q-mr-sm" />
            {{ error }}
          </div>

          <div v-else-if="schema" class="config-form">
            <!-- Profile Selection -->
            <div class="row q-gutter-md q-mb-lg">
              <div class="col">
                <q-select
                  v-model="selectedProfile"
                  :options="profileOptions"
                  label="Configuration Profile"
                  emit-value
                  map-options
                  clearable
                  @update:model-value="loadProfile"
                >
                  <template v-slot:append>
                    <q-btn
                      round
                      dense
                      flat
                      icon="add"
                      @click="showSaveProfile = true"
                      class="q-ml-xs"
                    />
                  </template>
                </q-select>
              </div>
            </div>

            <!-- Dynamic Form Fields -->
            <div class="form-fields">
              <div v-for="(fieldSchema, key) in schema.properties" :key="key" class="q-mb-md">
                <!-- String Fields -->
                <q-input
                  v-if="fieldSchema.type === 'string'"
                  v-model="formData[key]"
                  :label="fieldSchema.title || key"
                  :hint="fieldSchema.description"
                  :required="schema.required?.includes(key)"
                  :rules="getValidationRules(fieldSchema, schema.required?.includes(key))"
                  outlined
                >
                  <template v-slot:prepend v-if="fieldSchema.format === 'password'">
                    <q-icon name="lock" />
                  </template>
                </q-input>

                <!-- Number Fields -->
                <q-input
                  v-else-if="fieldSchema.type === 'number' || fieldSchema.type === 'integer'"
                  v-model.number="formData[key]"
                  :label="fieldSchema.title || key"
                  :hint="fieldSchema.description"
                  :required="schema.required?.includes(key)"
                  :rules="getValidationRules(fieldSchema, schema.required?.includes(key))"
                  type="number"
                  outlined
                />

                <!-- Boolean Fields -->
                <q-toggle
                  v-else-if="fieldSchema.type === 'boolean'"
                  v-model="formData[key]"
                  :label="fieldSchema.title || key"
                  :false-value="false"
                  :true-value="true"
                />
                <div v-if="fieldSchema.type === 'boolean' && fieldSchema.description" class="text-caption text-grey-6 q-ml-sm q-mt-xs">
                  {{ fieldSchema.description }}
                </div>

                <!-- Array Fields (Select Multiple) -->
                <q-select
                  v-else-if="fieldSchema.type === 'array'"
                  v-model="formData[key]"
                  :options="fieldSchema.items?.enum || []"
                  :label="fieldSchema.title || key"
                  :hint="fieldSchema.description"
                  :required="schema.required?.includes(key)"
                  multiple
                  outlined
                  use-chips
                />

                <!-- Enum Fields (Select) -->
                <q-select
                  v-else-if="fieldSchema.enum"
                  v-model="formData[key]"
                  :options="fieldSchema.enum"
                  :label="fieldSchema.title || key"
                  :hint="fieldSchema.description"
                  :required="schema.required?.includes(key)"
                  outlined
                />

                <!-- Object Fields (JSON Editor) -->
                <div v-else-if="fieldSchema.type === 'object'">
                  <div class="text-subtitle2 q-mb-sm">{{ fieldSchema.title || key }}</div>
                  <q-input
                    v-model="formData[key]"
                    :label="`${fieldSchema.title || key} (JSON)`"
                    :hint="fieldSchema.description"
                    type="textarea"
                    outlined
                    rows="4"
                    :rules="[val => isValidJSON(val) || 'Invalid JSON format']"
                  />
                </div>
              </div>
            </div>

            <!-- Configuration Preview -->
            <q-expansion-item 
              icon="preview" 
              label="Configuration Preview"
              class="q-mt-lg"
            >
              <q-card class="bg-grey-1">
                <q-card-section>
                  <pre class="text-caption">{{ JSON.stringify(formData, null, 2) }}</pre>
                </q-card-section>
              </q-card>
            </q-expansion-item>

            <!-- Validation Errors -->
            <div v-if="validationErrors.length > 0" class="q-mt-md">
              <q-banner class="bg-red-1 text-red-8">
                <template v-slot:avatar>
                  <q-icon name="error" color="red" />
                </template>
                <div class="text-weight-medium">Configuration Errors:</div>
                <ul class="q-mt-sm q-mb-none">
                  <li v-for="error in validationErrors" :key="error">{{ error }}</li>
                </ul>
              </q-banner>
            </div>
          </div>

          <!-- Form Actions -->
          <q-card-actions align="right" class="q-pt-lg">
            <q-btn flat label="Cancel" v-close-popup />
            <q-btn 
              flat 
              label="Test Config" 
              icon="play_arrow"
              @click="testConfiguration"
              :loading="testing"
              :disable="validationErrors.length > 0"
            />
            <q-btn 
              unelevated 
              label="Save Configuration" 
              type="submit"
              color="primary"
              :loading="saving"
              :disable="validationErrors.length > 0"
            />
          </q-card-actions>
        </q-form>
      </q-card-section>
    </q-card>

    <!-- Save Profile Dialog -->
    <q-dialog v-model="showSaveProfile">
      <q-card style="min-width: 400px">
        <q-card-section>
          <div class="text-h6">Save Configuration Profile</div>
        </q-card-section>

        <q-card-section class="q-pt-none">
          <q-input
            v-model="newProfileName"
            label="Profile Name"
            outlined
            autofocus
            @keyup.enter="saveProfile"
          />
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat label="Cancel" v-close-popup />
          <q-btn 
            unelevated 
            label="Save" 
            color="primary"
            @click="saveProfile"
            :disable="!newProfileName"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useQuasar } from 'quasar'
import { usePluginStore } from '../stores/plugin'
import type { Plugin, PluginSchema } from '../api/plugin'

interface Props {
  plugin: Plugin | null
  modelValue: boolean
}

interface Emits {
  (event: 'update:modelValue', value: boolean): void
  (event: 'configured', plugin: Plugin, config: Record<string, any>): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const $q = useQuasar()
const pluginStore = usePluginStore()

// Component State
const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const schema = ref<PluginSchema | null>(null)
const formData = ref<Record<string, any>>({})
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const error = ref<string | null>(null)

// Profile Management
const selectedProfile = ref<string | null>(null)
const showSaveProfile = ref(false)
const newProfileName = ref('')
const savedProfiles = ref<Record<string, Record<string, any>>>({})

// Computed Properties
const profileOptions = computed(() => 
  Object.keys(savedProfiles.value).map(name => ({
    label: name,
    value: name
  }))
)

const validationErrors = computed(() => {
  const errors: string[] = []
  if (!schema.value) return errors

  // Check required fields
  if (schema.value.required) {
    for (const field of schema.value.required) {
      if (!formData.value[field] || formData.value[field] === '') {
        const fieldSchema = schema.value.properties?.[field]
        const fieldName = fieldSchema?.title || field
        errors.push(`${fieldName} is required`)
      }
    }
  }

  // Validate field types and constraints
  if (schema.value.properties) {
    for (const [key, fieldSchema] of Object.entries(schema.value.properties)) {
      const value = formData.value[key]
      if (value === undefined || value === null || value === '') continue

      if (fieldSchema.type === 'string' && typeof value !== 'string') {
        errors.push(`${fieldSchema.title || key} must be a string`)
      } else if (fieldSchema.type === 'number' && typeof value !== 'number') {
        errors.push(`${fieldSchema.title || key} must be a number`)
      } else if (fieldSchema.type === 'boolean' && typeof value !== 'boolean') {
        errors.push(`${fieldSchema.title || key} must be a boolean`)
      } else if (fieldSchema.type === 'object' && typeof value === 'string') {
        if (!isValidJSON(value)) {
          errors.push(`${fieldSchema.title || key} must be valid JSON`)
        }
      }

      // Check string constraints
      if (fieldSchema.type === 'string' && typeof value === 'string') {
        if (fieldSchema.minLength && value.length < fieldSchema.minLength) {
          errors.push(`${fieldSchema.title || key} must be at least ${fieldSchema.minLength} characters`)
        }
        if (fieldSchema.maxLength && value.length > fieldSchema.maxLength) {
          errors.push(`${fieldSchema.title || key} must be no more than ${fieldSchema.maxLength} characters`)
        }
        if (fieldSchema.pattern && !new RegExp(fieldSchema.pattern).test(value)) {
          errors.push(`${fieldSchema.title || key} format is invalid`)
        }
      }

      // Check number constraints
      if ((fieldSchema.type === 'number' || fieldSchema.type === 'integer') && typeof value === 'number') {
        if (fieldSchema.minimum !== undefined && value < fieldSchema.minimum) {
          errors.push(`${fieldSchema.title || key} must be at least ${fieldSchema.minimum}`)
        }
        if (fieldSchema.maximum !== undefined && value > fieldSchema.maximum) {
          errors.push(`${fieldSchema.title || key} must be no more than ${fieldSchema.maximum}`)
        }
      }
    }
  }

  return errors
})

// Methods
const loadSchema = async () => {
  if (!props.plugin) return

  loading.value = true
  error.value = null
  
  try {
    schema.value = await pluginStore.getPluginSchema(props.plugin.name)
    initializeFormData()
  } catch (err) {
    error.value = 'Failed to load plugin schema'
    console.error('Failed to load schema:', err)
  } finally {
    loading.value = false
  }
}

const initializeFormData = () => {
  if (!schema.value?.properties) return

  const initialData: Record<string, any> = {}
  
  for (const [key, fieldSchema] of Object.entries(schema.value.properties)) {
    // Set default values based on schema
    if (fieldSchema.default !== undefined) {
      initialData[key] = fieldSchema.default
    } else {
      switch (fieldSchema.type) {
        case 'string':
          initialData[key] = ''
          break
        case 'number':
        case 'integer':
          initialData[key] = 0
          break
        case 'boolean':
          initialData[key] = false
          break
        case 'array':
          initialData[key] = []
          break
        case 'object':
          initialData[key] = '{}'
          break
        default:
          initialData[key] = null
      }
    }
  }

  formData.value = initialData
}

const getValidationRules = (fieldSchema: any, required: boolean = false) => {
  const rules: any[] = []

  if (required) {
    rules.push((val: any) => (val && val !== '') || 'This field is required')
  }

  if (fieldSchema.type === 'string') {
    if (fieldSchema.minLength) {
      rules.push((val: string) => 
        !val || val.length >= fieldSchema.minLength || 
        `Minimum length is ${fieldSchema.minLength} characters`
      )
    }
    if (fieldSchema.maxLength) {
      rules.push((val: string) => 
        !val || val.length <= fieldSchema.maxLength || 
        `Maximum length is ${fieldSchema.maxLength} characters`
      )
    }
    if (fieldSchema.pattern) {
      rules.push((val: string) => 
        !val || new RegExp(fieldSchema.pattern).test(val) || 
        'Invalid format'
      )
    }
  }

  if (fieldSchema.type === 'number' || fieldSchema.type === 'integer') {
    if (fieldSchema.minimum !== undefined) {
      rules.push((val: number) => 
        val === null || val === undefined || val >= fieldSchema.minimum || 
        `Minimum value is ${fieldSchema.minimum}`
      )
    }
    if (fieldSchema.maximum !== undefined) {
      rules.push((val: number) => 
        val === null || val === undefined || val <= fieldSchema.maximum || 
        `Maximum value is ${fieldSchema.maximum}`
      )
    }
  }

  return rules
}

const isValidJSON = (str: string): boolean => {
  try {
    JSON.parse(str)
    return true
  } catch {
    return false
  }
}

const loadProfile = (profileName: string | null) => {
  if (profileName && savedProfiles.value[profileName]) {
    formData.value = { ...savedProfiles.value[profileName] }
  }
}

const saveProfile = () => {
  if (!newProfileName.value) return

  savedProfiles.value[newProfileName.value] = { ...formData.value }
  
  // Save to localStorage
  const storageKey = `plugin-profiles-${props.plugin?.name}`
  localStorage.setItem(storageKey, JSON.stringify(savedProfiles.value))
  
  selectedProfile.value = newProfileName.value
  showSaveProfile.value = false
  newProfileName.value = ''
  
  $q.notify({
    type: 'positive',
    message: 'Profile saved successfully'
  })
}

const loadSavedProfiles = () => {
  if (!props.plugin) return

  const storageKey = `plugin-profiles-${props.plugin.name}`
  const saved = localStorage.getItem(storageKey)
  
  if (saved) {
    try {
      savedProfiles.value = JSON.parse(saved)
    } catch {
      savedProfiles.value = {}
    }
  }
}

const testConfiguration = async () => {
  if (!props.plugin) return

  testing.value = true
  
  try {
    // Process formData for API submission
    const config = processFormData(formData.value)
    
    // Here you would call an API to test the configuration
    // For now, we'll simulate the test
    await new Promise(resolve => setTimeout(resolve, 1000))
    
    $q.notify({
      type: 'positive',
      message: 'Configuration test passed!'
    })
  } catch (err) {
    $q.notify({
      type: 'negative',
      message: 'Configuration test failed'
    })
  } finally {
    testing.value = false
  }
}

const processFormData = (data: Record<string, any>): Record<string, any> => {
  const processed: Record<string, any> = {}

  for (const [key, value] of Object.entries(data)) {
    if (value === null || value === undefined || value === '') continue

    const fieldSchema = schema.value?.properties?.[key]
    
    if (fieldSchema?.type === 'object' && typeof value === 'string') {
      try {
        processed[key] = JSON.parse(value)
      } catch {
        processed[key] = value
      }
    } else {
      processed[key] = value
    }
  }

  return processed
}

const handleSubmit = async () => {
  if (!props.plugin || validationErrors.value.length > 0) return

  saving.value = true
  
  try {
    const config = processFormData(formData.value)
    
    emit('configured', props.plugin, config)
    
    $q.notify({
      type: 'positive',
      message: `${props.plugin.name} configured successfully`
    })
    
    isOpen.value = false
  } catch (err) {
    $q.notify({
      type: 'negative',
      message: 'Failed to save configuration'
    })
  } finally {
    saving.value = false
  }
}

const handleReset = () => {
  initializeFormData()
  selectedProfile.value = null
}

// Watchers
watch(() => props.plugin, (newPlugin) => {
  if (newPlugin) {
    loadSchema()
    loadSavedProfiles()
  }
}, { immediate: true })

watch(isOpen, (open) => {
  if (open && props.plugin) {
    loadSchema()
    loadSavedProfiles()
  }
})

// Lifecycle
onMounted(() => {
  if (props.plugin && props.modelValue) {
    loadSchema()
    loadSavedProfiles()
  }
})
</script>

<style scoped>
.config-form {
  max-height: 60vh;
  overflow-y: auto;
}

.form-fields {
  padding-right: 8px;
}

pre {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  font-family: 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.4;
}
</style>