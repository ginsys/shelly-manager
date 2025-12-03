<template>
  <main style="padding:16px" data-testid="backup-management-page">
    <div class="page-header">
      <h1 data-testid="page-title">Backup Management</h1>
    </div>

    <BackupCreatePanel
      :run-mode="runMode"
      :create-type="createType"
      :create-name="createName"
      :create-desc="createDesc"
      :create-compression="createCompression"
      :create-output-dir="createOutputDir"
      :schedule-preset="schedulePreset"
      :schedule-interval="scheduleInterval"
      :schedule-enabled="scheduleEnabled"
      :json-pretty="jsonOptions.pretty"
      :json-include-discovered="jsonOptions.include_discovered"
      :json-compression="jsonCompression"
      :yaml-include-discovered="yamlOptions.include_discovered"
      :yaml-compression="yamlCompression"
      :sma-compression-level="smaOptions.compression_level"
      :sma-include-discovered="smaOptions.include_discovered"
      :sma-include-network-settings="smaOptions.include_network_settings"
      :sma-include-plugin-configs="smaOptions.include_plugin_configs"
      :sma-include-system-settings="smaOptions.include_system_settings"
      :sma-exclude-sensitive="smaOptions.exclude_sensitive"
      :export-output-dir="exportOutputDir"
      :provider-name="providerName"
      :provider-version="providerVersion"
      :submitting="createSubmitting"
      :error="createError2"
      @update:run-mode="(v: any) => runMode = v"
      @update:create-type="(v: any) => createType = v"
      @update:create-name="(v: any) => createName = v"
      @update:create-desc="(v: any) => createDesc = v"
      @update:create-compression="(v: any) => createCompression = v"
      @update:create-output-dir="(v: any) => createOutputDir = v"
      @update:schedule-preset="(v: any) => schedulePreset = v"
      @update:schedule-interval="(v: any) => scheduleInterval = v"
      @update:schedule-enabled="(v: any) => scheduleEnabled = v"
      @update:json-pretty="(v: any) => jsonOptions.pretty = v"
      @update:json-include-discovered="(v: any) => jsonOptions.include_discovered = v"
      @update:json-compression="(v: any) => jsonCompression = v"
      @update:yaml-include-discovered="(v: any) => yamlOptions.include_discovered = v"
      @update:yaml-compression="(v: any) => yamlCompression = v"
      @update:sma-compression-level="(v: any) => smaOptions.compression_level = v"
      @update:sma-include-discovered="(v: any) => smaOptions.include_discovered = v"
      @update:sma-include-network-settings="(v: any) => smaOptions.include_network_settings = v"
      @update:sma-include-plugin-configs="(v: any) => smaOptions.include_plugin_configs = v"
      @update:sma-include-system-settings="(v: any) => smaOptions.include_system_settings = v"
      @update:sma-exclude-sensitive="(v: any) => smaOptions.exclude_sensitive = v"
      @update:export-output-dir="(v: any) => exportOutputDir = v"
      @apply-preset="applyIntervalPreset"
      @submit="createBackupPanel"
    />

    <!-- Backup Statistics -->
    <BackupStatistics :stats="statistics" />

    <!-- Filters -->
    <BackupFilters
      :format="filters.format"
      :success="filters.success"
      :loading="loading"
      @update:format="(v: string) => { filters.format = v; fetchBackups() }"
      @update:success="(v?: boolean) => { filters.success = v; fetchBackups() }"
      @refresh="refreshData"
    />

    <ErrorDisplay
      v-if="error && !loading"
      :error="{ code: 'BACKUP_LIST_FAILED', message: error, retryable: true }"
      title="Failed to load backups"
      @retry="fetchBackups"
      @dismiss="() => (error = '')"
    />

    <!-- Backups Table -->
    <BackupList
      :rows="backups"
      :loading="loading"
      :error="error"
      @download="downloadBackup"
      @restore="startRestore"
      @delete="confirmDelete"
    />

    <!-- Content Exports Table -->
    <ContentExportList :rows="contentExports" @download="downloadContent" />


    <!-- Pagination -->
    <PaginationBar
      v-if="meta?.pagination"
      :page="meta.pagination.page"
      :totalPages="meta.pagination.total_pages"
      :hasNext="meta.pagination.has_next"
      :hasPrev="meta.pagination.has_previous"
      @update:page="(p: number) => { currentPage = p; fetchBackups() }"
    />

    <!-- (removed modal-based backup creation; using in-page create panel) -->

    <!-- Restore Modal -->
    <RestoreModal
      v-if="showRestoreModal"
      :backup="restoreBackup"
      v-model:options="restoreOptions"
      :preview="restorePreview"
      :loading="restoreLoading"
      :error="restoreError"
      @close="closeRestoreModal"
      @preview="previewRestore"
      @execute="executeRestore"
    />

    <!-- Delete Confirmation Modal -->
    <ConfirmDialog
      v-if="deleteConfirm"
      title="Confirm Delete"
      :message="`Are you sure you want to delete backup <strong>${deleteConfirm.name}</strong>?<br/>This action cannot be undone.`"
      confirmText="Delete Backup"
      cancelText="Cancel"
      @cancel="deleteConfirm = null"
      @confirm="performDelete"
    />

    <!-- Success/Error Messages -->
    <MessageBanner v-if="message.text" :text="message.text" :type="message.type" @close="message.text = ''" />
  </main>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import PaginationBar from '@/components/PaginationBar.vue'
