<template>
  <div class="gitops-form">
    <div class="form-header">
      <h2>Create GitOps Export</h2>
      <button class="close-button" @click="$emit('cancel')" type="button">✖</button>
    </div>

    <form @submit.prevent="onSubmit" class="form-content">
      <!-- Basic Information -->
      <div class="form-section">
        <h3>Basic Information</h3>
        
        <div class="form-field">
          <label class="field-label">
            Export Name *
            <span class="field-help">A descriptive name for this GitOps export</span>
          </label>
          <input
            v-model="formData.name"
            type="text"
            required
            maxlength="100"
            placeholder="e.g. Production Devices GitOps"
            class="form-input"
            :class="{ error: errors.name }"
          />
          <div v-if="errors.name" class="field-error">{{ errors.name }}</div>
        </div>

        <div class="form-field">
          <label class="field-label">
            Description
            <span class="field-help">Optional description of the export purpose</span>
          </label>
          <textarea
            v-model="formData.description"
            maxlength="500"
            placeholder="e.g. Infrastructure as code configuration for Kubernetes deployment"
            class="form-textarea"
            rows="2"
          ></textarea>
        </div>
      </div>

      <!-- Export Format -->
      <div class="form-section">
        <h3>Export Format</h3>
        
        <div class="form-field">
          <label class="field-label">
            Target Platform *
            <span class="field-help">Choose the infrastructure platform</span>
          </label>
          <select
            v-model="formData.format"
            @change="onFormatChange"
            required
            class="form-select"
            :class="{ error: errors.format }"
          >
            <option value="">Select platform...</option>
            <option value="terraform">Terraform - Infrastructure as Code</option>
            <option value="ansible">Ansible - Configuration Management</option>
            <option value="kubernetes">Kubernetes - Container Orchestration</option>
            <option value="docker-compose">Docker Compose - Container Deployment</option>
            <option value="yaml">Generic YAML - Structured Configuration</option>
          </select>
          <div v-if="errors.format" class="field-error">{{ errors.format }}</div>
        </div>

        <div class="form-field">
          <label class="field-label">
            Repository Structure *
            <span class="field-help">How to organize the generated files</span>
          </label>
          <select
            v-model="formData.repository_structure"
            required
            class="form-select"
            :class="{ error: errors.repository_structure }"
          >
            <option value="">Select structure...</option>
            <option value="monorepo">Monorepo - All devices in single repository</option>
            <option value="hierarchical">Hierarchical - Organized by device type/location</option>
            <option value="per-device">Per Device - Separate structure for each device</option>
            <option value="flat">Flat - All files in root directory</option>
          </select>
          <div v-if="errors.repository_structure" class="field-error">{{ errors.repository_structure }}</div>
        </div>
      </div>

      <!-- Device Selection -->
      <div class="form-section">
        <h3>Device Selection</h3>
        
        <div class="device-selection">
          <label class="checkbox-label">
            <input
              type="radio"
              :value="true"
              v-model="selectAllDevices"
              class="form-radio"
            />
            <span>All devices ({{ availableDevices.length }} devices)</span>
            <span class="field-help">Include all discovered devices in export</span>
          </label>
          
          <label class="checkbox-label">
            <input
              type="radio"
              :value="false"
              v-model="selectAllDevices"
              class="form-radio"
            />
            <span>Select specific devices</span>
            <span class="field-help">Choose individual devices to export</span>
          </label>
        </div>

        <div v-if="!selectAllDevices" class="device-list">
          <div class="device-list-header">
            <div class="device-count">{{ selectedDevices.length }} of {{ availableDevices.length }} selected</div>
            <div class="device-actions">
              <button type="button" @click="selectAllInList" class="select-all-btn">Select All</button>
              <button type="button" @click="clearSelection" class="clear-all-btn">Clear All</button>
            </div>
          </div>
          
          <div class="device-checkboxes" v-if="availableDevices.length > 0">
            <label 
              v-for="device in availableDevices" 
              :key="device.id"
              class="device-checkbox"
            >
              <input
                type="checkbox"
                :value="device.id"
                v-model="selectedDevices"
                class="device-checkbox-input"
              />
              <div class="device-info">
                <div class="device-name">{{ device.name || device.ip }}</div>
                <div class="device-details">
                  {{ device.type }} • {{ device.ip }} • {{ device.status }}
                </div>
              </div>
            </label>
          </div>
          
          <div v-else class="no-devices">
            No devices available. Please discover devices first.
          </div>
        </div>
      </div>

      <!-- Format-Specific Options -->
      <div class="form-section" v-if="formData.format">
        <h3>{{ formatTitle(formData.format) }} Configuration</h3>
        
        <!-- Terraform Options -->
        <div v-if="formData.format === 'terraform'" class="format-options">
          <div class="form-field">
            <label class="field-label">
              Provider Version
              <span class="field-help">Terraform provider version constraint</span>
            </label>
            <input
              v-model="terraformOptions.provider_version"
              type="text"
              placeholder="e.g. >= 1.0, < 2.0"
              class="form-input"
            />
          </div>

          <div class="form-field">
            <label class="field-label">Module Structure</label>
            <select v-model="terraformOptions.module_structure" class="form-select">
              <option value="single">Single module for all devices</option>
              <option value="per-device">Separate module per device</option>
              <option value="per-type">Module per device type</option>
            </select>
          </div>

          <div class="checkbox-options">
            <label class="checkbox-label">
              <input v-model="terraformOptions.include_data_sources" type="checkbox" />
              <span>Include data sources</span>
              <span class="field-help">Generate data source blocks for discovery</span>
            </label>
            <label class="checkbox-label">
              <input v-model="terraformOptions.variable_files" type="checkbox" />
              <span>Generate variable files</span>
              <span class="field-help">Create separate .tfvars files</span>
            </label>
          </div>
        </div>

        <!-- Ansible Options -->
        <div v-if="formData.format === 'ansible'" class="format-options">
          <div class="form-field">
            <label class="field-label">Playbook Structure</label>
            <select v-model="ansibleOptions.playbook_structure" class="form-select">
              <option value="single">Single playbook for all devices</option>
              <option value="per-device">Separate playbook per device</option>
              <option value="roles">Role-based structure</option>
            </select>
          </div>

          <div class="form-field">
            <label class="field-label">Inventory Format</label>
            <select v-model="ansibleOptions.inventory_format" class="form-select">
              <option value="ini">INI format</option>
              <option value="yaml">YAML format</option>
            </select>
          </div>

          <div class="checkbox-options">
            <label class="checkbox-label">
              <input v-model="ansibleOptions.include_vault" type="checkbox" />
              <span>Use Ansible Vault</span>
              <span class="field-help">Encrypt sensitive variables</span>
            </label>
            <label class="checkbox-label">
              <input v-model="ansibleOptions.use_collections" type="checkbox" />
              <span>Use Ansible Collections</span>
              <span class="field-help">Generate collection-based playbooks</span>
            </label>
          </div>
        </div>

        <!-- Kubernetes Options -->
        <div v-if="formData.format === 'kubernetes'" class="format-options">
          <div class="form-field">
            <label class="field-label">
              Namespace
              <span class="field-help">Kubernetes namespace for resources</span>
            </label>
            <input
              v-model="kubernetesOptions.namespace"
              type="text"
              placeholder="e.g. shelly-manager"
              class="form-input"
            />
          </div>

          <div class="form-field">
            <label class="field-label">API Version</label>
            <select v-model="kubernetesOptions.api_version" class="form-select">
              <option value="apps/v1">apps/v1</option>
              <option value="v1">v1</option>
              <option value="networking.k8s.io/v1">networking.k8s.io/v1</option>
            </select>
          </div>

          <div class="form-field">
            <label class="field-label">ConfigMap Structure</label>
            <select v-model="kubernetesOptions.config_map_structure" class="form-select">
              <option value="single">Single ConfigMap for all devices</option>
              <option value="per-device">Separate ConfigMap per device</option>
            </select>
          </div>

          <div class="checkbox-options">
            <label class="checkbox-label">
              <input v-model="kubernetesOptions.use_kustomize" type="checkbox" />
              <span>Generate Kustomize configuration</span>
              <span class="field-help">Create kustomization.yaml files</span>
            </label>
            <label class="checkbox-label">
              <input v-model="kubernetesOptions.include_rbac" type="checkbox" />
              <span>Include RBAC resources</span>
              <span class="field-help">Generate roles and service accounts</span>
            </label>
          </div>
        </div>

        <!-- Docker Compose Options -->
        <div v-if="formData.format === 'docker-compose'" class="format-options">
          <div class="form-field">
            <label class="field-label">Compose Version</label>
            <select v-model="dockerComposeOptions.version" class="form-select">
              <option value="3.8">3.8</option>
              <option value="3.9">3.9</option>
              <option value="3.7">3.7</option>
            </select>
          </div>

          <div class="form-field">
            <label class="field-label">Network Mode</label>
            <select v-model="dockerComposeOptions.network_mode" class="form-select">
              <option value="bridge">Bridge</option>
              <option value="host">Host</option>
              <option value="custom">Custom</option>
            </select>
          </div>

          <div class="checkbox-options">
            <label class="checkbox-label">
              <input v-model="dockerComposeOptions.include_volumes" type="checkbox" />
              <span>Include volumes</span>
              <span class="field-help">Generate volume definitions</span>
            </label>
            <label class="checkbox-label">
              <input v-model="dockerComposeOptions.use_profiles" type="checkbox" />
              <span>Use profiles</span>
              <span class="field-help">Enable Docker Compose profiles</span>
            </label>
          </div>
        </div>
      </div>

      <!-- Git Configuration -->
      <div class="form-section">
        <h3>Git Configuration (Optional)</h3>
        
        <div class="form-field">
          <label class="field-label">
            Repository URL
            <span class="field-help">Git repository for CI/CD integration</span>
          </label>
          <input
            v-model="gitConfig.repository_url"
            type="url"
            placeholder="https://github.com/user/repo.git"
            class="form-input"
          />
        </div>

        <div class="form-field">
          <label class="field-label">
            Target Branch
            <span class="field-help">Git branch for the configuration</span>
          </label>
          <input
            v-model="gitConfig.branch"
            type="text"
            placeholder="main"
            class="form-input"
          />
        </div>

        <div class="form-field">
          <label class="field-label">
            Author Name
          </label>
          <input
            v-model="gitConfig.author_name"
            type="text"
            placeholder="Shelly Manager"
            class="form-input"
          />
        </div>

        <div class="form-field">
          <label class="field-label">
            Author Email
          </label>
          <input
            v-model="gitConfig.author_email"
            type="email"
            placeholder="admin@example.com"
            class="form-input"
          />
        </div>

        <div class="checkbox-options">
          <label class="checkbox-label">
            <input v-model="gitConfig.use_webhooks" type="checkbox" />
            <span>Configure webhooks</span>
            <span class="field-help">Set up automatic CI/CD triggers</span>
          </label>
        </div>

        <div v-if="gitConfig.use_webhooks" class="webhook-config">
          <div class="form-field">
            <label class="field-label">
              Webhook Secret
              <span class="field-help">Secret for webhook authentication</span>
            </label>
            <input
              v-model="gitConfig.webhook_secret"
              type="password"
              placeholder="Enter webhook secret"
              class="form-input"
            />
          </div>
        </div>
      </div>

      <!-- Additional Options -->
      <div class="form-section">
        <h3>Additional Options</h3>
        
        <div class="checkbox-options">
          <label class="checkbox-label">
            <input v-model="formData.include_secrets" type="checkbox" />
            <span>Include sensitive data</span>
            <span class="field-help">Export passwords, tokens, and other secrets</span>
          </label>
          
          <label class="checkbox-label">
            <input v-model="formData.generate_readme" type="checkbox" />
            <span>Generate README file</span>
            <span class="field-help">Create documentation for the export</span>
          </label>
        </div>

        <!-- Variable Substitution -->
        <div class="variable-substitution">
          <h4>Variable Substitution</h4>
          <div class="variable-help">
            Define variables that will be substituted in the generated files
          </div>
          
          <div v-for="(value, key, index) in formData.variable_substitution" :key="index" class="variable-row">
            <input
              :value="key"
              @input="updateVariableKey(key, $event.target.value)"
              placeholder="Variable name"
              class="variable-key form-input"
            />
            <input
              v-model="formData.variable_substitution[key]"
              placeholder="Variable value"
              class="variable-value form-input"
            />
            <button 
              type="button" 
              @click="removeVariable(key)" 
              class="remove-variable-btn"
              title="Remove variable"
            >
              ✖
            </button>
          </div>
          
          <button type="button" @click="addVariable" class="add-variable-btn">
            ➕ Add Variable
          </button>
        </div>
      </div>

      <!-- Size Estimation -->
      <div class="form-section" v-if="sizeEstimate">
        <h3>Estimated Output</h3>
        <div class="size-estimate">
          <div class="estimate-item">
            <strong>Devices:</strong> {{ estimatedDeviceCount }} devices
          </div>
          <div class="estimate-item">
            <strong>Estimated Files:</strong> {{ estimatedFileCount }} files
          </div>
          <div class="estimate-item">
            <strong>Estimated Size:</strong> {{ formatFileSize(sizeEstimate) }}
          </div>
        </div>
      </div>

      <!-- Preview Section -->
      <div v-if="previewData" class="preview-section">
        <h3>Export Preview</h3>
        <div class="preview-content">
          <div class="preview-status" :class="{ success: previewData.success, warning: !previewData.success }">
            <strong>Status:</strong> {{ previewData.success ? 'Ready to export' : 'Issues detected' }}
          </div>
          
          <div v-if="previewData.structure_preview?.length" class="structure-preview">
            <h4>File Structure</h4>
            <div class="structure-tree">
              <div v-for="path in previewData.structure_preview" :key="path" class="structure-item">
                {{ path }}
              </div>
            </div>
          </div>

          <div v-if="previewData.template_validation" class="validation-results">
            <h4>Validation Results</h4>
            <div class="validation-status" :class="{ valid: previewData.template_validation.valid }">
              {{ previewData.template_validation.valid ? '✅ All validations passed' : '❌ Validation issues found' }}
            </div>
            
            <!-- Format-specific validation -->
            <div v-if="previewData.template_validation.terraform" class="format-validation">
              <strong>Terraform:</strong>
              <span :class="{ success: previewData.template_validation.terraform.syntax_valid }">
                Syntax {{ previewData.template_validation.terraform.syntax_valid ? 'Valid' : 'Invalid' }}
              </span>
            </div>
          </div>

          <div v-if="previewData.warnings?.length" class="preview-warnings">
            <h4>⚠️ Warnings</h4>
            <ul class="warnings-list">
              <li v-for="warning in previewData.warnings" :key="warning" class="warning-item">
                {{ warning }}
              </li>
            </ul>
          </div>
        </div>
      </div>

      <!-- Error Display -->
      <div v-if="error" class="form-error">
        <strong>Error:</strong> {{ error }}
      </div>

      <!-- Form Actions -->
      <div class="form-actions">
        <button type="button" @click="$emit('cancel')" class="secondary-button">
          Cancel
        </button>
        <button
          type="button"
          @click="onPreview"
          :disabled="!isFormValid || loading"
          class="preview-button"
        >
          <span v-if="previewing">Generating Preview...</span>
          <span v-else>👁️ Preview</span>
        </button>
        <button
          type="submit"
          :disabled="!isFormValid || loading || !previewData?.success"
          class="primary-button"
        >
          <span v-if="loading">Creating Export...</span>
          <span v-else>🚀 Create Export</span>
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { 
  previewGitOpsExport,
  type GitOpsExportRequest,
  type GitOpsExportPreview,
  type GitOpsTemplateOptions,
  type GitOpsGitConfig
} from '@/api/export'
import type { Device } from '@/api/types'

