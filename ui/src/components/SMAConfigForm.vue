<template>
  <div class="sma-config-form">
    <!-- SMA Format Options -->
    <div class="form-section">
      <h4>SMA Configuration</h4>
      <p class="section-description">
        Configure Shelly Management Archive (SMA) specific options for optimal compression and data integrity.
      </p>

      <!-- Compression Settings -->
      <div class="config-group">
        <h5>Compression</h5>
        
        <label class="checkbox-label">
          <input
            v-model="config.compression"
            type="checkbox"
            class="form-checkbox"
          />
          <span>Enable compression</span>
          <span class="field-help">Compress archive using Gzip for smaller file size</span>
        </label>

        <div v-if="config.compression" class="compression-level">
          <label class="field-label">
            Compression Level
            <span class="field-help">Higher levels provide better compression but take longer</span>
          </label>
          <div class="slider-container">
            <input
              v-model.number="config.compression_level"
              type="range"
              min="1"
              max="9"
              step="1"
              class="compression-slider"
            />
            <div class="slider-labels">
              <span>1 (Fast)</span>
              <span class="current-value">{{ config.compression_level }}</span>
              <span>9 (Best)</span>
            </div>
          </div>
          <div class="compression-info">
            <div class="info-item">
              <strong>Speed:</strong> {{ getCompressionSpeed(config.compression_level) }}
            </div>
            <div class="info-item">
              <strong>Ratio:</strong> {{ getCompressionRatio(config.compression_level) }}
            </div>
          </div>
        </div>
      </div>

      <!-- Data Integrity -->
      <div class="config-group">
        <h5>Data Integrity</h5>
        
        <label class="checkbox-label">
          <input
            v-model="config.include_checksums"
            type="checkbox"
            class="form-checkbox"
          />
          <span>Include data checksums</span>
          <span class="field-help">Add SHA-256 checksums for data integrity verification</span>
        </label>
      </div>

      <!-- Data Sections -->
      <div class="config-group">
        <h5>Included Data Sections</h5>
        <p class="group-description">Select which data sections to include in the SMA archive:</p>
        
        <div class="section-checkboxes">
          <label class="checkbox-label">
            <input
              v-model="filters.include_discovered"
              type="checkbox"
              class="form-checkbox"
            />
            <span>Discovered Devices</span>
            <span class="field-help">Include unmanaged devices found during discovery</span>
          </label>

          <label class="checkbox-label">
            <input
              v-model="filters.include_network_settings"
              type="checkbox"
              class="form-checkbox"
            />
            <span>Network Settings</span>
            <span class="field-help">Include WiFi networks and MQTT configuration</span>
          </label>

          <label class="checkbox-label">
            <input
              v-model="filters.include_plugin_configs"
              type="checkbox"
              class="form-checkbox"
            />
            <span>Plugin Configurations</span>
            <span class="field-help">Include enabled plugins and their settings</span>
          </label>

          <label class="checkbox-label">
            <input
              v-model="filters.include_system_settings"
              type="checkbox"
              class="form-checkbox"
            />
            <span>System Settings</span>
            <span class="field-help">Include application-level configuration</span>
          </label>
        </div>
      </div>

      <!-- Export Metadata -->
      <div class="config-group">
        <h5>Export Metadata</h5>
        
        <div class="form-field">
          <label class="field-label">
            Created By (Optional)
            <span class="field-help">Identify who created this export</span>
          </label>
          <input
            v-model="options.created_by"
            type="text"
            placeholder="e.g. admin@company.com"
            maxlength="100"
            class="form-input"
          />
        </div>

        <div class="form-field">
          <label class="field-label">
            Export Type
            <span class="field-help">How this export was initiated</span>
          </label>
          <select v-model="options.export_type" class="form-select">
            <option value="manual">Manual Export</option>
            <option value="scheduled">Scheduled Export</option>
            <option value="api">API Export</option>
          </select>
        </div>
      </div>
    </div>

    <!-- Size Estimation -->
    <div class="form-section" v-if="sizeEstimate">
      <h4>Size Estimation</h4>
      <div class="size-estimate">
        <div class="estimate-row">
          <span class="estimate-label">Original Size:</span>
          <span class="estimate-value">{{ formatFileSize(sizeEstimate.originalSize) }}</span>
        </div>
        <div class="estimate-row" v-if="config.compression">
          <span class="estimate-label">Compressed Size:</span>
          <span class="estimate-value">{{ formatFileSize(sizeEstimate.compressedSize) }}</span>
        </div>
        <div class="estimate-row" v-if="config.compression">
          <span class="estimate-label">Compression Ratio:</span>
          <span class="estimate-value">{{ Math.round((1 - sizeEstimate.compressionRatio) * 100) }}%</span>
        </div>
        <div class="estimate-row">
          <span class="estimate-label">Record Count:</span>
          <span class="estimate-value">{{ sizeEstimate.recordCount }}</span>
        </div>
      </div>
    </div>

    <!-- SMA Format Info -->
    <div class="form-section info-section">
      <h4>📋 SMA Format Information</h4>
      <div class="format-info">
        <div class="info-card">
          <h5>🗂️ What is SMA?</h5>
          <p>SMA (Shelly Management Archive) is a specialized format for complete Shelly device management data export, including devices, templates, configurations, and metadata.</p>
        </div>
        
        <div class="info-card">
          <h5>✨ Key Features</h5>
          <ul>
            <li>JSON-based structure for human readability</li>
            <li>Gzip compression for reduced file size</li>
            <li>SHA-256 checksums for data integrity</li>
            <li>Version compatibility and migration support</li>
            <li>Selective section import/export</li>
          </ul>
        </div>

        <div class="info-card">
          <h5>🔒 Security Considerations</h5>
          <p>SMA files may contain sensitive information such as WiFi passwords, MQTT credentials, and API keys. Always store and transmit SMA files securely.</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, computed, watch } from 'vue'