import { formatFileSize, formatDate } from '@/utils/format'
import BackupStatistics from '@/components/backup/BackupStatistics.vue'
import BackupCreatePanel from '@/components/backup/BackupCreatePanel.vue'
import BackupFilters from '@/components/backup/BackupFilters.vue'
import BackupList from '@/components/backup/BackupList.vue'
import ContentExportList from '@/components/backup/ContentExportList.vue'
import RestoreModal from '@/components/backup/RestoreModal.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import MessageBanner from '@/components/shared/MessageBanner.vue'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import { useBackups } from '@/composables/useBackups'

const {
  backups, statistics, availableDevices, contentExports, meta, loading, error, downloading, message,
  filters, currentPage, pageSize,
  runMode, createType, createName, createDesc, createCompression, createOutputDir, exportOutputDir,
  jsonOptions, yamlOptions, jsonCompression, yamlCompression, smaOptions,
  scheduleEnabled, scheduleInterval, schedulePreset, createSubmitting, createError2, providerName, providerVersion,
  showRestoreModal, restoreBackup, restoreOptions, restorePreview, restoreLoading, restoreError, deleteConfirm,
  fetchBackups, fetchStatistics, fetchAvailableDevices, fetchContentExports, refreshData, applyIntervalPreset, createBackupPanel,
  startRestore, previewRestoreAction, executeRestoreAction, closeRestoreModal,
  downloadBackupAction, downloadContentAction, confirmDelete, performDelete, showMessage,
} = useBackups()

const route = useRoute()

