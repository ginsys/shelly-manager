<template>
  <div v-if="isOpen" class="modal" @click.self="close">
    <div class="modal-content">
      <div class="modal-header">
        <h2>Bulk Operations</h2>
        <button class="btn-close" data-testid="bulk-close" @click="close">×</button>
      </div>

      <ErrorDisplay
        :error="error"
        data-testid="bulk-error"
        @dismiss="clearError"
      />

      <!-- Operation Selection -->
      <div v-if="!operation && !isProcessing" class="operation-selection">
        <p class="selection-info">
          <strong>{{ selectedDevices.length }}</strong> device(s) selected
        </p>
        <p v-if="checking" class="form-note" role="status" data-testid="bulk-checking">
          Checking {{ selectedDevices.length }} {{ selectedDevices.length === 1 ? 'device' : 'devices' }}...
        </p>
        <p v-else-if="removedCount > 0" class="form-note" role="status" data-testid="bulk-removed">
          {{
            removedCount === 1
              ? '1 selected device no longer exists and was removed'
              : `${removedCount} selected devices no longer exist and were removed`
          }}
        </p>

        <p v-if="applyRunning" class="form-note" role="status" data-testid="bulk-apply-running">
          An earlier apply is still running. Closing the window did not stop it. Apply Configuration
          is off until it ends.
        </p>

        <ul class="picked-devices" data-testid="bulk-picked-devices">
          <li
            v-for="d in selectedDevices"
            :key="d.id"
            class="picked-device"
            data-testid="bulk-picked-device"
          >
            <span class="picked-name">{{ d.name || `Device ${d.id}` }}</span>
            <span class="picked-ip">{{ d.ip }}</span>
            <button
              type="button"
              class="picked-remove"
              :aria-label="`Remove ${d.name || d.ip} from the selection`"
              data-testid="bulk-remove-device"
              @click="emit('remove', d.id)"
            >
              ×
            </button>
          </li>
        </ul>
        <p v-if="selectedDevices.length === 0" class="form-note" data-testid="bulk-empty">
          No device is selected. Close this window and tick the devices to work on.
        </p>

        <div class="operation-cards">
          <button
            class="operation-card"
            :disabled="selectedDevices.length === 0 || checking"
            data-testid="bulk-op-import"
            @click="operation = 'import'"
          >
            <span class="operation-icon">📥</span>
            <h3>Bulk Import</h3>
            <p>Read the current configuration of the selected devices into the manager</p>
          </button>

          <button
            class="operation-card"
            :disabled="selectedDevices.length === 0 || checking"
            data-testid="bulk-op-drift"
            @click="operation = 'drift'"
          >
            <span class="operation-icon">🔍</span>
            <h3>Drift Detection</h3>
            <p>Compare the stored and the live configuration</p>
          </button>

          <button
            class="operation-card"
            :disabled="selectedDevices.length === 0 || checking"
            data-testid="bulk-op-drift-enhanced"
            @click="operation = 'drift-enhanced'"
          >
            <span class="operation-icon">🔬</span>
            <h3>Drift Report</h3>
            <p>The same comparison, with a comprehensive report</p>
          </button>

          <button
            class="operation-card"
            :disabled="selectedDevices.length === 0 || checking"
            data-testid="bulk-op-download"
            @click="operation = 'download'"
          >
            <span class="operation-icon">💾</span>
            <h3>Download Configuration</h3>
            <p>Save the desired configuration of the selected devices as one file</p>
          </button>

          <button
            class="operation-card"
            :disabled="selectedDevices.length === 0 || checking || applyRunning"
            data-testid="bulk-op-apply"
            @click="operation = 'apply'"
          >
            <span class="operation-icon">✍️</span>
            <h3>Apply Configuration</h3>
            <p>Write one configuration to the selected devices</p>
          </button>
        </div>
      </div>

      <!-- Import Form -->
      <div v-else-if="operation === 'import' && !isProcessing && !result">
        <h3>Bulk Import Configuration</h3>
        <form @submit.prevent="handleImport">
          <p class="form-note">
            Reads the current configuration of each selected device into the manager. Nothing is
            written to the devices; to write a configuration to them, use Apply Configuration.
          </p>

          <div class="form-field checkbox">
            <input
              id="stop-on-error"
              v-model="stopOnError"
              type="checkbox"
              class="form-checkbox"
              data-testid="bulk-stop-on-error"
            />
            <label for="stop-on-error">Stop on first error</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary" data-testid="bulk-submit">Import</button>
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
              data-testid="bulk-stop-on-error"
            />
            <label for="drift-stop-on-error">Stop on first error</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary" data-testid="bulk-submit">Detect Drift</button>
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
              data-testid="bulk-stop-on-error"
            />
            <label for="enh-stop-on-error">Stop on first error</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="operation = null">Back</button>
            <button type="submit" class="btn primary" data-testid="bulk-submit">Detect Drift</button>
          </div>
        </form>
      </div>

      <!-- Download Configuration Form -->
      <div v-else-if="operation === 'download' && !savedFile">
        <h3>Download Configuration</h3>
        <form @submit.prevent="handleDownload">
          <p class="form-note">
            Saves the desired configuration of each selected device (its templates and overrides
            merged) as one JSON file. It is read from the manager's database: no device is
            contacted. A device that cannot be read is listed with the reason and has no
            configuration in the file.
          </p>
          <p class="form-note" data-testid="bulk-download-notice">{{ maskNotice }}</p>

          <div class="form-actions">
            <button type="button" class="btn" :disabled="downloading" @click="operation = null">
              Back
            </button>
            <button
              type="submit"
              class="btn primary"
              :disabled="downloading"
              data-testid="bulk-download-submit"
            >
              {{ downloading ? 'Preparing the file...' : 'Download' }}
            </button>
          </div>
        </form>
      </div>

      <!-- Download Configuration Summary -->
      <div v-else-if="operation === 'download' && savedFile" data-testid="bulk-download-saved">
        <h3>Configuration downloaded</h3>
        <p class="form-note">
          Saved <strong>{{ savedFile.name }}</strong>: {{ savedFile.saved }} of
          {{ savedFile.total }} configurations.
        </p>
        <p class="form-note">{{ maskNotice }}</p>
        <div v-if="savedFile.failed.length > 0" data-testid="bulk-download-failed">
          <h4>Not in the file with a configuration</h4>
          <ul class="download-failed">
            <li v-for="d in savedFile.failed" :key="d.id">
              <strong>{{ d.name }}</strong
              >: {{ d.error }}
            </li>
          </ul>
        </div>
        <div class="form-actions">
          <button class="btn" data-testid="bulk-download-close" @click="close">Close</button>
        </div>
      </div>

      <!-- Processing State -->
      <div v-else-if="isProcessing" class="processing">
        <h3>Processing...</h3>
        <p class="progress-text">
          Contacting {{ contacting }} device(s). The answer arrives when every
          device has been tried.
        </p>
        <p v-if="operation === 'apply'" class="progress-text" data-testid="bulk-apply-processing">
          The devices are being written. Closing this window does not stop the call or undo what
          was written. A call can last up to 5 minutes.
        </p>
      </div>

      <!-- Apply Configuration Form. Outside the chain above, so it stays mounted while the call
           runs and a refused call returns to the confirmation with the text intact. -->
      <BulkApplyForm
        v-if="operation === 'apply'"
        v-show="!isProcessing && !result"
        ref="applyForm"
        v-model="stopOnError"
        :devices="selectedDevices"
        @back="operation = null"
        @submit="handleApply"
      />

      <!-- Results -->
      <div v-if="view" class="results" data-testid="bulk-results">
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
          <button class="btn" data-testid="bulk-results-close" @click="close">Close</button>
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
  bulkDriftDetect,
  bulkDriftDetectEnhanced,
  bulkDownload,
  viewOf,
  type BulkAnswer,
  type BulkRequest
} from '@/api/bulk'
import {
  bulkApply,
  applyViewOf,
  isApplyResult,
  NothingWrittenError,
  applyRunning,
  type BulkApplyResult
} from '@/api/bulkApply'
import { useError } from '@/composables/useError'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import BulkApplyForm from '@/components/bulk/BulkApplyForm.vue'
import type { SelectedDevice } from '@/stores/devices'