const props = defineProps<{
  loading?: boolean
  error?: string
  availableDevices?: Device[]
}>()

const emit = defineEmits<{
  submit: [GitOpsExportRequest]
  preview: [GitOpsExportRequest]
  cancel: []
}>()

// Form data
const formData = reactive<GitOpsExportRequest>({
  name: '',
  description: '',
  format: 'terraform',
  devices: [],
  repository_structure: 'hierarchical',
  template_options: {},
  git_config: {},
  variable_substitution: {},
  include_secrets: false,
  generate_readme: true
})

// Format-specific options
const terraformOptions = reactive({
  provider_version: '>=1.0',
  module_structure: 'per-type' as 'single' | 'per-device' | 'per-type',
  include_data_sources: true,
  variable_files: true
})

const ansibleOptions = reactive({
  playbook_structure: 'roles' as 'single' | 'per-device' | 'roles',
  inventory_format: 'yaml' as 'ini' | 'yaml',
  include_vault: false,
  use_collections: true
})

const kubernetesOptions = reactive({
  api_version: 'apps/v1',
  namespace: 'shelly-manager',
  use_kustomize: true,
  include_rbac: false,
  config_map_structure: 'per-device' as 'single' | 'per-device'
})

const dockerComposeOptions = reactive({
  version: '3.8',
  network_mode: 'bridge' as 'bridge' | 'host' | 'custom',
  include_volumes: true,
  use_profiles: false
})

