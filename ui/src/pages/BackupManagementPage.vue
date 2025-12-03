<template>
  <main style="padding:16px" data-testid="backup-management-page">
    <div class="page-header">
      <h1 data-testid="page-title">Backup Management</h1>
    </div>

    <!-- In-page Create Backup Panel -->
    <BackupCreateForm
      v-model:createType="createType"
      v-model:runMode="runMode"
      v-model:createName="createName"
      v-model:createDesc="createDesc"
      v-model:createCompression="createCompression"
      v-model:createOutputDir="createOutputDir"
      v-model:exportOutputDir="exportOutputDir"
      v-model:scheduleEnabled="scheduleEnabled"
      v-model:scheduleInterval="scheduleInterval"
      v-model:schedulePreset="schedulePreset"
      v-model:jsonOptions="jsonOptions"
      v-model:yamlOptions="yamlOptions"
      v-model:jsonCompression="jsonCompression"
      v-model:yamlCompression="yamlCompression"
      v-model:smaOptions="smaOptions"
      :submitting="createSubmitting"
      :error="createError2"
      :providerName="providerName"
      :providerVersion="providerVersion"
      @submit="createBackupPanel"
    />

    <!-- Backup Statistics -->
    <BackupStatisticsComponent :statistics="statistics" />

    <!-- Filters -->
    <BackupFilterBar
      v-model:filters="filters"
      :loading="loading"
      @filter-change="fetchBackups"
      @refresh="refreshData"
    />

    <!-- Backups Table -->
    <BackupList
      :backups="backups"
      :loading="loading"
      :error="error"
      :downloading="downloading"
      @download="downloadBackup"
      @restore="startRestore"
      @delete="confirmDelete"
    />

    <!-- Content Exports Table -->
    <ContentExportsList
      :content-exports="contentExports"
      @download="downloadContent"
    />


    <!-- Pagination -->
    <PaginationBar
      v-if="meta?.pagination"
      :page="meta.pagination.page"
      :totalPages="meta.pagination.total_pages"
      :hasNext="meta.pagination.has_next"
      :hasPrev="meta.pagination.has_previous"
      @update:page="(p: number) => { currentPage = p; fetchBackups() }"
    />

    <!-- Restore Modal -->
    <RestoreModal
      :show="showRestoreModal"
      :backup="restoreBackup"
      v-model:options="restoreOptions"
      :preview="restorePreview"
      :loading="restoreLoading"
      :error="restoreError"
      @close="closeRestoreModal"
      @preview-restore="previewRestore"
      @execute="executeRestore"
    />

    <!-- Delete Confirmation Modal -->
    <DeleteConfirmModal
      :backup="deleteConfirm"
      @confirm="performDelete"
      @cancel="deleteConfirm = null"
    />

    <!-- Success/Error Messages -->
    <div v-if="message.text" :class="['message', message.type]">
      {{ message.text }}
      <button class="message-close" @click="message.text = ''">✖</button>
    </div>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref, reactive, computed } from 'vue'
import { useError } from '@/composables/useError'
import {
  listBackups,
  getBackupStatistics,
  createBackup,
  createJSONExport,
  createYAMLExport,
  createSMAExport,
  deleteBackup,
  downloadBackupWithName,
  downloadExportWithName,
  previewRestore,
  executeRestore,
  type BackupRequest, 
  type BackupItem, 
  type BackupStatistics,
    type RestoreRequest,
    type RestorePreview
  } from '@/api/export'
  import { createSchedule, parseInterval, type ExportScheduleRequest } from '@/api/schedule'
  import { useRoute } from 'vue-router'
import api from '@/api/client'
import type { Device, Metadata } from '@/api/types'
import DataTable from '@/components/DataTable.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import BackupStatisticsComponent from '@/components/backup/BackupStatistics.vue'
import BackupFilterBar from '@/components/backup/BackupFilterBar.vue'
import BackupList from '@/components/backup/BackupList.vue'
import ContentExportsList from '@/components/backup/ContentExportsList.vue'
import RestoreModal from '@/components/backup/RestoreModal.vue'
import DeleteConfirmModal from '@/components/backup/DeleteConfirmModal.vue'
import BackupCreateForm from '@/components/backup/BackupCreateForm.vue'

