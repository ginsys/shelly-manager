<template>
  <div class="page">
    <div class="page-header">
      <h1>Drift Detection Schedules</h1>
      <button class="primary-button" data-testid="new-schedule" @click="openForm(null)">New schedule</button>
    </div>

    <!-- No banner while the server runs the scheduler. -->
    <div v-if="store.schedulerEnabled === false" class="notice" role="status" data-testid="scheduler-banner">
      <strong>The drift scheduler is off on this server.</strong>
      <p>
        Schedules are stored and shown, and none runs
        (<code>drift.scheduler_enabled</code> is not set).
      </p>
    </div>
    <div v-else-if="statusFailed" class="notice" role="status" data-testid="scheduler-banner">
      <strong>The scheduler state could not be read.</strong>
      <p>Whether the stored schedules run is unknown.</p>
    </div>

    <ErrorDisplay
      :error="actionError"
      :show-retry="false"
      data-testid="schedule-action-error"
      @dismiss="clearActionError"
    />

    <div v-if="store.loading" class="loading">Loading schedules...</div>
    <div v-else-if="store.error" class="error">{{ store.error }}</div>

    <div v-else class="content">
      <div v-if="store.schedules.length === 0" class="empty">
        <p>No drift detection schedules stored.</p>
      </div>

      <div v-else class="schedules-list">
        <div
          v-for="schedule in store.schedules"
          :key="schedule.id"
          class="schedule-card"
        >
          <div class="schedule-header">
            <div class="schedule-info">
              <h3>{{ schedule.name }}</h3>
              <p v-if="schedule.description" class="description">{{ schedule.description }}</p>
            </div>
            <div class="schedule-status">
              <span class="status-badge">{{ badge(schedule) }}</span>
            </div>
          </div>

          <div class="schedule-details">
            <div class="detail-item">
              <span class="label">Cron Schedule</span>
              <span class="value"><code>{{ schedule.cron_spec }}</code></span>
            </div>
            <div v-if="schedule.device_ids && schedule.device_ids.length > 0" class="detail-item">
              <span class="label">Devices</span>
              <span class="value">{{ schedule.device_ids.length }} device(s)</span>
            </div>
            <div v-if="hasFilter(schedule.device_filter)" class="detail-item">
              <span class="label">Device Filter (stored JSON)</span>
              <pre class="value filter-json">{{ formatFilter(schedule.device_filter) }}</pre>
            </div>
          </div>

          <div class="stored-meta">
            <span class="stored-meta-item">Run count: {{ schedule.run_count }}</span>
            <span v-if="schedule.last_run" class="stored-meta-item">Last run: {{ formatDate(schedule.last_run) }}</span>
            <span v-if="schedule.next_run" class="stored-meta-item">Next run: {{ formatDate(schedule.next_run) }}</span>
          </div>

          <div class="schedule-actions">
            <button
              class="btn-icon"
              :aria-expanded="runsOf(schedule.id).open"
              :data-testid="`runs-toggle-${schedule.id}`"
              @click="toggleRuns(schedule.id)"
            >
              Recent runs
            </button>
            <button class="btn-icon" :data-testid="`edit-${schedule.id}`" @click="openForm(schedule)">Edit</button>
            <button class="btn-icon" :data-testid="`toggle-${schedule.id}`" @click="handleToggle(schedule.id)">
              {{ schedule.enabled ? 'Disable' : 'Enable' }}
            </button>
            <button class="btn-icon danger" title="Delete" @click="handleDelete(schedule.id)">
              🗑️ Delete
            </button>
          </div>

          <div v-if="runsOf(schedule.id).open" class="runs" :data-testid="`runs-${schedule.id}`">
            <p v-if="runsOf(schedule.id).loading" class="hint">Loading runs...</p>
            <div v-else-if="runsOf(schedule.id).error" class="runs-error" role="alert">
              <span>{{ runsOf(schedule.id).error }}</span>
              <button class="btn-icon" :data-testid="`runs-retry-${schedule.id}`" @click="loadRuns(schedule.id)">Retry</button>
            </div>
            <p v-else-if="runsOf(schedule.id).runs.length === 0" class="hint">No runs yet</p>
            <table v-else class="runs-table">
              <thead>
                <tr><th>Started</th><th>Status</th><th>Duration</th><th>Error</th></tr>
              </thead>
              <tbody>
                <tr v-for="run in runsOf(schedule.id).runs" :key="run.id" class="run-row">
                  <td>{{ formatDate(run.started_at) }}</td>
                  <td>{{ run.status }}</td>
                  <td>{{ formatDuration(run.duration) }}</td>
                  <td>{{ run.error || '' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <ScheduleForm
      v-if="formOpen"
      :schedule="editing"
      :devices="devices"
      :devices-error="devicesError"
      @saved="formOpen = false"
      @cancel="formOpen = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useDriftStore } from '@/stores/drift'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import ScheduleForm from '@/components/drift/ScheduleForm.vue'
import { usePageError } from '@/composables/useNotice'
import { listDevices } from '@/api/devices'
import type { DriftSchedule, DriftRun } from '@/api/drift'
import type { Device } from '@/api/types'

const store = useDriftStore()
// The failure of a delete or a toggle, above the list. The store keeps no action failure
// (ginsys/shelly-manager#701).
const { error: actionError, setError: setActionError, clearError: clearActionError } = usePageError()

// Set when the read of the scheduler state failed; the store keeps null for that and for "not read yet".
const statusFailed = ref(false)

// A disabled schedule never runs; an enabled one runs only where the server runs the scheduler.
function badge(schedule: DriftSchedule): string {
  if (!schedule.enabled) return 'Disabled'
  return store.schedulerEnabled === false ? 'Scheduler off' : 'Enabled'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

// device_filter is stored as a raw backend JSON value; render it read-only.
function hasFilter(filter: unknown): boolean {
  if (filter === null || filter === undefined) return false
  if (typeof filter === 'string') return filter.trim() !== '' && filter.trim() !== 'null'
  return true
}

function formatFilter(filter: unknown): string {
  if (typeof filter === 'string') {
    try {
      return JSON.stringify(JSON.parse(filter), null, 2)
    } catch {
      return filter
    }
  }
  return JSON.stringify(filter, null, 2)
}

async function handleDelete(id: number) {
  if (!confirm('Delete this schedule and its run history?')) {
    return
  }
  clearActionError()
  try {
    await store.remove(id)
  } catch (e) {
    setActionError(e, { action: 'Delete drift schedule', resource: 'Drift schedule', resourceId: String(id) })
  }
}

async function handleToggle(id: number) {
  clearActionError()
  try {
    await store.toggle(id)
  } catch (e) {
    setActionError(e, { action: 'Toggle drift schedule', resource: 'Drift schedule', resourceId: String(id) })
  }
}

// The form: the devices it offers are every device, read each time it opens.
const formOpen = ref(false)
const editing = ref<DriftSchedule | null>(null)
const devices = ref<Device[]>([])
const devicesError = ref('')

async function openForm(schedule: DriftSchedule | null) {
  clearActionError()
  devicesError.value = ''
  try {
    devices.value = (await listDevices({ all: true })).items
  } catch (e: any) {
    devices.value = []
    devicesError.value = `Devices could not be loaded: ${e?.message || 'unknown error'}`
  }
  editing.value = schedule
  formOpen.value = true
}

// Recent runs of one card, read when the card is first expanded and again on a retry.
interface RunsState {
  open: boolean
  loading: boolean
  error: string
  runs: DriftRun[]
}
const runStates = reactive<Record<number, RunsState>>({})

function runsOf(id: number): RunsState {
  return runStates[id] ?? { open: false, loading: false, error: '', runs: [] }
}

async function loadRuns(id: number) {
  const state = (runStates[id] ??= { open: true, loading: false, error: '', runs: [] })
  state.loading = true
  state.error = ''
  try {
    state.runs = await store.fetchRuns(id)
  } catch (e: any) {
    state.error = `Runs could not be loaded: ${e?.message || 'unknown error'}`
  } finally {
    state.loading = false
  }
}

async function toggleRuns(id: number) {
  const state = runStates[id]
  if (state) {
    state.open = !state.open
    if (!state.open || state.runs.length > 0 || state.loading) return
  } else {
    runStates[id] = { open: true, loading: false, error: '', runs: [] }
  }
  await loadRuns(id)
}

// A run's duration comes in nanoseconds.
function formatDuration(ns: number | null | undefined): string {
  if (ns === null || ns === undefined) return ''
  const ms = ns / 1e6
  if (ms < 1000) return `${Math.round(ms)} ms`
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)} s`
  return `${Math.floor(s / 60)} min ${Math.round(s % 60)} s`
}

onMounted(async () => {
  store.fetchSchedules()
  try {
    await store.fetchSchedulerStatus()
  } catch {
    statusFailed.value = true
  }
})
</script>

<style scoped>
.page {
  padding: 20px;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.page-header h1 {
  margin: 0;
  font-size: 24px;
  color: #1f2937;
}

.notice {
  margin-bottom: 24px;
  padding: 16px 20px;
  background: #fef9c3;
  border: 1px solid #eab308;
  border-radius: 8px;
  color: #713f12;
}

.notice strong {
  display: block;
  font-size: 15px;
  margin-bottom: 6px;
}

.notice p {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
}

.loading,
.error,
.empty {
  padding: 32px;
  text-align: center;
  color: #64748b;
}

.error {
  color: #b91c1c;
  background: #fee2e2;
  border-radius: 8px;
}

.empty p {
  margin: 8px 0;
}

.schedules-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.schedule-card {
  padding: 20px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.schedule-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 16px;
}

.schedule-info h3 {
  margin: 0 0 4px 0;
  font-size: 18px;
  color: #1f2937;
}

.description {
  margin: 0;
  font-size: 14px;
  color: #64748b;
}

.status-badge {
  padding: 4px 12px;
  background: #e2e8f0;
  color: #475569;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.schedule-details {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e5e7eb;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-item .label {
  font-size: 12px;
  font-weight: 600;
  color: #6b7280;
  text-transform: uppercase;
}

.detail-item .value {
  font-size: 14px;
  color: #1f2937;
}

.filter-json {
  margin: 0;
  padding: 8px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
}

.stored-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  margin-bottom: 16px;
  font-size: 12px;
  color: #94a3b8;
}

.schedule-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.btn-icon {
  padding: 6px 12px;
  border: 1px solid #cbd5e1;
  background: #fff;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
}

.btn-icon.danger {
  border-color: #fca5a5;
  color: #dc2626;
}

.btn-icon.danger:hover {
  background: #fee2e2;
}

.primary-button {
  padding: 8px 16px;
  background: #2563eb;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}

.primary-button:hover {
  background: #1d4ed8;
}

.runs {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid #e5e7eb;
}

.hint {
  margin: 0;
  font-size: 13px;
  color: #64748b;
}

.runs-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  background: #fee2e2;
  color: #b91c1c;
  border-radius: 6px;
  font-size: 14px;
}

.runs-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.runs-table th,
.runs-table td {
  padding: 6px 8px;
  text-align: left;
  border-bottom: 1px solid #f1f5f9;
}

.runs-table th {
  color: #6b7280;
  font-size: 12px;
  text-transform: uppercase;
}
</style>