const gitConfig = reactive<GitOpsGitConfig>({
  repository_url: '',
  branch: 'main',
  author_name: 'Shelly Manager',
  author_email: '',
  use_webhooks: false,
  webhook_secret: ''
})

// Form state
const selectedDevices = ref<number[]>([])
const selectAllDevices = ref(true)
const sizeEstimate = ref(0)
const previewData = ref<GitOpsExportPreview | null>(null)
const previewing = ref(false)

// Validation errors
const errors = reactive<Record<string, string>>({})

// Computed properties
const availableDevices = computed(() => props.availableDevices || [])

const estimatedDeviceCount = computed(() => 
  selectAllDevices.value ? availableDevices.value.length : selectedDevices.value.length
)

const estimatedFileCount = computed(() => {
  let files = estimatedDeviceCount.value

  // Adjust based on format and structure
  if (formData.format === 'terraform') {
    if (terraformOptions.module_structure === 'per-device') files *= 2
    if (terraformOptions.variable_files) files += 1
  } else if (formData.format === 'kubernetes') {
    files *= 2 // ConfigMap + Deployment per device
    if (kubernetesOptions.use_kustomize) files += 1
    if (kubernetesOptions.include_rbac) files += 2
  }

  if (formData.generate_readme) files += 1

  return Math.max(files, 1)
})