function scrollToCreate() {
  const el = document.getElementById('create-backup')
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

onMounted(() => {
  refreshData()
  const q: any = route.query
  if (q && (q.schedule === '1' || q.schedule === 'true')) {
    runMode.value = 'schedule'
    const t = String(q.type || '')
    if (['backup','json','yaml','sma'].includes(t)) createType.value = t as any
    scrollToCreate()
  }
})

const downloadBackup = downloadBackupAction
const downloadContent = downloadContentAction
const previewRestore = previewRestoreAction
const executeRestore = executeRestoreAction
</script>

<style scoped src="../styles/pages/backup.css">
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
  min-width: 120px;
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

.stat-value.success {
  color: #10b981;
}

.stat-value.failure {
  color: #ef4444;
}

.filters-section {
  margin-bottom: 24px;
  padding: 16px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.filter-row {
  display: flex;
  gap: 16px;
  align-items: flex-end;
  flex-wrap: wrap;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.filter-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #374151;
}

.filter-select {
  padding: 6px 10px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  background: white;
  font-size: 0.875rem;
}

/* In-page create backup panel styles */
.create-section { 
  margin-bottom: 24px; 
  padding: 16px; 
  background: #f9fafb; 
  border: 1px solid #e5e7eb; 
  border-radius: 6px 
}
.grid-2 { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px }
.form-actions { margin-top: 12px; display:flex; align-items:center; gap: 8px }

.filter-actions {
  margin-left: auto;
}

.refresh-button {
  background: #10b981;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 4px;
  font-size: 0.875rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.refresh-button:hover:not(:disabled) {
  background: #059669;
}

.refresh-button:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

.backup-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.backup-description {
  font-size: 0.75rem;
  color: #6b7280;
  font-style: italic;
}

.backup-id {
  font-size: 0.75rem;
  color: #6b7280;
  font-family: monospace;
}

.format-badge {
  background: #dbeafe;
  color: #1e40af;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.file-size {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.checksum {
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

.status-badge.success {
  background: #dcfce7;
  color: #166534;
}

.status-badge.failure {
  background: #fee2e2;
  color: #991b1b;
}

.encryption-badge {
  font-size: 0.875rem;
  font-weight: 500;
}

.encryption-badge.encrypted {
  color: #059669;
}

.encryption-badge.plain {
  color: #6b7280;
}

.time-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.created-by {
  font-size: 0.75rem;
  color: #6b7280;
}

.error-message {
  font-size: 0.75rem;
  color: #dc2626;
  margin-top: 2px;
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

.download-btn:hover:not(:disabled) {
  background: #dcfce7;
  border-color: #10b981;
}

.restore-btn:hover:not(:disabled) {
  background: #fef3c7;
  border-color: #f59e0b;
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

.restore-modal {
  max-width: 600px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px;
  border-bottom: 1px solid #e5e7eb;
}

.modal-header h3 {
  margin: 0;
  color: #1f2937;
}

.close-button {
  background: none;
  border: none;
  color: #6b7280;
  cursor: pointer;
  font-size: 1.2rem;
  padding: 4px;
  line-height: 1;
  transition: color 0.2s;
}

.close-button:hover {
  color: #374151;
}

.restore-content {
  padding: 24px;
}

.backup-info {
  margin-bottom: 24px;
  padding: 16px;
  background: #f3f4f6;
  border-radius: 6px;
}

.backup-info h4 {
  margin: 0 0 8px 0;
  color: #1f2937;
}

.backup-info p {
  margin: 0 0 12px 0;
  color: #4b5563;
}

.backup-details {
  font-size: 0.875rem;
  color: #6b7280;
}

.form-section {
  margin-bottom: 24px;
}

.form-section h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
  font-size: 1rem;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  cursor: pointer;
}

.restore-preview {
  margin-bottom: 24px;
  padding: 16px;
  background: #f0f9ff;
  border: 1px solid #bae6fd;
  border-radius: 6px;
}

.restore-preview h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
}

.preview-stats {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
  margin-bottom: 16px;
  font-size: 0.875rem;
}

.warnings, .conflicts {
  margin-bottom: 12px;
}

.warnings h5, .conflicts h5 {
  margin: 0 0 8px 0;
  color: #1f2937;
}

.warnings ul, .conflicts ul {
  margin: 0;
  padding-left: 20px;
  font-size: 0.875rem;
}

.warnings {
  color: #d97706;
}

.conflicts {
  color: #dc2626;
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

.form-error {
  margin-bottom: 20px;
  padding: 12px;
  background: #fee2e2;
  border: 1px solid #fecaca;
  border-radius: 6px;
  color: #dc2626;
  font-size: 0.875rem;
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

  .filter-row {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .filter-actions {
    margin-left: 0;
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

  .preview-stats {
    grid-template-columns: 1fr;
  }
}
</style>
import BackupStatistics from '@/components/backup/BackupStatistics.vue'
import BackupCreatePanel from '@/components/backup/BackupCreatePanel.vue'
import BackupFilters from '@/components/backup/BackupFilters.vue'
import BackupList from '@/components/backup/BackupList.vue'
import ContentExportList from '@/components/backup/ContentExportList.vue'
import RestoreModal from '@/components/backup/RestoreModal.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import MessageBanner from '@/components/shared/MessageBanner.vue'
import { formatFileSize, formatDate } from '@/utils/format'
