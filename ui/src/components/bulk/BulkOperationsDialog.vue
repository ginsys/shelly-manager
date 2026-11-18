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
            <p>Read the current configuration of the selected devices into the manager</p>
          </button>

          <button class="operation-card" @click="operation = 'export'">
            <span class="operation-icon">📤</span>
            <h3>Bulk Export</h3>
            <p>Send the stored import/export configuration to the selected devices</p>
          </button>

          <button class="operation-card" @click="operation = 'drift'">
            <span class="operation-icon">🔍</span>
            <h3>Drift Detection</h3>
            <p>Compare the stored and the live configuration</p>
          </button>

          <button class="operation-card" @click="operation = 'drift-enhanced'">
            <span class="operation-icon">🔬</span>
            <h3>Drift Report</h3>
            <p>The same comparison, with a comprehensive report</p>
          </button>
        </div>
      </div>

      <!-- Import Form -->
      <div v-else-if="operation === 'import' && !isProcessing && !result">
        <h3>Bulk Import Configuration</h3>
        <form @submit.prevent="handleImport">
          <p class="form-note">
            Reads the current configuration of each selected device into the manager. Nothing is
            written to the devices.
          </p>

          <div class="form-field checkbox">
            <input
              id="stop-on-error"
              v-model="stopOnError"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="stop-on-error">Stop on first error</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary">Import</button>
          </div>
        </form>
      </div>

      <!-- Export Form -->
      <div v-else-if="operation === 'export' && !isProcessing && !result">
        <h3>Bulk Export Configuration</h3>
        <form @submit.prevent="handleExport">
          <p class="form-note">
            Writes the stored import/export configuration of each selected device to that device.
            A device that answers with another MAC than the stored one gets nothing and is
            reported as failed. This is not the desired configuration, and it does not change the
            applied state.
          </p>

          <div class="form-field checkbox">
            <input
              id="export-stop-on-error"
              v-model="stopOnError"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="export-stop-on-error">Stop on first error</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary">Export</button>
          </div>
        </form>
      </div>

      <!-- Drift Detection Form -->
      <div v-else-if="operation === 'drift' && !isProcessing && !result">
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

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary">Detect Drift</button>
          </div>
        </form>
      </div>

      <!-- Enhanced Drift Detection Form -->
      <div v-else-if="operation === 'drift-enhanced' && !isProcessing && !result">
        <h3>Drift Report</h3>
        <form @submit.prevent="handleDriftDetectEnhanced">
          <p class="form-note">
            The same comparison as drift detection, answered with the comprehensive report. With
            "stop on first error" the devices after the first error are not contacted and are not
            in the report.
          </p>

          <div class="form-field checkbox">
            <input
              id="enh-stop-on-error"
              v-model="stopOnError"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="enh-stop-on-error">Stop on first error</label>
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
        <p class="progress-text">
          Contacting {{ selectedDeviceIds.length }} device(s). The answer arrives when every
          device has been tried.
        </p>
      </div>

      <!-- Results -->
      <div v-if="view" class="results">
        <div class="results-summary">
          <h3>Results</h3>
          <div class="summary-stats">
            <div v-for="stat in view.stats" :key="stat.label" class="stat" :class="stat.tone">
              <span class="stat-label">{{ stat.label }}</span>
              <span class="stat-value">{{ stat.value }}</span>
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
              <tr v-for="row in view.rows" :key="row.deviceId" :class="row.tone">
                <td>{{ row.deviceName || `Device ${row.deviceId}` }}</td>
                <td>
                  <span class="status-badge" :class="row.tone">
                    {{ row.status }}
                  </span>
                </td>
                <td>
                  {{ row.message || '-' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="form-actions">
          <button class="btn" @click="close">Close</button>
          <button class="btn" @click="downloadResults">Download Results</button>
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
  viewOf,
  type BulkAnswer,
  type BulkRequest
} from '@/api/bulk'

interface Props {
  isOpen: boolean
  selectedDeviceIds: number[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  complete: [result: BulkAnswer]
}>()

type Operation = 'import' | 'export' | 'drift' | 'drift-enhanced'

const operation = ref<Operation | null>(null)
const isProcessing = ref(false)
const result = ref<BulkAnswer | null>(null)
const view = computed(() => (result.value ? viewOf(result.value) : null))

// The one option the routes read: after the first failed device the rest are skipped.
const stopOnError = ref(false)

function close() {
  operation.value = null
  isProcessing.value = false
  result.value = null
  emit('close')
}

async function run(call: (request: BulkRequest) => Promise<BulkAnswer>, failure: string) {
  try {
    isProcessing.value = true
    result.value = await call({
      device_ids: props.selectedDeviceIds,
      options: { stop_on_error: stopOnError.value }
    })
    isProcessing.value = false
    emit('complete', result.value)
  } catch (e: any) {
    alert(e?.message || failure)
    isProcessing.value = false
  }
}

const handleImport = () => run(bulkImport, 'Bulk import failed')
const handleExport = () => run(bulkExport, 'Bulk export failed')
const handleDriftDetect = () => run(bulkDriftDetect, 'Bulk drift detection failed')
const handleDriftDetectEnhanced = () => run(bulkDriftDetectEnhanced, 'Drift report failed')

function downloadResults() {
  if (!result.value) return

  const data = JSON.stringify(result.value, null, 2)
  const blob = new Blob([data], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `bulk-${operation.value ?? 'operation'}-result.json`
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

.form-note {
  margin: 0 0 16px 0;
  font-size: 14px;
  color: #4b5563;
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
  background: #f3f4f6;
}

.stat.warning {
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

.results-table tbody tr.failure {
  background: #fef2f2;
}

.results-table tbody tr.warning {
  background: #fffbeb;
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

.status-badge.failure {
  background: #fee2e2;
  color: #991b1b;
}

.status-badge.skipped {
  background: #f3f4f6;
  color: #4b5563;
}

.status-badge.warning {
  background: #fef3c7;
  color: #92400e;
}
</style>
