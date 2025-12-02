<template>
  <div class="page">
    <div class="page-header">
      <h1>Drift Reports</h1>
      <div class="filters">
        <select v-model="resolvedFilter" class="select" @change="handleFilterChange">
          <option value="">All Reports</option>
          <option value="false">Unresolved Only</option>
          <option value="true">Resolved Only</option>
        </select>
      </div>
    </div>

    <div v-if="store.loading" class="loading">Loading reports...</div>
    <div v-else-if="store.error" class="error">{{ store.error }}</div>

    <div v-else class="content">
      <div v-if="store.reports.length === 0" class="empty">
        <p>No drift reports found.</p>
        <p v-if="resolvedFilter">Try adjusting your filters.</p>
      </div>

      <div v-else class="reports-table-container">
        <table class="reports-table">
          <thead>
            <tr>
              <th>Device</th>
              <th>Field</th>
              <th>Drift Type</th>
              <th>Expected</th>
              <th>Actual</th>
              <th>Severity</th>
              <th>Detected</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="report in store.reports"
              :key="report.id"
              :class="{ resolved: report.resolved }"
            >
              <td>
                <router-link :to="`/devices/${report.deviceId}`" class="device-link">
                  {{ report.deviceName }}
                </router-link>
              </td>
              <td>
                <code class="field-name">{{ report.field }}</code>
              </td>
              <td>
                <span class="drift-type" :class="report.driftType">
                  {{ formatDriftType(report.driftType) }}
                </span>
              </td>
              <td>
                <code class="value">{{ formatValue(report.expectedValue) }}</code>
              </td>
              <td>
                <code class="value">{{ formatValue(report.actualValue) }}</code>
              </td>
              <td>
                <span class="severity-badge" :class="report.severity">
                  {{ report.severity }}
                </span>
              </td>
              <td>
                <span class="timestamp">{{ formatDate(report.detectedAt) }}</span>
              </td>
              <td>
                <span v-if="report.resolved" class="status-badge resolved">
                  Resolved
                  <span v-if="report.resolvedAt" class="resolved-info">
                    {{ formatDate(report.resolvedAt) }}
                    <span v-if="report.resolvedBy">by {{ report.resolvedBy }}</span>
                  </span>
                </span>
                <span v-else class="status-badge unresolved">Unresolved</span>
              </td>
              <td>
                <button
                  v-if="!report.resolved"
                  class="btn-small"
                  @click="openResolveDialog(report)"
                >
                  Resolve
                </button>
                <span v-else class="resolved-mark">✓</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="store.reportMeta && store.reportMeta.total > pageSize" class="pagination">
        <button class="btn" :disabled="page === 1" @click="handlePageChange(page - 1)">
          Previous
        </button>
        <span class="page-info">
          Page {{ page }} of {{ Math.ceil(store.reportMeta.total / pageSize) }}
          ({{ store.reportMeta.total }} total)
        </span>
        <button
          class="btn"
          :disabled="page >= Math.ceil(store.reportMeta.total / pageSize)"
          @click="handlePageChange(page + 1)"
        >
          Next
        </button>
      </div>
    </div>

    <!-- Resolve Dialog -->
    <div v-if="resolvingReport" class="modal" @click.self="closeResolveDialog">
      <div class="modal-content">
        <div class="modal-header">
          <h2>Resolve Drift Report</h2>
          <button class="btn-close" @click="closeResolveDialog">×</button>
        </div>

        <div class="report-summary">
          <div class="summary-item">
            <span class="label">Device:</span>
            <span class="value">{{ resolvingReport.deviceName }}</span>
          </div>
          <div class="summary-item">
            <span class="label">Field:</span>
            <code class="value">{{ resolvingReport.field }}</code>
          </div>
          <div class="summary-item">
            <span class="label">Drift Type:</span>
            <span class="value">{{ formatDriftType(resolvingReport.driftType) }}</span>
          </div>
          <div class="summary-item">
            <span class="label">Severity:</span>
            <span class="severity-badge" :class="resolvingReport.severity">
              {{ resolvingReport.severity }}
            </span>
          </div>
        </div>

        <form @submit.prevent="handleResolve">
          <div class="form-field">
            <label for="notes">Resolution Notes</label>
            <textarea
              id="notes"
              v-model="resolveNotes"
              class="form-textarea"
              rows="4"
              placeholder="Optional: Add notes about how this drift was resolved..."
            />
          </div>

          <div class="form-actions">
            <button type="button" class="btn" @click="closeResolveDialog">Cancel</button>
            <button type="submit" class="btn primary">Mark as Resolved</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useDriftStore } from '@/stores/drift'