const isFormValid = computed(() => {
  const hasName = formData.name.trim().length > 0
  const hasFormat = formData.format.length > 0
  const hasStructure = formData.repository_structure.length > 0
  const hasDevices = selectAllDevices.value || selectedDevices.value.length > 0
  const noErrors = Object.keys(errors).length === 0
  
  return hasName && hasFormat && hasStructure && hasDevices && noErrors
})

// Methods
function selectAllInList() {
  selectedDevices.value = availableDevices.value.map(d => d.id)
}

function clearSelection() {
  selectedDevices.value = []
}

function onFormatChange() {
  // Reset preview when format changes
  previewData.value = null
  updateTemplateOptions()
}

function updateTemplateOptions() {
  const options: GitOpsTemplateOptions = {}

  if (formData.format === 'terraform') {
    options.terraform = { ...terraformOptions }
  } else if (formData.format === 'ansible') {
    options.ansible = { ...ansibleOptions }
  } else if (formData.format === 'kubernetes') {
    options.kubernetes = { ...kubernetesOptions }
  } else if (formData.format === 'docker-compose') {
    options.docker_compose = { ...dockerComposeOptions }
  }

  formData.template_options = options
  formData.git_config = { ...gitConfig }
}

function addVariable() {
  const key = `VAR_${Object.keys(formData.variable_substitution || {}).length + 1}`
  if (!formData.variable_substitution) {
    formData.variable_substitution = {}
  }
  formData.variable_substitution[key] = ''
}