// What a run can end in: the answer of one of the read routes, or the one of bulk-apply.
type Answer = BulkAnswer | BulkApplyResult

interface Props {
  isOpen: boolean
  // The devices the operation runs on. The request carries exactly these ids and never
  // "every device": with none picked, nothing can be started.
  selectedDevices: SelectedDevice[]
  // The page is asking the server whether the ticked devices still exist: nothing is sent meanwhile.
  checking?: boolean
  // How many ticked devices the last check removed because the server no longer has them.
  removedCount?: number
}

const props = withDefaults(defineProps<Props>(), { checking: false, removedCount: 0 })
const emit = defineEmits<{
  close: []
  complete: [result: Answer]
  remove: [id: number]
}>()

const { error, setError, clearError } = useError()

type Operation = 'import' | 'drift' | 'drift-enhanced' | 'download' | 'apply'

const operation = ref<Operation | null>(null)
const isProcessing = ref(false)
const result = ref<Answer | null>(null)
const view = computed(() => {
  const answer = result.value
  if (!answer) return null
  return isApplyResult(answer) ? applyViewOf(answer) : viewOf(answer)
})

// The apply form holds what the operator typed, which may be a secret: it is emptied when an answer
// arrives and when the dialog closes.
const applyForm = ref<InstanceType<typeof BulkApplyForm> | null>(null)

// The one option the routes read: after the first failed device the rest are skipped.
const stopOnError = ref(false)