import type { DriftReport } from '@/api/drift'

const store = useDriftStore()

const page = ref(1)
const pageSize = ref(25)
const resolvedFilter = ref('')
const resolvingReport = ref<DriftReport | null>(null)
const resolveNotes = ref('')

const resolvedBoolean = computed<boolean | undefined>(() => {
  if (resolvedFilter.value === 'true') return true
  if (resolvedFilter.value === 'false') return false
  return undefined
})

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function formatDriftType(type: string): string {
  return type
    .replace(/_/g, ' ')
    .replace(/\b\w/g, l => l.toUpperCase())
}

function formatValue(value: any): string {
  if (value === null || value === undefined) return 'null'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function openResolveDialog(report: DriftReport) {
  resolvingReport.value = report
  resolveNotes.value = report.notes || ''
}

function closeResolveDialog() {
  resolvingReport.value = null
  resolveNotes.value = ''
}

async function handleResolve() {
  if (!resolvingReport.value) return

  try {
    await store.resolveReport(
      resolvingReport.value.id,
      resolveNotes.value.trim() || undefined
    )
    closeResolveDialog()
  } catch (e: any) {
    alert(e?.message || 'Failed to resolve report')
  }
}

async function handleFilterChange() {
  page.value = 1
  await store.fetchReports(page.value, pageSize.value, resolvedBoolean.value)
}

async function handlePageChange(newPage: number) {
  page.value = newPage
  await store.fetchReports(page.value, pageSize.value, resolvedBoolean.value)
}

onMounted(() => {
  store.fetchReports(page.value, pageSize.value, resolvedBoolean.value)
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

.filters {
  display: flex;
  gap: 12px;
}

.select {
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 14px;
  background: white;
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

.reports-table-container {
  overflow-x: auto;
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.reports-table {
  width: 100%;
  border-collapse: collapse;
}

.reports-table thead {
  background: #f9fafb;
}

.reports-table th {
  padding: 12px;
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: #6b7280;
  text-transform: uppercase;
  border-bottom: 1px solid #e5e7eb;
}

.reports-table td {
  padding: 12px;
  border-bottom: 1px solid #f3f4f6;
  font-size: 14px;
}

.reports-table tbody tr:hover {
  background: #f9fafb;
}

.reports-table tbody tr.resolved {
  opacity: 0.6;
}

.device-link {
  color: #2563eb;
  text-decoration: none;
  font-weight: 500;
}

.device-link:hover {
  text-decoration: underline;
}

.field-name {
  padding: 2px 6px;
  background: #f3f4f6;
  border-radius: 4px;
  font-family: ui-monospace, monospace;
  font-size: 13px;
}

.drift-type {
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.drift-type.config_changed {
  background: #dbeafe;
  color: #1e40af;
}

.drift-type.unexpected_value {
  background: #fef3c7;
  color: #92400e;
}

.drift-type.missing_config {
  background: #fee2e2;
  color: #991b1b;
}

.value {
  padding: 2px 6px;
  background: #f9fafb;
  border-radius: 4px;
  font-family: ui-monospace, monospace;
  font-size: 13px;
}

.severity-badge {
  padding: 4px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
}

.severity-badge.low {
  background: #dbeafe;
  color: #1e40af;
}

.severity-badge.medium {
  background: #fef3c7;
  color: #92400e;
}

.severity-badge.high {
  background: #fed7aa;
  color: #9a3412;
}

.severity-badge.critical {
  background: #fee2e2;
  color: #991b1b;
}

.timestamp {
  font-size: 13px;
  color: #6b7280;
}

.status-badge {
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.status-badge.resolved {
  background: #dcfce7;
  color: #166534;
}

.status-badge.unresolved {
  background: #fef3c7;
  color: #92400e;
}

.resolved-info {
  display: block;
  font-size: 11px;
  margin-top: 2px;
  opacity: 0.8;
}

.btn-small {
  padding: 4px 12px;
  border: 1px solid #cbd5e1;
  background: #fff;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}

.btn-small:hover {
  background: #f8fafc;
}

.resolved-mark {
  color: #16a34a;
  font-size: 18px;
  font-weight: bold;
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

.report-summary {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  background: #f9fafb;
  border-radius: 6px;
  margin-bottom: 20px;
}

.summary-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.summary-item .label {
  font-size: 13px;
  font-weight: 600;
  color: #6b7280;
  min-width: 80px;
}

.summary-item .value {
  font-size: 14px;
  color: #1f2937;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 16px;
}

.form-field label {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
}

.form-textarea {
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-family: inherit;
  font-size: 14px;
  resize: vertical;
}

.form-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>