function removeVariable(key: string) {
  if (formData.variable_substitution) {
    delete formData.variable_substitution[key]
  }
}

function updateVariableKey(oldKey: string, newKey: string) {
  if (!formData.variable_substitution || oldKey === newKey) return
  
  const value = formData.variable_substitution[oldKey]
  delete formData.variable_substitution[oldKey]
  if (newKey.trim()) {
    formData.variable_substitution[newKey] = value
  }
}

function validateForm() {
  // Clear previous errors
  Object.keys(errors).forEach(key => delete errors[key])

  // Name validation
  if (!formData.name.trim()) {
    errors.name = 'Export name is required'
  } else if (formData.name.length > 100) {
    errors.name = 'Name must be 100 characters or less'
  }

  // Format validation
  if (!formData.format) {
    errors.format = 'Format is required'
  }

  // Repository structure validation
  if (!formData.repository_structure) {
    errors.repository_structure = 'Repository structure is required'
  }

  // Device validation
  if (!selectAllDevices.value && selectedDevices.value.length === 0) {
    errors.devices = 'At least one device must be selected'
  }
}

function calculateSizeEstimate() {
  // Simple size estimation based on device count, format, and options
  let baseSize = estimatedDeviceCount.value * 1024 // ~1KB per device base
  let files = estimatedFileCount.value

  // Format-specific size adjustments
  if (formData.format === 'terraform') {
    baseSize *= 2 // Terraform files are larger
    if (terraformOptions.variable_files) baseSize += 512
  } else if (formData.format === 'kubernetes') {
    baseSize *= 1.5 // YAML manifests
    if (kubernetesOptions.include_rbac) baseSize += 1024
  } else if (formData.format === 'ansible') {
    baseSize *= 1.2 // Playbooks
    if (ansibleOptions.include_vault) baseSize *= 1.1
  }

  // Structure adjustments
  if (formData.repository_structure === 'per-device') {
    baseSize *= 1.3 // More files, more overhead
  } else if (formData.repository_structure === 'flat') {
    baseSize *= 0.9 // Less directory structure
  }

  // Additional options
  if (formData.include_secrets) baseSize += 256
  if (formData.generate_readme) baseSize += 2048

  sizeEstimate.value = Math.max(baseSize, 1024) // Minimum 1KB
}