// State
const backups = ref<BackupItem[]>([])
const statistics = ref<BackupStatistics>({
  total: 0,
  success: 0, 
  failure: 0,
  total_size: 0,
  by_format: {}
})
const availableDevices = ref<Device[]>([])
const meta = ref<Metadata>()
const contentExports = ref<any[]>([])
// Create Export state
const showCreateForm = ref(false)
const createFormat = ref<'json' | 'yaml' | 'sma'>('json')
const outputDir = ref('./data/exports')
  const jsonOptions = reactive({ pretty: true, include_discovered: true })
  const yamlOptions = reactive({ include_discovered: true })
  const jsonCompression = ref<'none'|'gzip'|'zip'>('none')
  const yamlCompression = ref<'none'|'gzip'|'zip'>('none')
const smaOptions = reactive({
  compression_level: 6,
  include_discovered: true,
  include_network_settings: false,
  include_plugin_configs: true,
  include_system_settings: true,
  exclude_sensitive: true,
})
const createLoading = ref(false)
const createError = ref('')
const loading = ref(false)
const { error: errorObj, hasError, setError, clearError } = useError()
const error = computed(() => errorObj.value?.message || '')

// Filters
const filters = reactive({
  format: '',
  success: undefined as boolean | undefined
})
const currentPage = ref(1)
const pageSize = ref(20)

// UI State
  const showRestoreModal = ref(false)
const downloading = ref('')
const deleteConfirm = ref<BackupItem | null>(null)
  const message = reactive({ 
    text: '', 
    type: 'success' as 'success' | 'error' 
  })

  // In-page create backup state
  const runMode = ref<'now' | 'schedule'>('now')
  const createType = ref<'backup' | 'json' | 'yaml' | 'sma'>('backup')
  const createName = ref('')
  const createDesc = ref('')
  const createCompression = ref<'none' | 'gzip' | 'zip'>('gzip')
  const createOutputDir = ref('./data/backups')
  const exportOutputDir = ref('./data/exports')
  const scheduleEnabled = ref(true)
  const scheduleInterval = ref('24 hours')
  const schedulePreset = ref('24 hours')
  const createSubmitting = ref(false)
  const createError2 = ref('')
  const providerName = ref('')
  const providerVersion = ref('')

// Restore state
const restoreBackup = ref<BackupItem | null>(null)
const restoreOptions = reactive<RestoreRequest>({
  backup_id: '',
  include_settings: true,
  include_schedules: true,
  include_metrics: false,
  dry_run: true
})
const restorePreview = ref<RestorePreview | null>(null)
const restoreLoading = ref(false)
const restoreError = ref('')

// Initialize
onMounted(() => {
  // Load data asynchronously without blocking page render
  loadInitialData()
})

/**
 * Load initial data in parallel (non-blocking)
 */
function loadInitialData() {
  // Fire and forget - don't block UI rendering
  Promise.all([
    fetchBackups().catch(err => console.warn('Failed to fetch backups:', err)),
    fetchStatistics().catch(err => console.warn('Failed to fetch statistics:', err)),
    fetchAvailableDevices().catch(err => console.warn('Failed to fetch devices:', err)),
    fetchContentExports().catch(err => console.warn('Failed to fetch content exports:', err)),
  ]).catch(err => {
    console.warn('Some data failed to load:', err)
  })
}

/**
 * Fetch backups list with current filters
 */
async function fetchBackups() {
  loading.value = true
  clearError()

  try {
    const result = await listBackups({
      page: currentPage.value,
      pageSize: pageSize.value,
      format: filters.format || undefined,
      success: filters.success
    })

    backups.value = result.items
    meta.value = result.meta
  } catch (err: any) {
    setError(err, { action: 'Loading backups', resource: 'Backup' })
  } finally {
    loading.value = false
  }
}

