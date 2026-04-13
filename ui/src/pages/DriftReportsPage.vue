<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h1>Drift Reports</h1>
        <p class="subhead">Per-device, per-path drift items tracked over time.</p>
      </div>
      <div class="filters">
        <select v-model="resolvedFilter" class="select" @change="handleFilterChange">
          <option value="">All Items</option>
          <option value="false">Unresolved Only</option>
          <option value="true">Resolved Only</option>
        </select>
      </div>
    </div>

    <div v-if="store.loading" class="loading">Loading drift items...</div>
    <div v-else-if="store.error" class="error">{{ store.error }}</div>

    <div v-else class="content">
      <div v-if="store.trends.length === 0" class="empty">
        <p>No drift items found.</p>
        <p v-if="resolvedFilter">Try adjusting your filters.</p>
      </div>

      <div v-else class="reports-table-container">
        <table class="reports-table">
          <thead>
            <tr>
              <th>Device</th>
              <th>Path</th>
              <th>Category</th>
              <th>Severity</th>
              <th>First Seen</th>
              <th>Last Seen</th>
              <th>Occurrences</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in store.trends"
              :key="item.id"
              :class="{ resolved: item.resolved }"
            >
              <td>
                <router-link :to="`/devices/${item.device_id}`" class="device-link">
                  Device #{{ item.device_id }}
                </router-link>
              </td>
              <td>
                <code class="field-name">{{ item.path }}</code>
              </td>
              <td>
                <span class="category-badge">{{ item.category }}</span>
              </td>
              <td>
                <span class="severity-badge" :class="item.severity">
                  {{ item.severity }}
                </span>
              </td>
              <td>
                <span class="timestamp">{{ formatDate(item.first_seen) }}</span>
              </td>
              <td>
                <span class="timestamp">{{ formatDate(item.last_seen) }}</span>
              </td>
              <td class="numeric">{{ item.occurrences }}</td>
              <td>
                <span v-if="item.resolved" class="status-badge resolved">
                  Resolved
                  <span v-if="item.resolved_at" class="resolved-info">
                    {{ formatDate(item.resolved_at) }}
                  </span>
                </span>
                <span v-else class="status-badge unresolved">Unresolved</span>
              </td>
              <td>
                <button
                  v-if="!item.resolved"
                  class="btn-small"
                  @click="openResolveDialog(item)"
                >
                  Resolve
                </button>
                <span v-else class="resolved-mark">✓</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="resolvingItem" class="modal" @click.self="closeResolveDialog">
      <div class="modal-content">
        <div class="modal-header">
          <h2>Resolve Drift Item</h2>
          <button class="btn-close" @click="closeResolveDialog">×</button>
        </div>

        <div class="report-summary">
          <div class="summary-item">
            <span class="label">Device:</span>
            <span class="value">#{{ resolvingItem.device_id }}</span>
          </div>
          <div class="summary-item">
            <span class="label">Path:</span>
            <code class="value">{{ resolvingItem.path }}</code>
          </div>
          <div class="summary-item">
            <span class="label">Category:</span>
            <span class="value">{{ resolvingItem.category }}</span>
          </div>
          <div class="summary-item">
            <span class="label">Severity:</span>
            <span class="severity-badge" :class="resolvingItem.severity">
              {{ resolvingItem.severity }}
            </span>
          </div>
          <div class="summary-item">
            <span class="label">Occurrences:</span>
            <span class="value">{{ resolvingItem.occurrences }}</span>
          </div>
        </div>

        <p class="resolution-note">
          Marking this drift pattern as resolved clears it from the unresolved list.
          The backend does not persist resolution notes.
        </p>

        <div class="form-actions">
          <button type="button" class="btn" @click="closeResolveDialog">Cancel</button>
          <button type="button" class="btn primary" @click="handleResolve">
            Mark as Resolved
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useDriftStore } from '@/stores/drift'
import type { DriftTrend } from '@/api/drift'

const store = useDriftStore()

const resolvedFilter = ref('')
const resolvingItem = ref<DriftTrend | null>(null)

const resolvedBoolean = computed<boolean | undefined>(() => {
  if (resolvedFilter.value === 'true') return true
  if (resolvedFilter.value === 'false') return false
  return undefined
})

function formatDate(dateStr?: string | null): string {
  if (!dateStr) return '—'
  return new Date(dateStr).toLocaleString()
}

function openResolveDialog(item: DriftTrend) {
  resolvingItem.value = item
}

function closeResolveDialog() {
  resolvingItem.value = null
}

async function handleResolve() {
  if (!resolvingItem.value) return
  try {
    await store.resolveTrend(resolvingItem.value.id)
    closeResolveDialog()
  } catch (e: any) {
    alert(e?.message || 'Failed to resolve drift item')
  }
}

async function handleFilterChange() {
  await store.fetchTrends(resolvedBoolean.value)
}

onMounted(() => {
  store.fetchTrends(resolvedBoolean.value)
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
  gap: 16px;
}

.page-header h1 {
  margin: 0;
  font-size: 24px;
  color: #1f2937;
}

.subhead {
  margin: 4px 0 0 0;
  font-size: 13px;
  color: #64748b;
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

.reports-table td.numeric {
  text-align: right;
  font-variant-numeric: tabular-nums;
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

.category-badge {
  padding: 2px 8px;
  background: #e0e7ff;
  color: #3730a3;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
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

.severity-badge.medium,
.severity-badge.warning {
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
  margin-bottom: 16px;
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
  min-width: 90px;
}

.summary-item .value {
  font-size: 14px;
  color: #1f2937;
}

.resolution-note {
  font-size: 13px;
  color: #6b7280;
  padding: 12px;
  background: #f9fafb;
  border-left: 3px solid #cbd5e1;
  border-radius: 4px;
  margin-bottom: 16px;
}

.form-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>