import type { SMAExportRequest } from '@/api/export'

interface SMAConfig {
  compression: boolean
  compression_level: number
  include_checksums: boolean
}

interface SMAFilters {
  include_discovered: boolean
  include_network_settings: boolean
  include_plugin_configs: boolean
  include_system_settings: boolean
}

interface SMAOptions {
  created_by: string
  export_type: 'manual' | 'scheduled' | 'api'
}

interface SizeEstimate {
  originalSize: number
  compressedSize: number
  compressionRatio: number
  recordCount: number
}

const props = defineProps<{
  deviceCount?: number
  templateCount?: number
  includeDiscovered?: boolean
}>()

const emit = defineEmits<{
  'update:config': [Partial<SMAExportRequest>]
  'update:sizeEstimate': [SizeEstimate]
}>()

// Form state
const config = reactive<SMAConfig>({
  compression: true,
  compression_level: 6,
  include_checksums: true
})

const filters = reactive<SMAFilters>({
  include_discovered: true,
  include_network_settings: true,
  include_plugin_configs: false,
  include_system_settings: false
})

const options = reactive<SMAOptions>({
  created_by: '',
  export_type: 'manual'
})

// Computed size estimate
const sizeEstimate = computed((): SizeEstimate => {
  const deviceCount = props.deviceCount || 0
  const templateCount = props.templateCount || 0
  let recordCount = deviceCount + templateCount
  
  // Base size calculation
  let originalSize = 2048 // Base JSON structure
  originalSize += deviceCount * 1500 // ~1.5KB per device
  originalSize += templateCount * 800 // ~800B per template
  
  // Add discovered devices
  if (filters.include_discovered) {
    const discoveredCount = Math.ceil(deviceCount * 0.2) // Estimate 20% discovered
    originalSize += discoveredCount * 300
    recordCount += discoveredCount
  }
  
  // Add other sections
  if (filters.include_network_settings) originalSize += 1024
  if (filters.include_plugin_configs) originalSize += 2048
  if (filters.include_system_settings) originalSize += 512
  
  const compressionRatio = config.compression ? getCompressionRatioNumeric(config.compression_level) : 1.0
  const compressedSize = Math.round(originalSize * compressionRatio)
  
  return {
    originalSize,
    compressedSize,
    compressionRatio,
    recordCount
  }
})

// Methods
function getCompressionSpeed(level: number): string {
  if (level <= 3) return 'Very Fast'
  if (level <= 5) return 'Fast'
  if (level <= 7) return 'Balanced'
  return 'Slow (Best Compression)'
}

function getCompressionRatio(level: number): string {
  const ratio = getCompressionRatioNumeric(level)
  return `~${Math.round((1 - ratio) * 100)}% smaller`
}