// The run whose answer may still be shown. Closing the dialog moves it on, so the answer of a
// request sent before the close never lands in a dialog that was reopened for other devices.
let currentRun = 0

function close() {
  currentRun++
  applyForm.value?.reset()
  operation.value = null
  isProcessing.value = false
  downloading.value = false
  savedFile.value = null
  result.value = null
  clearError()
  emit('close')
}

// How many devices the running call names: the live selection can change while it runs.
const contacting = ref(0)

// `ids` are the devices to name when they are not the live selection: apply names the ones the
// operator confirmed.
async function run(
  call: (request: BulkRequest) => Promise<Answer>,
  action: string,
  ids: number[] = props.selectedDevices.map((d) => d.id)
) {
  if (ids.length === 0) return
  clearError()
  const mine = ++currentRun
  try {
    contacting.value = ids.length
    isProcessing.value = true
    const answer = await call({
      device_ids: ids,
      options: { stop_on_error: stopOnError.value }
    })
    // The devices were contacted either way, so the page still refreshes its list.
    if (mine !== currentRun) {
      emit('complete', answer)
      return
    }
    result.value = answer
    isProcessing.value = false
    applyForm.value?.reset()
    emit('complete', answer)
  } catch (e: unknown) {
    if (mine !== currentRun) return
    isProcessing.value = false
    // After an answer that may follow writes, what was typed is forgotten, so the call cannot be sent
    // again by a click on the same confirmation. After a refusal before any write it stays to correct.
    if (!(e instanceof NothingWrittenError)) applyForm.value?.reset()
    setError(e, { action, resource: 'Devices' })
  }
}

const handleImport = () => run(bulkImport, 'Bulk import')
const handleDriftDetect = () => run(bulkDriftDetect, 'Bulk drift detection')
const handleDriftDetectEnhanced = () => run(bulkDriftDetectEnhanced, 'Drift report')
// The devices are the ones the operator confirmed in the form, which can differ from the live
// selection if the page removed a device meanwhile: a device that is gone is answered by the route.
async function handleApply(p: { deviceIds: number[]; config: Record<string, unknown> }) {
  // A second apply waits for the first, even one started by a dialog that was closed or unmounted.
  if (applyRunning.value) return
  applyRunning.value = true
  try {
    await run(
      (request) => bulkApply({ ...request, config: p.config }),
      'Bulk apply',
      p.deviceIds
    )
  } finally {
    applyRunning.value = false
  }
}

function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

function downloadResults() {
  if (!result.value) return

  const data = JSON.stringify(result.value, null, 2)
  saveBlob(new Blob([data], { type: 'application/json' }), `bulk-${operation.value ?? 'operation'}-result.json`)
}

// Download configuration: the desired configuration of the picked devices as one file. The request has no
// options (the route refuses `stop_on_error`), and no device is contacted, so the page's list is not
// refreshed. The file holds every secret masked, so it cannot restore a device; the dialog says so and
// never shows a configuration itself.
const maskNotice =
  'Every secret in the file is masked as ******** and the real values are never included, so the file cannot restore a device.'

const downloading = ref(false)
const savedFile = ref<{
  name: string
  saved: number
  total: number
  failed: { id: number; name: string; error: string }[]
} | null>(null)

async function handleDownload() {
  if (props.selectedDevices.length === 0) return
  clearError()
  const mine = ++currentRun
  try {
    downloading.value = true
    const { blob, filename, file } = await bulkDownload({
      device_ids: props.selectedDevices.map((d) => d.id)
    })
    // Closed meanwhile: nothing is saved for a dialog the user left.
    if (mine !== currentRun) return
    const name = filename || 'shelly-configs.json'
    saveBlob(blob, name)
    savedFile.value = {
      name,
      saved: file.devices.filter((d) => d.status === 'ok').length,
      total: file.devices.length,
      failed: file.devices
        .filter((d) => d.status !== 'ok')
        .map((d) => ({
          id: d.device_id,
          name: d.device_name || `Device ${d.device_id}`,
          error: d.error || 'the configuration could not be read'
        }))
    }
  } catch (e: unknown) {
    if (mine !== currentRun) return
    setError(e, { action: 'Bulk download', resource: 'Devices' })
  } finally {
    if (mine === currentRun) downloading.value = false
  }
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

.picked-devices {
  list-style: none;
  margin: 0 0 20px 0;
  padding: 0;
  max-height: 180px;
  overflow-y: auto;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.picked-devices:empty {
  display: none;
}

.picked-device {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 12px;
  border-bottom: 1px solid #f3f4f6;
  font-size: 13px;
}

.picked-device:last-child {
  border-bottom: none;
}

.picked-name {
  flex: 1;
  font-weight: 500;
}

.picked-ip {
  color: #6b7280;
  font-family: monospace;
}

.picked-remove {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 18px;
  line-height: 1;
  color: #6b7280;
}

.picked-remove:hover {
  color: #991b1b;
}

.operation-card:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