async function onPreview() {
  validateForm()
  
  if (!isFormValid.value) return
  
  previewing.value = true
  previewData.value = null
  
  try {
    updateTemplateOptions()
    
    const request: GitOpsExportRequest = {
      ...formData,
      devices: selectAllDevices.value ? undefined : selectedDevices.value
    }

    const result = await previewGitOpsExport(request)
    previewData.value = result.preview
    
    emit('preview', request)
  } catch (err: any) {
    console.error('Preview failed:', err)
    // Error handling is done by parent component
  } finally {
    previewing.value = false
  }
}

function onSubmit() {
  validateForm()
  
  if (!isFormValid.value) {
    return
  }
  
  updateTemplateOptions()
  
  // Prepare the request
  const request: GitOpsExportRequest = {
    ...formData,
    devices: selectAllDevices.value ? undefined : selectedDevices.value
  }

  emit('submit', request)
}

function formatTitle(format: string): string {
  const titles: Record<string, string> = {
    terraform: 'Terraform',
    ansible: 'Ansible',
    kubernetes: 'Kubernetes',
    'docker-compose': 'Docker Compose',
    yaml: 'YAML'
  }
  return titles[format] || format
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

// Watchers
watch([selectAllDevices, selectedDevices, formData, terraformOptions, ansibleOptions, kubernetesOptions, dockerComposeOptions], 
  calculateSizeEstimate, { deep: true })
watch([formData], validateForm, { deep: true })

// Initialize with one variable
formData.variable_substitution = { 'ENVIRONMENT': 'production' }
</script>

<style scoped>
.gitops-form {
  background: white;
  border-radius: 8px;
  max-height: 90vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px;
  border-bottom: 1px solid #e5e7eb;
}

.form-header h2 {
  margin: 0;
  color: #1f2937;
  font-size: 1.5rem;
}

.close-button {
  background: none;
  border: none;
  color: #6b7280;
  cursor: pointer;
  font-size: 1.2rem;
  padding: 4px;
  line-height: 1;
  transition: color 0.2s;
}

.close-button:hover {
  color: #374151;
}

.form-content {
  padding: 24px;
  overflow-y: auto;
}

.form-section {
  margin-bottom: 32px;
}

.form-section h3 {
  margin: 0 0 16px 0;
  color: #1f2937;
  font-size: 1.125rem;
  font-weight: 600;
  border-bottom: 1px solid #e5e7eb;
  padding-bottom: 8px;
}

.form-section h4 {
  margin: 0 0 12px 0;
  color: #374151;
  font-size: 1rem;
  font-weight: 600;
}

.form-field {
  margin-bottom: 20px;
}

.field-label {
  display: block;
  font-weight: 500;
  color: #374151;
  margin-bottom: 6px;
  font-size: 0.875rem;
}

.field-help {
  display: block;
  font-weight: 400;
  color: #6b7280;
  font-size: 0.75rem;
  margin-top: 2px;
}

.form-input, .form-select, .form-textarea {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 0.875rem;
  transition: border-color 0.2s, box-shadow 0.2s;
  background: white;
}

.form-input:focus, .form-select:focus, .form-textarea:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.form-input.error, .form-select.error, .form-textarea.error {
  border-color: #dc2626;
}

.form-textarea {
  resize: vertical;
  min-height: 60px;
}

.checkbox-label {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  cursor: pointer;
  margin-bottom: 12px;
}

.form-checkbox, .form-radio {
  width: auto;
  margin: 0;
}

.checkbox-label span {
  font-weight: 500;
  color: #374151;
}

.device-selection {
  margin-bottom: 16px;
}

.device-list {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  background: #f9fafb;
  padding: 16px;
}

.device-list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.device-count {
  font-weight: 500;
  color: #374151;
}

.device-actions {
  display: flex;
  gap: 8px;
}

.select-all-btn, .clear-all-btn {
  background: none;
  border: 1px solid #d1d5db;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.select-all-btn:hover, .clear-all-btn:hover {
  background: #e5e7eb;
}

.device-checkboxes {
  display: grid;
  grid-template-columns: 1fr;
  gap: 8px;
  max-height: 200px;
  overflow-y: auto;
}

.device-checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  background: white;
  cursor: pointer;
  transition: background-color 0.2s;
}