// Fetch content exports (JSON, YAML, SMA)
async function fetchContentExports() {
  try {
    const [jsonList, yamlList, smaList] = await Promise.all([
      // Reuse export history API via UI helper
      // Using direct API to filter by plugin
      api.get('/export/history', { params: { page: 1, page_size: 50, plugin: 'json' } }),
      api.get('/export/history', { params: { page: 1, page_size: 50, plugin: 'yaml' } }),
      api.get('/export/history', { params: { page: 1, page_size: 50, plugin: 'sma' } }),
    ])
    const extract = (res: any) => (res.data?.data?.history || []).map((x: any) => ({
      id: x.id,
      export_id: x.export_id,
      plugin_name: x.plugin_name,
      format: x.format,
      record_count: x.record_count,
      file_size: x.file_size,
      created_at: x.created_at,
      success: x.success,
    }))
    contentExports.value = [
      ...extract(jsonList),
      ...extract(yamlList),
      ...extract(smaList),
    ].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
  } catch (err) {
    console.error('Failed to fetch content exports:', err)
    contentExports.value = []
  }
}

function closeCreateModal() {
  showCreateForm.value = false
  createError.value = ''
}

async function createExport() {
  createLoading.value = true
  createError.value = ''
  try {
    if (createFormat.value === 'json') {
      await createJSONExport({ output_path: outputDir.value, ...jsonOptions })
    } else if (createFormat.value === 'yaml') {
      await createYAMLExport({ output_path: outputDir.value, ...yamlOptions })
    } else {
      await createSMAExport(
        { output_path: outputDir.value, compression_level: smaOptions.compression_level, include_checksums: true },
        {
          include_discovered: smaOptions.include_discovered,
          include_network_settings: smaOptions.include_network_settings,
          include_plugin_configs: smaOptions.include_plugin_configs,
          include_system_settings: smaOptions.include_system_settings,
        },
        {}
      )
    }
    showMessage('Export created successfully', 'success')
    closeCreateModal()
    await fetchContentExports()
  } catch (err: any) {
    createError.value = err.message || 'Failed to create export'
  } finally {
    createLoading.value = false
  }
}

/**
 * Fetch backup statistics
 */
async function fetchStatistics() {
  try {
    statistics.value = await getBackupStatistics()
  } catch (err) {
    console.error('Failed to load backup statistics:', err)
  }
}

/**
 * Fetch available devices for backup selection
 */
async function fetchAvailableDevices() {
  try {
    const res = await api.get('/devices', { params: { page_size: 1000 } })
    if (res.data && res.data.success && res.data.data && res.data.data.devices) {
      availableDevices.value = res.data.data.devices
    } else {
      availableDevices.value = []
    }
  } catch (err) {
    console.error('Failed to load devices:', err)
    availableDevices.value = []
  }
}

/**
 * Refresh all data
 */
async function refreshData() {
  await Promise.all([
    fetchBackups(),
    fetchStatistics(),
    fetchAvailableDevices()
  ])
  showMessage('Data refreshed successfully', 'success')
}

/**
 * Handle backup creation
 */
