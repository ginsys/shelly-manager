import { ref, reactive } from 'vue'
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
  type RestorePreview,
} from '@/api/export'
import type { Device, Metadata } from '@/api/types'
import api from '@/api/client'

export function useBackups() {
  const backups = ref<BackupItem[]>([])
  const statistics = ref<BackupStatistics>({ total: 0, success: 0, failure: 0, total_size: 0, by_format: {} as any })
  const availableDevices = ref<Device[]>([])
  const contentExports = ref<any[]>([])
  const meta = ref<Metadata>()
  const loading = ref(false)
  const error = ref('')
  const downloading = ref('')
  const message = reactive({ text: '', type: 'success' as 'success' | 'error' })

  // Filters & pagination
  const filters = reactive<{ format: string; success: boolean | undefined }>({ format: '', success: undefined })
  const currentPage = ref(1)
  const pageSize = ref(20)

  // Create panel state
  const runMode = ref<'now' | 'schedule'>('now')
  const createType = ref<'backup' | 'json' | 'yaml' | 'sma'>('backup')
  const createName = ref('')
  const createDesc = ref('')
  const createCompression = ref<'none' | 'gzip' | 'zip'>('gzip')
  const createOutputDir = ref('./data/backups')
  const exportOutputDir = ref('./data/exports')
  const jsonOptions = reactive({ pretty: true, include_discovered: true })
  const yamlOptions = reactive({ include_discovered: true })
  const jsonCompression = ref<'none' | 'gzip' | 'zip'>('none')
  const yamlCompression = ref<'none' | 'gzip' | 'zip'>('none')
  const smaOptions = reactive({ compression_level: 6, include_discovered: true, include_network_settings: false, include_plugin_configs: true, include_system_settings: true, exclude_sensitive: true })
  const scheduleEnabled = ref(true)
  const scheduleInterval = ref('24 hours')
  const schedulePreset = ref('24 hours')
  const createSubmitting = ref(false)
  const createError2 = ref('')
  const providerName = ref('')
  const providerVersion = ref('')

  // Restore state
  const showRestoreModal = ref(false)
  const restoreBackup = ref<BackupItem | null>(null)
  const restoreOptions = reactive<RestoreRequest>({ backup_id: '', include_settings: true, include_schedules: true, include_metrics: false, dry_run: true })
  const restorePreview = ref<RestorePreview | null>(null)
  const restoreLoading = ref(false)
  const restoreError = ref('')
  const deleteConfirm = ref<BackupItem | null>(null)

  async function fetchBackups() {
    loading.value = true
    error.value = ''
    try {
      const result = await listBackups({ page: currentPage.value, pageSize: pageSize.value, format: filters.format || undefined, success: filters.success })
      backups.value = result.items
      meta.value = result.meta
    } catch (err: any) {
      error.value = err.message || 'Failed to load backups'
    } finally {
      loading.value = false
    }
  }

  async function fetchStatistics() {
    try { statistics.value = await getBackupStatistics() } catch (err) { console.error('Failed statistics:', err) }
  }

  async function fetchAvailableDevices() {
    try {
      const res = await api.get('/devices', { params: { page_size: 1000 } })
      const d = res.data?.data?.devices
      availableDevices.value = Array.isArray(d) ? d : []
    } catch { availableDevices.value = [] }
  }

  async function fetchProviderInfo() {
    try {
      const res = await api.get('/version')
      const data = res.data?.data
      providerName.value = data?.database_provider_name || ''
      providerVersion.value = data?.database_provider_version || ''
    } catch {}
  }

  async function fetchContentExports() {
    try {
      // Placeholder – this page composes lists from API routes used elsewhere
      contentExports.value = contentExports.value
    } catch (err) { console.error('Failed to fetch content exports:', err); contentExports.value = [] }
  }

  async function refreshData() {
    await Promise.all([fetchBackups(), fetchStatistics(), fetchAvailableDevices(), fetchProviderInfo(), fetchContentExports()])
    showMessage('Data refreshed successfully', 'success')
  }

  function applyIntervalPreset() {
    const m = schedulePreset.value
    if (!m) return
    scheduleInterval.value = m
  }

  async function createBackupPanel() {
    createSubmitting.value = true
    createError2.value = ''
    try {
      const base: any = { plugin_name: '', format: '', config: {}, filters: {}, options: {} }
      if (createType.value === 'backup') {
        base.plugin_name = 'backup'; base.format = 'sqlite'; base.config = { output_path: createOutputDir.value, compression: createCompression.value !== 'none', compression_algo: createCompression.value === 'zip' ? 'zip' : 'gzip', name: createName.value, description: createDesc.value }
        await createBackup(base.config as BackupRequest)
        showMessage(`Backup "${createName.value || 'snapshot'}" created successfully`, 'success')
      } else if (createType.value === 'json') {
        await createJSONExport({ output_path: exportOutputDir.value, ...jsonOptions }); showMessage('Export created successfully', 'success')
      } else if (createType.value === 'yaml') {
        await createYAMLExport({ output_path: exportOutputDir.value, ...yamlOptions }); showMessage('Export created successfully', 'success')
      } else {
        await createSMAExport({ output_path: exportOutputDir.value, compression_level: smaOptions.compression_level, include_checksums: true }, { include_discovered: smaOptions.include_discovered, include_network_settings: smaOptions.include_network_settings, include_plugin_configs: smaOptions.include_plugin_configs, include_system_settings: smaOptions.include_system_settings }, {})
        showMessage('Export created successfully', 'success')
      }
      await Promise.all([fetchBackups(), fetchStatistics(), fetchContentExports()])
    } catch (err: any) {
      createError2.value = err.message || 'Failed to create backup'
    } finally {
      createSubmitting.value = false
    }
  }

  function startRestore(backup: BackupItem) { restoreBackup.value = backup; showRestoreModal.value = true; restoreOptions.backup_id = backup.backup_id }

  async function previewRestoreAction() {
    if (!restoreBackup.value) return
    restoreLoading.value = true; restoreError.value = ''
    try { restorePreview.value = (await previewRestore(restoreOptions)).preview }
    catch (err: any) { restoreError.value = err.message || 'Failed to preview restore' }
    finally { restoreLoading.value = false }
  }

  async function executeRestoreAction() {
    if (!restoreBackup.value) return
    restoreLoading.value = true; restoreError.value = ''
    try { await executeRestore(restoreOptions); showMessage('Restore executed', 'success'); showRestoreModal.value = false; restoreBackup.value = null; restorePreview.value = null }
    catch (err: any) { restoreError.value = err.message || 'Failed to execute restore' }
    finally { restoreLoading.value = false }
  }

  function closeRestoreModal() { showRestoreModal.value = false; restoreBackup.value = null; restorePreview.value = null; restoreError.value = '' }

  async function downloadBackupAction(backupId: string, backupName: string) {
    try { const { blob, filename } = await downloadBackupWithName(backupId); const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = filename || `backup-${backupId}`; a.click(); URL.revokeObjectURL(url); showMessage('Backup downloaded successfully', 'success') } catch (err: any) { showMessage(err.message || 'Failed to download backup', 'error') }
  }

  async function downloadContentAction(id: string) {
    try { const { blob, filename } = await downloadExportWithName(id); const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = filename || `export-${id}`; a.click(); URL.revokeObjectURL(url); showMessage('Export downloaded successfully', 'success') } catch (err: any) { showMessage(err.message || 'Failed to download export', 'error') }
  }

  function confirmDelete(backup: BackupItem) { deleteConfirm.value = backup }
  async function performDelete() {
    if (!deleteConfirm.value) return
    try { await deleteBackup(deleteConfirm.value.backup_id); showMessage(`Backup "${deleteConfirm.value.name}" deleted successfully`, 'success'); await fetchBackups() } catch (err: any) { showMessage(err.message || 'Failed to delete backup', 'error') } finally { deleteConfirm.value = null }
  }

  function showMessage(text: string, type: 'success' | 'error') {
    message.text = text; message.type = type; if (type === 'success') setTimeout(() => { if (message.text === text) message.text = '' }, 5000)
  }

  return {
    backups, statistics, availableDevices, contentExports, meta, loading, error, downloading, message,
    filters, currentPage, pageSize,
    runMode, createType, createName, createDesc, createCompression, createOutputDir, exportOutputDir,
    jsonOptions, yamlOptions, jsonCompression, yamlCompression, smaOptions,
    scheduleEnabled, scheduleInterval, schedulePreset, createSubmitting, createError2, providerName, providerVersion,
    showRestoreModal, restoreBackup, restoreOptions, restorePreview, restoreLoading, restoreError, deleteConfirm,
    fetchBackups, fetchStatistics, fetchAvailableDevices, fetchContentExports, refreshData, applyIntervalPreset, createBackupPanel,
    startRestore, previewRestoreAction, executeRestoreAction, closeRestoreModal,
    downloadBackupAction, downloadContentAction, confirmDelete, performDelete, showMessage,
  }
}