.device-checkbox:hover {
  background: #f3f4f6;
}

.device-checkbox-input {
  width: auto;
  margin: 0;
}

.device-info {
  flex: 1;
}

.device-name {
  font-weight: 500;
  color: #1f2937;
  font-size: 0.875rem;
}

.device-details {
  font-size: 0.75rem;
  color: #6b7280;
}

.no-devices {
  text-align: center;
  color: #9ca3af;
  font-style: italic;
  padding: 32px;
}

.format-options, .checkbox-options {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.webhook-config {
  margin-top: 16px;
  padding: 16px;
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
}

.variable-substitution {
  margin-top: 20px;
  padding: 16px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
}

.variable-help {
  font-size: 0.875rem;
  color: #6b7280;
  margin-bottom: 12px;
}

.variable-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
  align-items: center;
}

.variable-key, .variable-value {
  flex: 1;
}

.remove-variable-btn {
  background: #fee2e2;
  border: 1px solid #fecaca;
  color: #dc2626;
  padding: 8px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.75rem;
  line-height: 1;
}

.remove-variable-btn:hover {
  background: #fca5a5;
}

.add-variable-btn {
  background: #dcfce7;
  border: 1px solid #bbf7d0;
  color: #166534;
  padding: 8px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
  transition: background-color 0.2s;
}