function scrollToCreate() {
  const el = document.getElementById('create-backup')
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

async function createBackupPanel() {
  createSubmitting.value = true
  createError2.value = ''
  try {
    // Build a unified ExportRequest for scheduling when needed
    const buildRequest = () => {
      const base: any = { plugin_name: '', format: '', config: {}, filters: {}, options: {} }
      if (createType.value === 'backup') {
        base.plugin_name = 'backup'
        base.format = 'sqlite'
        base.config = {
          output_path: createOutputDir.value,
          compression: createCompression.value !== 'none',
          compression_algo: createCompression.value === 'zip' ? 'zip' : 'gzip',
          name: createName.value,
          description: createDesc.value,
        }
      } else if (createType.value === 'json') {
        base.plugin_name = 'json'
        base.format = 'json'
        base.config = { output_path: exportOutputDir.value, ...jsonOptions }
      } else if (createType.value === 'yaml') {
        base.plugin_name = 'yaml'
        base.format = 'yaml'
        base.config = { output_path: exportOutputDir.value, ...yamlOptions }
      } else if (createType.value === 'sma') {
        base.plugin_name = 'sma'
        base.format = 'sma'
        base.config = { output_path: exportOutputDir.value, compression_level: smaOptions.compression_level, include_checksums: true }
        base.filters = {
          include_discovered: smaOptions.include_discovered,
          include_network_settings: smaOptions.include_network_settings,
          include_plugin_configs: smaOptions.include_plugin_configs,
          include_system_settings: smaOptions.include_system_settings,
        }
      }
      return base
    }

    if (runMode.value === 'schedule') {
      // Create schedule instead of running immediately
      const req = buildRequest()
      const seconds = parseInterval(scheduleInterval.value)
      const sched: ExportScheduleRequest = {
        name: createName.value || `${createType.value} schedule`,
        interval_sec: seconds,
        enabled: !!scheduleEnabled.value,
        request: req,
      }
      await createSchedule(sched)
      showMessage('Schedule created successfully', 'success')
    } else {
      // Immediate run paths (reuse specific endpoints for consistency)
      if (createType.value === 'backup') {
        const payload: any = {
          name: createName.value,
          description: createDesc.value,
          format: 'sqlite',
          config: { 
            output_path: createOutputDir.value,
            compression: createCompression.value !== 'none',
            compression_algo: createCompression.value === 'zip' ? 'zip' : 'gzip',
          },
        }
        await createBackup(payload)
        showMessage(`Backup "${createName.value || 'snapshot'}" created successfully`, 'success')
        await Promise.all([fetchBackups(), fetchStatistics()])
    } else if (createType.value === 'json') {
      await createJSONExport({ 
        output_path: exportOutputDir.value, 
        ...jsonOptions,
        compression: jsonCompression.value !== 'none',
        compression_algo: jsonCompression.value === 'zip' ? 'zip' : 'gzip',
      })
      showMessage('JSON export created successfully', 'success')
      await fetchContentExports()
    } else if (createType.value === 'yaml') {
      await createYAMLExport({ 
        output_path: exportOutputDir.value, 
        ...yamlOptions,
        compression: yamlCompression.value !== 'none',
        compression_algo: yamlCompression.value === 'zip' ? 'zip' : 'gzip',
      })
      showMessage('YAML export created successfully', 'success')
      await fetchContentExports()
    } else if (createType.value === 'sma') {
        await createSMAExport(
          { output_path: exportOutputDir.value, compression_level: smaOptions.compression_level, include_checksums: true },
          {
            include_discovered: smaOptions.include_discovered,
            include_network_settings: smaOptions.include_network_settings,
            include_plugin_configs: smaOptions.include_plugin_configs,
            include_system_settings: smaOptions.include_system_settings,
          },
          {}
        )
        showMessage('SMA export created successfully', 'success')
        await fetchContentExports()
      }
    }
  } catch (err: any) {
    createError2.value = err.message || 'Failed to create backup'
  } finally {
    createSubmitting.value = false
  }
}

function applyIntervalPreset() {
  if (schedulePreset.value) {
    scheduleInterval.value = schedulePreset.value
  }
}

/**
 * Poll backup result until completion
 */
function pollBackupResult(backupId: string) {
  const poll = async () => {
    try {
      // This would check the backup status
      // For now just refresh the list after a delay
      setTimeout(() => {
        fetchBackups()
      }, 2000)
    } catch (err) {
      console.error('Polling error:', err)
    }
  }
  
  poll()
}

/**
 * Download a backup file
 */
async function downloadBackup(backupId: string, backupName: string) {
  downloading.value = backupId
  
  try {
    const { blob, filename } = await downloadBackupWithName(backupId)
    
    // Create download link
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename || `${backupName}-${backupId}`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    
    showMessage('Backup downloaded successfully', 'success')
  } catch (err: any) {
    showMessage(err.message || 'Failed to download backup', 'error')
  } finally {
    downloading.value = ''
  }
}

/**
 * Start restore process
 */
function startRestore(backup: BackupItem) {
  restoreBackup.value = backup
  restoreOptions.backup_id = backup.backup_id
  restorePreview.value = null
  restoreError.value = ''
  showRestoreModal.value = true
}

/**
 * Preview restore changes
 */
async function previewRestore() {
  if (!restoreBackup.value) return
  
  restoreLoading.value = true
  restoreError.value = ''
  
  try {
    restorePreview.value = await previewRestore({
      ...restoreOptions,
      dry_run: true
    })
  } catch (err: any) {
    restoreError.value = err.message || 'Failed to preview restore'
  } finally {
    restoreLoading.value = false
  }
}

/**
 * Execute restore
 */
async function executeRestore() {
  if (!restoreBackup.value) return
  
  restoreLoading.value = true
  restoreError.value = ''
  
  try {
    const result = await executeRestore(restoreOptions)
    
    if (restoreOptions.dry_run) {
      showMessage('Restore preview completed successfully', 'success')
    } else {
      showMessage(`Restore executed successfully (ID: ${result.restore_id})`, 'success')
      closeRestoreModal()
      // Optionally refresh device data
    }
  } catch (err: any) {
    restoreError.value = err.message || 'Failed to execute restore'
  } finally {
    restoreLoading.value = false
  }
}

/**
 * Confirm backup deletion
 */
function confirmDelete(backup: BackupItem) {
  deleteConfirm.value = backup
}

/**
 * Perform backup deletion
 */
async function performDelete() {
  if (!deleteConfirm.value) return
  
  try {
    await deleteBackup(deleteConfirm.value.backup_id)
    showMessage(`Backup "${deleteConfirm.value.name}" deleted successfully`, 'success')
    deleteConfirm.value = null
    
    // Refresh the list
    await fetchBackups()
    await fetchStatistics()
  } catch (err: any) {
    showMessage(err.message || 'Failed to delete backup', 'error')
  }
}

/**
 * Close restore modal
 */
function closeRestoreModal() {
  showRestoreModal.value = false
  restoreBackup.value = null
  restorePreview.value = null
  restoreError.value = ''
}

/**
 * Show message
 */
function showMessage(text: string, type: 'success' | 'error') {
  message.text = text
  message.type = type
  
  // Auto-hide success messages
  if (type === 'success') {
    setTimeout(() => {
      if (message.text === text) {
        message.text = ''
      }
    }, 5000)
  }
}

/**
 * Format file size
 */
function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

/**
 * Format date
 */
  function formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString()
  }

  const route = useRoute()

  // Fetch provider info to label backup formats appropriately
  onMounted(async () => {
    try {
      const res = await api.get('/version')
      const data = res.data?.data
      if (data) {
        providerName.value = data.database_provider_name || ''
        providerVersion.value = data.database_provider_version || ''
      }
    } catch {}

    // Handle deep-linking to schedule creation
    const q = route.query as any
    if (q && (q.schedule === '1' || q.schedule === 'true')) {
      runMode.value = 'schedule'
      if (q.type && ['backup','json','yaml','sma'].includes(String(q.type))) {
        createType.value = q.type
      }
      scrollToCreate()
    }
  })

// Download a content export (JSON/YAML/SMA)
async function downloadContent(id: string) {
  try {
    const { blob, filename } = await downloadExportWithName(id)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename || `export-${id}`
    a.click()
    URL.revokeObjectURL(url)
    showMessage('Export downloaded successfully', 'success')
  } catch (err: any) {
    showMessage(err.message || 'Failed to download export', 'error')
  }
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
