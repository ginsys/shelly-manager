<template>
  <div class="page">
    <div class="page-header">
      <h1>Drift Detection Schedules</h1>
      <button class="btn primary" @click="showCreateDialog = true">Create Schedule</button>
    </div>

    <div v-if="store.loading" class="loading">Loading schedules...</div>
    <div v-else-if="store.error" class="error">{{ store.error }}</div>

    <div v-else class="content">
      <div v-if="store.schedules.length === 0" class="empty">
        <p>No drift detection schedules configured.</p>
        <p>Click "Create Schedule" to set up automated drift detection.</p>
      </div>

      <div v-else class="schedules-list">
        <div
          v-for="schedule in store.schedules"
          :key="schedule.id"
          class="schedule-card"
          :class="{ disabled: !schedule.enabled }"
        >
          <div class="schedule-header">
            <div class="schedule-info">
              <h3>{{ schedule.name }}</h3>
              <p v-if="schedule.description" class="description">{{ schedule.description }}</p>
            </div>
            <div class="schedule-status">
              <span class="status-badge" :class="{ enabled: schedule.enabled }">
                {{ schedule.enabled ? 'Enabled' : 'Disabled' }}
              </span>
            </div>
          </div>

          <div class="schedule-details">
            <div class="detail-item">
              <span class="label">Check Interval:</span>
              <span class="value">{{ schedule.checkInterval }}</span>
            </div>
            <div v-if="schedule.deviceIds && schedule.deviceIds.length > 0" class="detail-item">
              <span class="label">Devices:</span>
              <span class="value">{{ schedule.deviceIds.length }} device(s)</span>
            </div>
            <div v-if="schedule.deviceFilter" class="detail-item">
              <span class="label">Device Filter:</span>
              <span class="value">{{ schedule.deviceFilter }}</span>
            </div>
            <div v-if="schedule.lastRun" class="detail-item">
              <span class="label">Last Run:</span>
              <span class="value">{{ formatDate(schedule.lastRun) }}</span>
            </div>
            <div v-if="schedule.nextRun" class="detail-item">
              <span class="label">Next Run:</span>
              <span class="value">{{ formatDate(schedule.nextRun) }}</span>
            </div>
          </div>

          <div class="schedule-actions">
            <button class="btn-icon" title="Toggle" @click="handleToggle(schedule.id)">
              {{ schedule.enabled ? '⏸' : '▶' }}
            </button>
            <button class="btn-icon" title="View Runs" @click="viewRuns(schedule.id)">📊</button>
            <button class="btn-icon" title="Edit" @click="editSchedule(schedule)">✏️</button>
            <button class="btn-icon danger" title="Delete" @click="handleDelete(schedule.id)">
              🗑️
            </button>
          </div>
        </div>
      </div>

      <div v-if="store.scheduleMeta && store.scheduleMeta.total > pageSize" class="pagination">
        <button
          class="btn"
          :disabled="page === 1"
          @click="handlePageChange(page - 1)"
        >
          Previous
        </button>
        <span class="page-info">
          Page {{ page }} of {{ Math.ceil(store.scheduleMeta.total / pageSize) }}
          ({{ store.scheduleMeta.total }} total)
        </span>
        <button
          class="btn"
          :disabled="page >= Math.ceil(store.scheduleMeta.total / pageSize)"
          @click="handlePageChange(page + 1)"
        >
          Next
        </button>
      </div>
    </div>

    <!-- Create/Edit Dialog -->
    <div v-if="showCreateDialog || editingSchedule" class="modal" @click.self="closeDialog">
      <div class="modal-content">
        <div class="modal-header">
          <h2>{{ editingSchedule ? 'Edit Schedule' : 'Create Schedule' }}</h2>
          <button class="btn-close" @click="closeDialog">×</button>
        </div>
        <form @submit.prevent="handleSubmit">
          <div class="form-field">
            <label for="name">Name *</label>
            <input
              id="name"
              v-model="formData.name"
              type="text"
              required
              class="form-input"
            />
          </div>

          <div class="form-field">
            <label for="description">Description</label>
            <textarea
              id="description"
              v-model="formData.description"
              class="form-textarea"
              rows="2"
            />
          </div>

          <div class="form-field">
            <label for="checkInterval">Check Interval *</label>
            <input
              id="checkInterval"
              v-model="formData.checkInterval"
              type="text"
              required
              class="form-input"
              placeholder="e.g., 1h, 24h, 30m"
            />
            <small>Examples: 1h, 24h, 30m, 7d</small>
          </div>

          <div class="form-field">
            <label for="deviceIds">Device IDs (comma-separated)</label>
            <input
              id="deviceIds"
              v-model="deviceIdsInput"
              type="text"
              class="form-input"
              placeholder="e.g., 1,2,3"
            />
          </div>

          <div class="form-field">
            <label for="deviceFilter">Device Filter Expression</label>
            <input
              id="deviceFilter"
              v-model="formData.deviceFilter"
              type="text"
              class="form-input"
              placeholder="e.g., type:shelly1"
            />
          </div>

          <div class="form-field checkbox">
            <input
              id="enabled"
              v-model="formData.enabled"
              type="checkbox"
              class="form-checkbox"
            />
            <label for="enabled">Enabled</label>
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="closeDialog">Cancel</button>
            <button type="submit" class="btn primary">
              {{ editingSchedule ? 'Update' : 'Create' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useDriftStore } from '@/stores/drift'
import type { DriftSchedule, CreateDriftScheduleRequest } from '@/api/drift'

const router = useRouter()
const store = useDriftStore()

const page = ref(1)
const pageSize = ref(25)
const showCreateDialog = ref(false)
const editingSchedule = ref<DriftSchedule | null>(null)
const deviceIdsInput = ref('')

const formData = ref<CreateDriftScheduleRequest>({
  name: '',
  description: '',
  checkInterval: '24h',
  deviceIds: [],
  deviceFilter: '',
  enabled: true
})

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function resetForm() {
  formData.value = {
    name: '',
    description: '',
    checkInterval: '24h',
    deviceIds: [],
    deviceFilter: '',
    enabled: true
  }
  deviceIdsInput.value = ''
}

function editSchedule(schedule: DriftSchedule) {
  editingSchedule.value = schedule
  formData.value = {
    name: schedule.name,
    description: schedule.description,
    checkInterval: schedule.checkInterval,
    deviceIds: schedule.deviceIds,
    deviceFilter: schedule.deviceFilter,
    enabled: schedule.enabled
  }
  deviceIdsInput.value = schedule.deviceIds?.join(',') || ''
}

function closeDialog() {
  showCreateDialog.value = false
  editingSchedule.value = null
  resetForm()
}

async function handleSubmit() {
  try {
    // Parse device IDs
    if (deviceIdsInput.value.trim()) {
      formData.value.deviceIds = deviceIdsInput.value
        .split(',')
        .map(id => parseInt(id.trim()))
        .filter(id => !isNaN(id))
    } else {
      formData.value.deviceIds = undefined
    }

    if (editingSchedule.value) {
      await store.update(editingSchedule.value.id, formData.value)
    } else {
      await store.create(formData.value)
    }
    closeDialog()
    await store.fetchSchedules(page.value, pageSize.value)
  } catch (e: any) {
    alert(e?.message || 'Failed to save schedule')
  }
}

async function handleToggle(id: number) {
  try {
    await store.toggle(id)
  } catch (e: any) {
    alert(e?.message || 'Failed to toggle schedule')
  }
}

async function handleDelete(id: number) {
  if (!confirm('Are you sure you want to delete this schedule?')) {
    return
  }
  try {
    await store.remove(id)
  } catch (e: any) {
    alert(e?.message || 'Failed to delete schedule')
  }
}

function viewRuns(scheduleId: number) {
  router.push(`/drift/schedules/${scheduleId}/runs`)
}

async function handlePageChange(newPage: number) {
  page.value = newPage
  await store.fetchSchedules(page.value, pageSize.value)
}

onMounted(() => {
  store.fetchSchedules(page.value, pageSize.value)
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
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0;
  font-size: 24px;
  color: #1f2937;
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
  transition: box-shadow 0.2s;
}

.schedule-card:hover {
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}

.schedule-card.disabled {
  opacity: 0.6;
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
  background: #fee2e2;
  color: #991b1b;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.status-badge.enabled {
  background: #dcfce7;
  color: #166534;
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
  font-size: 16px;
  transition: background 0.2s;
}

.btn-icon:hover {
  background: #f8fafc;
}

.btn-icon.danger {
  border-color: #fca5a5;
  color: #dc2626;
}

.btn-icon.danger:hover {
  background: #fee2e2;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  margin-top: 24px;
}

.page-info {
  font-size: 14px;
  color: #64748b;
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

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn.primary {
  background: #2563eb;
  color: white;
  border-color: #2563eb;
}

.btn.primary:hover {
  background: #1d4ed8;
}

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
  max-width: 600px;
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

.form-field small {
  font-size: 12px;
  color: #6b7280;
}

.form-input,
.form-textarea {
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-family: inherit;
  font-size: 14px;
}

.form-textarea {
  resize: vertical;
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
</style>
