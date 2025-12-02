<template>
  <div v-if="isOpen" class="modal" @click.self="close">
    <div class="modal-content">
      <div class="modal-header">
        <h2>Bulk Operations</h2>
        <button class="btn-close" @click="close">×</button>
      </div>

      <!-- Operation Selection -->
      <div v-if="!operation && !isProcessing" class="operation-selection">
        <p class="selection-info">
          <strong>{{ selectedDeviceIds.length }}</strong> device(s) selected
        </p>

        <div class="operation-cards">
          <button class="operation-card" @click="operation = 'import'">
            <span class="operation-icon">📥</span>
            <h3>Bulk Import</h3>
            <p>Import configurations to selected devices</p>
          </button>

          <button class="operation-card" @click="operation = 'export'">
            <span class="operation-icon">📤</span>
            <h3>Bulk Export</h3>
            <p>Export configurations from selected devices</p>
          </button>

          <button class="operation-card" @click="operation = 'drift'">
            <span class="operation-icon">🔍</span>
            <h3>Drift Detection</h3>
            <p>Detect configuration drift</p>
          </button>

          <button class="operation-card" @click="operation = 'drift-enhanced'">
            <span class="operation-icon">🔬</span>
            <h3>Enhanced Drift</h3>
            <p>Advanced drift detection with options</p>
          </button>
        </div>
      </div>

      <!-- Import Form -->
      <div v-else-if="operation === 'import' && !isProcessing">
        <h3>Bulk Import Configuration</h3>
        <form @submit.prevent="handleImport">
          <div class="form-field">
            <label for="import-config">Configuration (JSON)</label>
            <textarea
              id="import-config"
              v-model="importConfig"
              class="form-textarea"
              rows="10"
              placeholder='{"wifi": {"ssid": "Network"}}'
              required
            />
          </div>

          <div class="form-field checkbox">
            <input
              id="stop-on-error"
              v-model="stopOnError"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="stop-on-error">Stop on first error</label>
          </div>

          <div class="form-field checkbox">
            <input
              id="validate-only"
              v-model="validateOnly"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="validate-only">Validate only (dry run)</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary">Import</button>
          </div>
        </form>
      </div>

      <!-- Export Form -->
      <div v-else-if="operation === 'export' && !isProcessing">
        <h3>Bulk Export Configuration</h3>
        <form @submit.prevent="handleExport">
          <div class="form-field">
            <label for="export-format">Export Format</label>
            <select id="export-format" v-model="exportFormat" class="form-select">
              <option value="json">JSON</option>
              <option value="yaml">YAML</option>
              <option value="sma">SMA</option>
            </select>
          </div>

          <div class="form-field checkbox">
            <input
              id="include-secrets"
              v-model="includeSecrets"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="include-secrets">Include secrets</label>
          </div>

          <div class="form-field checkbox">
            <input
              id="include-metadata"
              v-model="includeMetadata"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="include-metadata">Include metadata</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary">Export</button>
          </div>
        </form>
      </div>

      <!-- Drift Detection Form -->
      <div v-else-if="operation === 'drift' && !isProcessing">
        <h3>Bulk Drift Detection</h3>
        <form @submit.prevent="handleDriftDetect">
          <div class="form-field checkbox">
            <input
              id="drift-stop-on-error"
              v-model="stopOnError"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="drift-stop-on-error">Stop on first error</label>
          </div>

          <div class="form-field checkbox">
            <input
              id="detailed-report"
              v-model="detailedReport"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="detailed-report">Generate detailed report</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary">Detect Drift</button>
          </div>
        </form>
      </div>

      <!-- Enhanced Drift Detection Form -->
      <div v-else-if="operation === 'drift-enhanced' && !isProcessing">
        <h3>Enhanced Drift Detection</h3>
        <form @submit.prevent="handleDriftDetectEnhanced">
          <div class="form-field">
            <label for="compare-with">Compare With</label>
            <select id="compare-with" v-model="compareWith" class="form-select">
              <option value="template">Template</option>
              <option value="baseline">Baseline</option>
              <option value="peer">Peer Devices</option>
            </select>
          </div>

          <div class="form-field">
            <label for="threshold">Detection Threshold</label>
            <select id="threshold" v-model="threshold" class="form-select">
              <option value="strict">Strict</option>
              <option value="moderate">Moderate</option>
              <option value="relaxed">Relaxed</option>
            </select>
          </div>

          <div class="form-field checkbox">
            <input
              id="include-history"
              v-model="includeHistory"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="include-history">Include historical data</label>
          </div>

          <div class="form-field checkbox">
            <input
              id="enh-detailed-report"
              v-model="detailedReport"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="enh-detailed-report">Generate detailed report</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary">Detect Drift</button>
          </div>
        </form>
      </div>

      <!-- Processing State -->
      <div v-else-if="isProcessing" class="processing">
        <h3>Processing...</h3>
        <div class="progress-bar">
          <div class="progress-fill" :style="{ width: progressPercent + '%' }" />
        </div>
        <p class="progress-text">
          {{ processedCount }} / {{ totalCount }} devices processed
        </p>
      </div>

      <!-- Results -->
      <div v-if="result" class="results">
        <div class="results-summary">
          <h3>Results</h3>
          <div class="summary-stats">
            <div class="stat success">
              <span class="stat-label">Success</span>
              <span class="stat-value">{{ result.successCount }}</span>
            </div>
            <div class="stat failure">
              <span class="stat-label">Failed</span>
              <span class="stat-value">{{ result.failureCount }}</span>
            </div>
            <div class="stat skipped">
              <span class="stat-label">Skipped</span>
              <span class="stat-value">{{ result.skippedCount }}</span>
            </div>
          </div>
        </div>

        <div class="results-details">
          <h4>Device Results</h4>
          <table class="results-table">
            <thead>
              <tr>
                <th>Device</th>
                <th>Status</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="deviceResult in result.results"
                :key="deviceResult.deviceId"
                :class="deviceResult.status"
              >
                <td>{{ deviceResult.deviceName }}</td>
                <td>
                  <span class="status-badge" :class="deviceResult.status">
                    {{ deviceResult.status }}
                  </span>
                </td>
                <td>
                  {{ deviceResult.message || deviceResult.error || '-' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="form-actions">
          <button class="btn" @click="close">Close</button>
          <button v-if="result.operationId" class="btn" @click="downloadResults">
            Download Results
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  bulkImport,
  bulkExport,
  bulkDriftDetect,
  bulkDriftDetectEnhanced,
  type BulkOperationResult
} from '@/api/bulk'

interface Props {
  isOpen: boolean
  selectedDeviceIds: number[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  complete: [result: BulkOperationResult]
}>()

const operation = ref<'import' | 'export' | 'drift' | 'drift-enhanced' | null>(null)
const isProcessing = ref(false)
const result = ref<BulkOperationResult | null>(null)

// Import fields
const importConfig = ref('')
const stopOnError = ref(false)
const validateOnly = ref(false)

// Export fields
const exportFormat = ref<'json' | 'yaml' | 'sma'>('json')
const includeSecrets = ref(false)
const includeMetadata = ref(true)

// Drift fields
const detailedReport = ref(true)
const compareWith = ref<'template' | 'baseline' | 'peer'>('template')
const threshold = ref<'strict' | 'moderate' | 'relaxed'>('moderate')
const includeHistory = ref(false)

// Progress tracking
const processedCount = ref(0)
const totalCount = computed(() => props.selectedDeviceIds.length)
const progressPercent = computed(() => {
  if (totalCount.value === 0) return 0
  return Math.round((processedCount.value / totalCount.value) * 100)
})

function close() {
  operation.value = null
  isProcessing.value = false
  result.value = null
  emit('close')
}

async function handleImport() {
  try {
    isProcessing.value = true
    processedCount.value = 0

    let configs: any[]
    try {
      configs = [JSON.parse(importConfig.value)]
    } catch (e) {
      alert('Invalid JSON configuration')
      isProcessing.value = false
      return
    }

    result.value = await bulkImport({
      deviceIds: props.selectedDeviceIds,
      configurations: configs,
      options: { stopOnError: stopOnError.value, validateOnly: validateOnly.value }
    })

    processedCount.value = totalCount.value
    emit('complete', result.value)
  } catch (e: any) {
    alert(e?.message || 'Bulk import failed')
    isProcessing.value = false
  }
}

async function handleExport() {
  try {
    isProcessing.value = true
    processedCount.value = 0

    result.value = await bulkExport({
      deviceIds: props.selectedDeviceIds,
      options: {
        format: exportFormat.value,
        includeSecrets: includeSecrets.value,
        includeMetadata: includeMetadata.value
      }
    })

    processedCount.value = totalCount.value
    emit('complete', result.value)
  } catch (e: any) {
    alert(e?.message || 'Bulk export failed')
    isProcessing.value = false
  }
}

async function handleDriftDetect() {
  try {
    isProcessing.value = true
    processedCount.value = 0

    result.value = await bulkDriftDetect({
      deviceIds: props.selectedDeviceIds,
      options: { stopOnError: stopOnError.value, detailedReport: detailedReport.value }
    })

    processedCount.value = totalCount.value
    emit('complete', result.value)
  } catch (e: any) {
    alert(e?.message || 'Bulk drift detection failed')
    isProcessing.value = false
  }
}

async function handleDriftDetectEnhanced() {
  try {
    isProcessing.value = true
    processedCount.value = 0

    result.value = await bulkDriftDetectEnhanced({
      deviceIds: props.selectedDeviceIds,
      options: {
        stopOnError: stopOnError.value,
        detailedReport: detailedReport.value,
        includeHistory: includeHistory.value,
        compareWith: compareWith.value,
        threshold: threshold.value
      }
    })

    processedCount.value = totalCount.value
    emit('complete', result.value)
  } catch (e: any) {
    alert(e?.message || 'Enhanced drift detection failed')
    isProcessing.value = false
  }
}

function downloadResults() {
  if (!result.value) return

  const data = JSON.stringify(result.value, null, 2)
  const blob = new Blob([data], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `bulk-operation-${result.value.operationId}.json`
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<style scoped>
.modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: white;
  padding: 24px;
  border-radius: 8px;
  width: 90%;
  max-width: 700px;
  max-height: 90vh;
  overflow-y: auto;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.modal-header h2 {
  margin: 0;
  font-size: 20px;
}

.btn-close {
  padding: 4px 8px;
  border: none;
  background: transparent;
  font-size: 24px;
  cursor: pointer;
  color: #6b7280;
}

.btn-close:hover {
  color: #1f2937;
}

.selection-info {
  text-align: center;
  margin-bottom: 20px;
  font-size: 14px;
  color: #6b7280;
}

.operation-cards {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.operation-card {
  padding: 20px;
  border: 2px solid #e5e7eb;
  border-radius: 8px;
  background: white;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
}

.operation-card:hover {
  border-color: #2563eb;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}

.operation-icon {
  font-size: 32px;
  display: block;
  margin-bottom: 12px;
}

.operation-card h3 {
  margin: 0 0 8px 0;
  font-size: 16px;
  color: #1f2937;
}

.operation-card p {
  margin: 0;
  font-size: 13px;
  color: #6b7280;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 16px;
}

.form-field.checkbox {
  flex-direction: row;
  align-items: center;
}

.form-field.checkbox label {
  margin-left: 8px;
}

.form-field label {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
}

.form-textarea,
.form-select {
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-family: inherit;
  font-size: 14px;
}

.form-textarea {
  resize: vertical;
  font-family: ui-monospace, monospace;
}

.form-checkbox {
  width: 20px;
  height: 20px;
}

.form-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 20px;
}

.btn {
  padding: 8px 16px;
  border: 1px solid #cbd5e1;
  background: #fff;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
}

.btn:hover {
  background: #f8fafc;
}

.btn.primary {
  background: #2563eb;
  color: white;
  border-color: #2563eb;
}

.btn.primary:hover {
  background: #1d4ed8;
}

.processing {
  text-align: center;
  padding: 40px 20px;
}

.processing h3 {
  margin: 0 0 20px 0;
}

.progress-bar {
  height: 24px;
  background: #e5e7eb;
  border-radius: 12px;
  overflow: hidden;
  margin-bottom: 12px;
}

.progress-fill {
  height: 100%;
  background: #2563eb;
  transition: width 0.3s;
}

.progress-text {
  font-size: 14px;
  color: #6b7280;
}

.results {
  margin-top: 20px;
}

.results-summary h3 {
  margin: 0 0 16px 0;
}

.summary-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin-bottom: 24px;
}

.stat {
  padding: 16px;
  border-radius: 8px;
  text-align: center;
}

.stat.success {
  background: #dcfce7;
}

.stat.failure {
  background: #fee2e2;
}

.stat.skipped {
  background: #fef3c7;
}

.stat-label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: #6b7280;
  margin-bottom: 4px;
  text-transform: uppercase;
}

.stat-value {
  display: block;
  font-size: 24px;
  font-weight: 700;
  color: #1f2937;
}

.results-details h4 {
  margin: 0 0 12px 0;
  font-size: 14px;
  font-weight: 600;
}

.results-table {
  width: 100%;
  border-collapse: collapse;
}

.results-table thead {
  background: #f9fafb;
}

.results-table th {
  padding: 8px 12px;
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: #6b7280;
  text-transform: uppercase;
  border-bottom: 1px solid #e5e7eb;
}

.results-table td {
  padding: 8px 12px;
  border-bottom: 1px solid #f3f4f6;
  font-size: 13px;
}

.results-table tbody tr.success {
  background: #f0fdf4;
}

.results-table tbody tr.failed {
  background: #fef2f2;
}

.status-badge {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
}

.status-badge.success {
  background: #dcfce7;
  color: #166534;
}

.status-badge.failed {
  background: #fee2e2;
  color: #991b1b;
}

.status-badge.skipped {
  background: #fef3c7;
  color: #92400e;
}
</style>
