<template>
  <main style="padding:16px">
    <div class="page-header">
      <h1>Export Schedules</h1>
      <button class="primary-button" @click="showCreateForm = true">
        ➕ Create Schedule
      </button>
    </div>

    <!-- Schedule Statistics -->
    <section class="stats-section">
      <div class="stats">
        <div class="card">
          <span class="stat-label">Total:</span> 
          <span class="stat-value">{{ store.stats.total }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Enabled:</span> 
          <span class="stat-value enabled">{{ store.stats.enabled }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Disabled:</span> 
          <span class="stat-value disabled">{{ store.stats.disabled }}</span>
        </div>
      </div>
    </section>

    <!-- Filters -->
    <ScheduleFilterBar
      :plugin="store.plugin"
      :enabled="store.enabled"
      @update:plugin="(v: string) => { store.setPlugin(v); store.fetchSchedules() }"
      @update:enabled="(v: boolean | undefined) => { store.setEnabled(v); store.fetchSchedules() }"
    />

    <!-- Schedules Table -->
    <DataTable
      :rows="store.schedulesSorted"
      :loading="store.loading"
      :error="store.error"
      :cols="8"
      :rowKey="(row: any) => row.id"
    >
      <template #header>
        <th>Name</th>
        <th>Plugin</th>
        <th>Format</th>
        <th>Interval</th>
        <th>Status</th>
        <th>Last Run</th>
        <th>Next Run</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>
          <div class="schedule-name">
            <strong>{{ row.name }}</strong>
            <div class="schedule-id">ID: {{ row.id }}</div>
          </div>
        </td>
        <td>{{ row.request.plugin_name }}</td>
        <td>{{ row.request.format.toUpperCase() }}</td>
        <td>{{ formatInterval(row.interval_sec) }}</td>
        <td>
          <span :class="['status-badge', row.enabled ? 'enabled' : 'disabled']">
            {{ row.enabled ? 'Enabled' : 'Disabled' }}
          </span>
        </td>
        <td>
          <div v-if="row.last_run" class="time-info">
            {{ new Date(row.last_run).toLocaleString() }}
            <div v-if="store.getRecentRun(row.id)" class="recent-result">
              <span :class="store.getRecentRun(row.id)?.record_count ? 'success' : 'warning'">
                {{ store.getRecentRun(row.id)?.record_count || 0 }} records
              </span>
            </div>
          </div>
          <span v-else class="no-data">Never</span>
        </td>
        <td>
          <div v-if="row.enabled && row.next_run" class="time-info">
            {{ new Date(row.next_run).toLocaleString() }}
            <div class="time-remaining">
              {{ formatTimeRemaining(row.next_run) }}
            </div>
          </div>
          <span v-else class="no-data">—</span>
        </td>
        <td>
          <div class="action-buttons">
            <button 
              v-if="row.enabled" 
              class="action-btn run-btn" 
              :disabled="store.isScheduleRunning(row.id)"
              @click="runSchedule(row.id)"
              title="Run now"
            >
              <span v-if="store.isScheduleRunning(row.id)">⏳</span>
              <span v-else>▶️</span>
            </button>
            <button 
              class="action-btn toggle-btn" 
              @click="toggleSchedule(row.id)"
              :title="row.enabled ? 'Disable' : 'Enable'"
            >
              {{ row.enabled ? '⏸️' : '▶️' }}
            </button>
            <button 
              class="action-btn edit-btn" 
              @click="editSchedule(row)"
              title="Edit"
            >
              ✏️
            </button>
            <button 
              class="action-btn delete-btn" 
              @click="confirmDelete(row)"
              title="Delete"
            >
              🗑️
            </button>
          </div>
        </td>
      </template>
    </DataTable>

    <!-- Pagination -->
    <PaginationBar
      v-if="store.meta?.pagination"
      :page="store.meta.pagination.page"
      :totalPages="store.meta.pagination.total_pages"
      :hasNext="store.meta.pagination.has_next"
      :hasPrev="store.meta.pagination.has_previous"
      @update:page="(p: number) => { store.setPage(p); store.fetchSchedules() }"
    />

    <!-- Create/Edit Form Modal -->
    <div v-if="showCreateForm || editingSchedule" class="modal-overlay" @click="closeModal">
      <div class="modal-content" @click.stop>
        <ScheduleForm
          :schedule="editingSchedule"
          :loading="store.currentLoading"
          :error="store.currentError"
          @submit="handleFormSubmit"
          @cancel="closeModal"
        />
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <div v-if="deleteConfirm" class="modal-overlay" @click="deleteConfirm = null">
      <div class="modal-content confirm-modal" @click.stop>
        <h3>Confirm Delete</h3>
        <p>Are you sure you want to delete schedule <strong>{{ deleteConfirm.name }}</strong>?</p>
        <p class="warning">This action cannot be undone.</p>
        <div class="modal-actions">
          <button class="secondary-button" @click="deleteConfirm = null">Cancel</button>
          <button class="danger-button" @click="performDelete">Delete Schedule</button>
        </div>
      </div>
    </div>

    <!-- Success/Error Messages -->
    <div v-if="message.text" :class="['message', message.type]">
      {{ message.text }}
      <button class="message-close" @click="message.text = ''">✖</button>
    </div>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref, reactive } from 'vue'
import { useScheduleStore } from '@/stores/schedule'
import { formatInterval } from '@/api/schedule'
import type { ExportSchedule, ExportScheduleRequest } from '@/api/schedule'
import DataTable from '@/components/DataTable.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import ScheduleFilterBar from '@/components/ScheduleFilterBar.vue'
import ScheduleForm from '@/components/ScheduleForm.vue'

const store = useScheduleStore()

// UI state
const showCreateForm = ref(false)
const editingSchedule = ref<ExportSchedule | null>(null)
const deleteConfirm = ref<ExportSchedule | null>(null)
const message = reactive({ 
  text: '', 
  type: 'success' as 'success' | 'error' 
})

// Initialize
onMounted(() => {
  store.fetchSchedules()
})

/**
 * Format time remaining until next run
 */
function formatTimeRemaining(nextRun: string): string {
  const now = new Date()
  const next = new Date(nextRun)
  const diff = next.getTime() - now.getTime()
  
  if (diff <= 0) return 'Overdue'
  
  const minutes = Math.floor(diff / (1000 * 60))
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)
  
  if (days > 0) return `in ${days} day${days !== 1 ? 's' : ''}`
  if (hours > 0) return `in ${hours} hour${hours !== 1 ? 's' : ''}`
  if (minutes > 0) return `in ${minutes} min`
  return 'very soon'
}