function getCompressionRatioNumeric(level: number): number {
  // Estimated compression ratios for different levels
  const ratios = {
    1: 0.85, 2: 0.80, 3: 0.75,
    4: 0.70, 5: 0.68, 6: 0.65,
    7: 0.62, 8: 0.60, 9: 0.58
  }
  return ratios[level as keyof typeof ratios] || 0.65
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

// Watchers to emit configuration updates
watch([config, filters, options], () => {
  const exportRequest: Partial<SMAExportRequest> = {
    plugin_name: 'sma',
    format: 'sma',
    config: {
      compression: config.compression,
      compression_level: config.compression_level,
      include_checksums: config.include_checksums
    },
    filters: {
      include_discovered: filters.include_discovered,
      include_network_settings: filters.include_network_settings,
      include_plugin_configs: filters.include_plugin_configs,
      include_system_settings: filters.include_system_settings
    },
    options: {
      created_by: options.created_by || undefined,
      export_type: options.export_type
    }
  }
  
  emit('update:config', exportRequest)
}, { deep: true })

watch(sizeEstimate, (estimate) => {
  emit('update:sizeEstimate', estimate)
}, { deep: true })
</script>

<style scoped>
.sma-config-form {
  margin-top: 16px;
}

.form-section {
  margin-bottom: 24px;
}

.form-section h4 {
  margin: 0 0 8px 0;
  color: #1f2937;
  font-size: 1rem;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-description {
  margin: 0 0 16px 0;
  color: #6b7280;
  font-size: 0.875rem;
  line-height: 1.4;
}

.config-group {
  margin-bottom: 20px;
  padding: 16px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.config-group h5 {
  margin: 0 0 12px 0;
  color: #374151;
  font-size: 0.875rem;
  font-weight: 600;
}

.group-description {
  margin: 0 0 12px 0;
  color: #6b7280;
  font-size: 0.75rem;
}

.checkbox-label {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  cursor: pointer;
  margin-bottom: 12px;
}

.checkbox-label span {
  font-weight: 500;
  color: #374151;
}

.field-help {
  display: block;
  font-weight: 400 !important;
  color: #6b7280 !important;
  font-size: 0.75rem !important;
  margin-top: 2px;
}

.form-checkbox {
  width: auto;
  margin: 0;
}

.compression-level {
  margin-top: 12px;
  padding: 12px;
  background: white;
  border: 1px solid #d1d5db;
  border-radius: 4px;
}

.field-label {
  display: block;
  font-weight: 500;
  color: #374151;
  margin-bottom: 6px;
  font-size: 0.875rem;
}

.slider-container {
  margin: 8px 0;
}

.compression-slider {
  width: 100%;
  height: 6px;
  border-radius: 3px;
  background: #e5e7eb;
  outline: none;
  cursor: pointer;
}

.slider-labels {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 4px;
  font-size: 0.75rem;
  color: #6b7280;
}

.current-value {
  background: #3b82f6;
  color: white;
  padding: 2px 6px;
  border-radius: 3px;
  font-weight: 500;
}

.compression-info {
  display: flex;
  gap: 16px;
  margin-top: 8px;
  font-size: 0.75rem;
}

.info-item {
  color: #374151;
}

.section-checkboxes {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-field {
  margin-bottom: 16px;
}

.form-input, .form-select {
  width: 100%;
  padding: 8px 10px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  font-size: 0.875rem;
  transition: border-color 0.2s;
}

.form-input:focus, .form-select:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.size-estimate {
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
  padding: 16px;
}

.estimate-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-size: 0.875rem;
}

.estimate-row:last-child {
  margin-bottom: 0;
}

.estimate-label {
  color: #374151;
  font-weight: 500;
}

.estimate-value {
  color: #1f2937;
  font-weight: 600;
}

.info-section {
  background: #fffbf0;
  border: 1px solid #fed7aa;
  border-radius: 6px;
  padding: 20px;
}

.format-info {
  display: grid;
  gap: 16px;
}

.info-card {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  padding: 16px;
}

.info-card h5 {
  margin: 0 0 8px 0;
  color: #1f2937;
  font-size: 0.875rem;
  font-weight: 600;
}

.info-card p {
  margin: 0;
  color: #4b5563;
  font-size: 0.875rem;
  line-height: 1.4;
}

.info-card ul {
  margin: 0;
  padding-left: 16px;
  color: #4b5563;
  font-size: 0.875rem;
  line-height: 1.4;
}

.info-card li {
  margin-bottom: 4px;
}

.info-card li:last-child {
  margin-bottom: 0;
}

/* Responsive design */
@media (max-width: 768px) {
  .compression-info {
    flex-direction: column;
    gap: 4px;
  }

  .format-info {
    gap: 12px;
  }
  
  .info-card {
    padding: 12px;
  }
}
</style>