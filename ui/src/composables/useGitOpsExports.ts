import { ref, reactive } from 'vue'
import {
  listGitOpsExports,
  getGitOpsExportStatistics,
  getGitOpsExportResult,
  createGitOpsExport,
  deleteGitOpsExport,
  downloadGitOpsExport,
  previewGitOpsExport,
  type GitOpsExportRequest,
  type GitOpsExportItem,
  type GitOpsExportStatistics,
  type GitOpsExportResult,
  type GitOpsIntegrationStatus,
} from '@/api/export'
import type { Device, Metadata } from '@/api/types'

export function useGitOpsExports() {
  const exports = ref<GitOpsExportItem[]>([])
  const statistics = ref<GitOpsExportStatistics>({ total: 0, success: 0, failure: 0, by_format: {}, by_structure: {}, total_files: 0, total_size: 0 })
  const integrationStatus = ref<GitOpsIntegrationStatus | null>(null)
  const availableDevices = ref<Device[]>([])
  const meta = ref<Metadata>()
  const loading = ref(false)
  const error = ref('')

  const filters = reactive({ format: '', success: undefined as boolean | undefined })
  const currentPage = ref(1)
  const pageSize = ref(20)

  const showCreateForm = ref(false)
  const showPreviewModal = ref(false)
  const createLoading = ref(false)
  const createError = ref('')
  const downloading = ref('')
  const deleteConfirm = ref<GitOpsExportItem | null>(null)
  const previewExportItem = ref<GitOpsExportItem | null>(null)
  const previewData = ref<GitOpsExportResult | null>(null)
  const message = reactive({ text: '', type: 'success' as 'success' | 'error' })

  async function fetchExports() {
    loading.value = true
    error.value = ''
    try {
      const result = await listGitOpsExports({
        page: currentPage.value,
        pageSize: pageSize.value,
        format: filters.format || undefined,
        success: filters.success,
      })
      exports.value = result.items
      meta.value = result.meta
    } catch (err: any) {
      error.value = err.message || 'Failed to load GitOps exports'
    } finally {
      loading.value = false
    }
  }

  async function fetchStatistics() {
    try { statistics.value = await getGitOpsExportStatistics() } catch (err) { console.error('Failed to load GitOps export statistics:', err) }
  }

  async function fetchIntegrationStatus() {
    try {
      integrationStatus.value = { repository_connected: true, branch_exists: true, webhook_configured: false, ci_status: 'passing', last_commit: 'abc123d' }
    } catch (err) { console.error('Failed to load integration status:', err) }
  }

  async function fetchAvailableDevices() {
    try { availableDevices.value = [] } catch (err) { console.error('Failed to load devices:', err) }
  }

  async function refreshData() {
    await Promise.all([fetchExports(), fetchStatistics(), fetchAvailableDevices(), fetchIntegrationStatus()])
    showMessage('Data refreshed successfully', 'success')
  }

  async function handleCreateExport(request: GitOpsExportRequest) {
    createLoading.value = true
    createError.value = ''
    try {
      const result = await createGitOpsExport(request)
      showMessage(`GitOps export "${request.name}" created successfully`, 'success')
      closeCreateModal()
      await fetchExports(); await fetchStatistics()
      pollExportResult(result.export_id)
    } catch (err: any) {
      createError.value = err.message || 'Failed to create GitOps export'
    } finally {
      createLoading.value = false
    }
  }

  async function handlePreviewExport(request: GitOpsExportRequest) {
    try {
      const result = await previewGitOpsExport(request)
      if (result.preview.success) showMessage('Export preview generated successfully', 'success')
      else showMessage('Export preview completed with warnings', 'error')
      // Could attach preview data if needed
      console.log('Preview result:', result)
    } catch (err: any) {
      showMessage(err.message || 'Failed to preview export', 'error')
    }
  }

  function pollExportResult(exportId: string) {
    setTimeout(() => { fetchExports() }, 2000)
  }

  async function previewExport(exportItem: GitOpsExportItem) {
    try {
      const result = await getGitOpsExportResult(exportItem.export_id)
      previewExportItem.value = exportItem
      previewData.value = result
      showPreviewModal.value = true
    } catch (err: any) {
      showMessage(err.message || 'Failed to load export preview', 'error')
    }
  }

  async function downloadExport(exportId: string, name?: string) {
    try {
      downloading.value = exportId
      const blob = await downloadGitOpsExport(exportId)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = name ? `${name}.zip` : `gitops-export-${exportId}.zip`
      a.click()
      URL.revokeObjectURL(url)
      showMessage('Export downloaded successfully', 'success')
    } catch (err: any) {
      showMessage(err.message || 'Failed to download export', 'error')
    } finally {
      downloading.value = ''
    }
  }

  function confirmDelete(exportItem: GitOpsExportItem) { deleteConfirm.value = exportItem }

  async function performDelete() {
    if (!deleteConfirm.value) return
    try {
      await deleteGitOpsExport(deleteConfirm.value.export_id)
      showMessage(`GitOps export "${deleteConfirm.value.name}" deleted successfully`, 'success')
      deleteConfirm.value = null
      await fetchExports(); await fetchStatistics()
    } catch (err: any) { showMessage(err.message || 'Failed to delete export', 'error') }
  }

  function closeCreateModal() { showCreateForm.value = false; createError.value = '' }
  function closePreviewModal() { showPreviewModal.value = false; previewExportItem.value = null; previewData.value = null }

  function showMessage(text: string, type: 'success' | 'error') {
    message.text = text; message.type = type
    if (type === 'success') setTimeout(() => { if (message.text === text) message.text = '' }, 5000)
  }

  return {
    // state
    exports, statistics, integrationStatus, availableDevices, meta, loading, error,
    filters, currentPage, pageSize, showCreateForm, showPreviewModal, createLoading, createError, downloading, deleteConfirm, previewExportItem, previewData, message,
    // actions
    fetchExports, fetchStatistics, fetchIntegrationStatus, fetchAvailableDevices, refreshData,
    handleCreateExport, handlePreviewExport, pollExportResult, previewExport, downloadExport, confirmDelete, performDelete,
    closeCreateModal, closePreviewModal, showMessage,
  }
}

