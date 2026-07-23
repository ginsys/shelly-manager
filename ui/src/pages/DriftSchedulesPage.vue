<template>
  <div class="page">
    <div class="page-header">
      <h1>Drift Detection Schedules</h1>
    </div>

    <!-- Fail-closed notice (#270): schedules are stored but never executed. -->
    <div class="notice" role="status">
      <strong>Drift schedules are not executed in this release.</strong>
      <p>
        Stored schedules are shown for inspection and deletion only. Creating, editing,
        enabling and run history are unavailable until scheduled execution ships
        (tracked in issue #279). Any schedule below is inert persisted data, not an
        operational job.
      </p>
    </div>

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
              <span class="status-badge">Stored (inactive)</span>
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
            <span class="stored-meta-label">Persisted values (not executed):</span>
            <span class="stored-meta-item">Stored enabled flag: {{ schedule.enabled ? 'true' : 'false' }}</span>
            <span class="stored-meta-item">Run count: {{ schedule.run_count }}</span>
            <span v-if="schedule.last_run" class="stored-meta-item">Last run: {{ formatDate(schedule.last_run) }}</span>
            <span v-if="schedule.next_run" class="stored-meta-item">Next run: {{ formatDate(schedule.next_run) }}</span>
            <span class="stored-meta-item">Run history: unavailable</span>
          </div>

          <div class="schedule-actions">
            <button class="btn-icon danger" title="Delete" @click="handleDelete(schedule.id)">
              🗑️ Delete
            </button>
          </div>
        </div>
      </div>

      <div v-if="store.scheduleMeta && (store.scheduleMeta.total_count || 0) > pageSize" class="pagination">
        <button
          class="btn"
          :disabled="page === 1"
          @click="handlePageChange(page - 1)"
        >
          Previous
        </button>
        <span class="page-info">
          Page {{ page }} of {{ Math.ceil((store.scheduleMeta.total_count || 0) / pageSize) }}
          ({{ (store.scheduleMeta.total_count || 0) }} total)
        </span>
        <button
          class="btn"
          :disabled="page >= Math.ceil((store.scheduleMeta.total_count || 0) / pageSize)"
          @click="handlePageChange(page + 1)"
        >
          Next
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useDriftStore } from '@/stores/drift'

const store = useDriftStore()

const page = ref(1)
const pageSize = ref(25)

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
  if (!confirm('Delete this stored schedule? This only removes persisted data.')) {
    return
  }
  try {
    await store.remove(id)
  } catch (e: any) {
    alert(e?.message || 'Failed to delete schedule')
  }
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
  /* Neutral, non-operational styling: stored data, never an active job. */
  opacity: 0.85;
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

.stored-meta-label {
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
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
</style>