/**
 * Run a schedule immediately
 */
async function runSchedule(id: string) {
  try {
    const result = await store.runScheduleNow(id)
    message.text = `Schedule ran successfully. ${result.record_count || 0} records exported.`
    message.type = 'success'
  } catch (error: any) {
    message.text = error.message || 'Failed to run schedule'
    message.type = 'error'
  }
}

/**
 * Toggle schedule enabled/disabled
 */
async function toggleSchedule(id: string) {
  try {
    await store.toggleScheduleEnabled(id)
    message.text = 'Schedule updated successfully'
    message.type = 'success'
  } catch (error: any) {
    message.text = error.message || 'Failed to toggle schedule'
    message.type = 'error'
  }
}

/**
 * Edit a schedule
 */
function editSchedule(schedule: ExportSchedule) {
  editingSchedule.value = schedule
  store.clearErrors()
}

/**
 * Confirm delete
 */
function confirmDelete(schedule: ExportSchedule) {
  deleteConfirm.value = schedule
}

/**
 * Perform the delete
 */
async function performDelete() {
  if (!deleteConfirm.value) return
  
  try {
    await store.deleteSchedule(deleteConfirm.value.id)
    message.text = `Schedule "${deleteConfirm.value.name}" deleted successfully`
    message.type = 'success'
    deleteConfirm.value = null
  } catch (error: any) {
    message.text = error.message || 'Failed to delete schedule'
    message.type = 'error'
  }
}

/**
 * Handle form submission (create or update)
 */
async function handleFormSubmit(request: ExportScheduleRequest) {
  try {
    if (editingSchedule.value) {
      // Update existing
      await store.updateSchedule(editingSchedule.value.id, request)
      message.text = 'Schedule updated successfully'
    } else {
      // Create new
      await store.createSchedule(request)
      message.text = 'Schedule created successfully'
    }
    
    message.type = 'success'
    closeModal()
    // Refresh the list to get updated data
    store.fetchSchedules()
  } catch (error: any) {
    // Error is already stored in the store, form will display it
    console.error('Form submission error:', error)
  }
}