.add-variable-btn:hover {
  background: #bbf7d0;
}

.size-estimate {
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
  padding: 16px;
}

.estimate-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-size: 0.875rem;
}

.estimate-item:last-child {
  margin-bottom: 0;
}

.preview-section {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 20px;
}

.preview-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.preview-status {
  padding: 12px;
  border-radius: 6px;
  font-weight: 500;
}

.preview-status.success {
  background: #dcfce7;
  color: #166534;
  border: 1px solid #bbf7d0;
}

.preview-status.warning {
  background: #fef3c7;
  color: #92400e;
  border: 1px solid #fcd34d;
}

.structure-preview h4, .validation-results h4, .preview-warnings h4 {
  margin: 0 0 8px 0;
  color: #374151;
}

.structure-tree {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  padding: 12px;
  max-height: 200px;
  overflow-y: auto;
}

.structure-item {
  font-family: monospace;
  font-size: 0.875rem;
  color: #374151;
  padding: 2px 0;
}

.validation-status {
  padding: 8px 12px;
  border-radius: 4px;
  font-weight: 500;
  margin-bottom: 8px;
}

.validation-status.valid {
  background: #dcfce7;
  color: #166534;
}

.validation-status:not(.valid) {
  background: #fee2e2;
  color: #dc2626;
}

.format-validation {
  font-size: 0.875rem;
  margin-bottom: 4px;
}

.format-validation .success {
  color: #166534;
}

.preview-warnings {
  background: #fffbeb;
  border: 1px solid #fed7aa;
  border-radius: 6px;
  padding: 12px;
}

.warnings-list {
  margin: 0;
  padding-left: 20px;
}

.warning-item {
  color: #92400e;
  margin-bottom: 4px;
  font-size: 0.875rem;
}

.field-error {
  margin-top: 4px;
  color: #dc2626;
  font-size: 0.75rem;
}

.form-error {
  margin-bottom: 20px;
  padding: 12px;
  background: #fee2e2;
  border: 1px solid #fecaca;
  border-radius: 6px;
  color: #dc2626;
  font-size: 0.875rem;
}

.form-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  padding-top: 20px;
  border-top: 1px solid #e5e7eb;
}

.primary-button {
  background-color: #3b82f6;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.primary-button:hover:not(:disabled) {
  background-color: #2563eb;
}

.primary-button:disabled {
  background-color: #9ca3af;
  cursor: not-allowed;
}

.preview-button {
  background-color: #f59e0b;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.preview-button:hover:not(:disabled) {
  background-color: #d97706;
}

.preview-button:disabled {
  background-color: #9ca3af;
  cursor: not-allowed;
}

.secondary-button {
  background-color: #f3f4f6;
  color: #374151;
  border: 1px solid #d1d5db;
  padding: 10px 20px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s, border-color 0.2s;
}

.secondary-button:hover {
  background-color: #e5e7eb;
  border-color: #9ca3af;
}

/* Responsive design */
@media (max-width: 768px) {
  .form-header {
    padding: 16px;
  }

  .form-content {
    padding: 16px;
  }

  .device-list-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .device-checkboxes {
    grid-template-columns: 1fr;
  }

  .form-actions {
    flex-direction: column;
  }

  .variable-row {
    flex-direction: column;
    gap: 8px;
  }

  .remove-variable-btn {
    align-self: flex-end;
  }

  .estimate-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
}
</style>