/**
 * Close any open modal
 */
function closeModal() {
  showCreateForm.value = false
  editingSchedule.value = null
  store.clearErrors()
}
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0;
  color: #1f2937;
}

.primary-button {
  background-color: #3b82f6;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: background-color 0.2s;
}

.primary-button:hover {
  background-color: #2563eb;
}

.stats-section {
  margin-bottom: 24px;
}

.stats {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.card {
  border: 1px solid #e5e7eb;
  padding: 16px;
  border-radius: 6px;
  background: #ffffff;
  display: flex;
  align-items: center;
  gap: 8px;
}

.stat-label {
  font-weight: 500;
  color: #6b7280;
}

.stat-value {
  font-size: 1.25rem;
  font-weight: 600;
  color: #1f2937;
}

.stat-value.enabled {
  color: #10b981;
}

.stat-value.disabled {
  color: #f59e0b;
}

.schedule-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.schedule-id {
  font-size: 0.75rem;
  color: #6b7280;
  font-family: monospace;
}

.status-badge {
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  text-transform: uppercase;
}

.status-badge.enabled {
  background: #dcfce7;
  color: #166534;
}

.status-badge.disabled {
  background: #fef3c7;
  color: #92400e;
}

.time-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.time-remaining {
  font-size: 0.75rem;
  color: #6b7280;
  font-style: italic;
}

.recent-result {
  font-size: 0.75rem;
}

.recent-result .success {
  color: #10b981;
}

.recent-result .warning {
  color: #f59e0b;
}

.no-data {
  color: #9ca3af;
  font-style: italic;
}

.action-buttons {
  display: flex;
  gap: 4px;
  align-items: center;
}

.action-btn {
  background: none;
  border: 1px solid #d1d5db;
  padding: 4px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 0.875rem;
}

.action-btn:hover:not(:disabled) {
  background: #f3f4f6;
}

.action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.run-btn:hover:not(:disabled) {
  background: #dcfce7;
  border-color: #10b981;
}

.toggle-btn:hover:not(:disabled) {
  background: #fef3c7;
  border-color: #f59e0b;
}

.edit-btn:hover:not(:disabled) {
  background: #dbeafe;
  border-color: #3b82f6;
}

.delete-btn:hover:not(:disabled) {
  background: #fee2e2;
  border-color: #dc2626;
}

.modal-overlay {
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
  padding: 16px;
}

.modal-content {
  background: white;
  border-radius: 8px;
  max-width: 800px;
  width: 100%;
  max-height: 90vh;
  overflow: auto;
}

.confirm-modal {
  padding: 24px;
  max-width: 400px;
}

.confirm-modal h3 {
  margin: 0 0 16px 0;
  color: #1f2937;
}

.confirm-modal p {
  margin: 0 0 8px 0;
  color: #4b5563;
}

.confirm-modal .warning {
  color: #dc2626;
  font-weight: 500;
  margin-bottom: 24px;
}

.modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

.secondary-button {
  background: #f3f4f6;
  color: #374151;
  border: 1px solid #d1d5db;
  padding: 8px 16px;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.secondary-button:hover {
  background: #e5e7eb;
}

.danger-button {
  background: #dc2626;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.danger-button:hover {
  background: #b91c1c;
}

.message {
  position: fixed;
  top: 20px;
  right: 20px;
  padding: 12px 16px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  gap: 12px;
  z-index: 1001;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}

.message.success {
  background: #dcfce7;
  color: #166534;
  border: 1px solid #bbf7d0;
}

.message.error {
  background: #fee2e2;
  color: #dc2626;
  border: 1px solid #fecaca;
}

.message-close {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  font-size: 1.1rem;
  padding: 0;
  line-height: 1;
}

/* Responsive design */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }

  .stats {
    flex-direction: column;
  }

  .action-buttons {
    flex-direction: column;
    gap: 2px;
  }

  .action-btn {
    width: 100%;
    text-align: center;
  }

  .modal-content {
    margin: 8px;
  }
}
</style>