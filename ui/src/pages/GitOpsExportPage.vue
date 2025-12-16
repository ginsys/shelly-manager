<template>
  <main style="padding:16px">
    <div class="page-header">
      <h1>GitOps Export</h1>
      <button class="primary-button" @click="showCreateForm = true">
        🚀 Create Export
      </button>
    </div>

    <!-- Export Statistics -->
    <GitOpsStatistics :statistics="statistics" />

    <!-- Integration Status Dashboard -->
    <GitOpsIntegrationStatusComponent :status="integrationStatus" />

    <!-- Filters -->
    <GitOpsFilterBar
      v-model:filters="filters"
      :loading="loading"
      @filter-change="fetchExports"
      @refresh="refreshData"
    />

    <!-- GitOps Exports Table -->
    <GitOpsExportList
      :exports="exports"
      :loading="loading"
      :error="error"
      :downloading="downloading"
      @preview="previewExport"
      @download="downloadExport"
      @delete="confirmDelete"
    />

    <!-- Pagination -->
    <PaginationBar
      v-if="meta?.pagination"
      :page="meta.pagination.page"
      :totalPages="meta.pagination.total_pages"
      :hasNext="meta.pagination.has_next"
      :hasPrev="meta.pagination.has_previous"
      @update:page="(p: number) => { currentPage = p; fetchExports() }"
    />

    <!-- Create GitOps Export Form Modal -->
    <div v-if="showCreateForm" class="modal-overlay" @click="closeCreateModal">
      <div class="modal-content create-modal" @click.stop>
        <GitOpsConfigForm
          :available-devices="availableDevices"
          :loading="createLoading"
          :error="createError"
          @submit="handleCreateExport"
          @preview="handlePreviewExport"
          @cancel="closeCreateModal"
        />
      </div>
    </div>

    <!-- Preview Modal -->
    <GitOpsPreviewModal
      :show="showPreviewModal"
      :exportItem="previewExportItem"
      :previewData="previewData"
      @close="closePreviewModal"
      @download="downloadExport"
    />

    <!-- Delete Confirmation Modal -->
    <div v-if="deleteConfirm" class="modal-overlay" @click="deleteConfirm = null">
      <div class="modal-content confirm-modal" @click.stop>
        <h3>Confirm Delete</h3>
        <p>Are you sure you want to delete GitOps export <strong>{{ deleteConfirm.name }}</strong>?</p>
        <p class="warning">This action cannot be undone.</p>
        <div class="modal-actions">
          <button class="secondary-button" @click="deleteConfirm = null">Cancel</button>
          <button class="danger-button" @click="performDelete">Delete Export</button>
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
import { onMounted, ref, reactive, computed } from 'vue'
import { useError } from '@/composables/useError'
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
  type GitOpsExportPreview,
  type GitOpsIntegrationStatus
} from '@/api/export'
import type { Device, Metadata } from '@/api/types'
import PaginationBar from '@/components/PaginationBar.vue'
import GitOpsConfigForm from '@/components/GitOpsConfigForm.vue'
import GitOpsStatistics from '@/components/gitops/GitOpsStatistics.vue'
import GitOpsIntegrationStatusComponent from '@/components/gitops/GitOpsIntegrationStatus.vue'
import GitOpsFilterBar from '@/components/gitops/GitOpsFilterBar.vue'
import GitOpsExportList from '@/components/gitops/GitOpsExportList.vue'
import GitOpsPreviewModal from '@/components/gitops/GitOpsPreviewModal.vue'

// State
const exports = ref<GitOpsExportItem[]>([])
const statistics = ref<GitOpsExportStatistics>({
  total: 0,
  success: 0,
  failure: 0,
  by_format: {},
  by_structure: {},
  total_files: 0,
  total_size: 0
})
const integrationStatus = ref<GitOpsIntegrationStatus | null>(null)
const availableDevices = ref<Device[]>([])
const meta = ref<Metadata>()
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
const showCreateForm = ref(false)
const showPreviewModal = ref(false)
const createLoading = ref(false)
const createError = ref('')
const downloading = ref('')
const deleteConfirm = ref<GitOpsExportItem | null>(null)
const previewExportItem = ref<GitOpsExportItem | null>(null)
const previewData = ref<GitOpsExportResult | null>(null)
const message = reactive({ 
  text: '', 
  type: 'success' as 'success' | 'error' 
})

// Initialize
onMounted(() => {
  fetchExports()
  fetchStatistics()
  fetchAvailableDevices()
  fetchIntegrationStatus()
})

/**
 * Fetch GitOps exports list with current filters
 */
async function fetchExports() {
  loading.value = true
  clearError()

  try {
    const result = await listGitOpsExports({
      page: currentPage.value,
      pageSize: pageSize.value,
      format: filters.format || undefined,
      success: filters.success
    })

    exports.value = result.items
    meta.value = result.meta
  } catch (err: any) {
    setError(err, { action: 'Loading GitOps exports', resource: 'GitOps Export' })
  } finally {
    loading.value = false
  }
}

/**
 * Fetch GitOps export statistics
 */
async function fetchStatistics() {
  try {
    statistics.value = await getGitOpsExportStatistics()
  } catch (err) {
    console.error('Failed to load GitOps export statistics:', err)
  }
}

/**
 * Fetch integration status (mock implementation)
 */
async function fetchIntegrationStatus() {
  try {
    // This would normally come from an API endpoint
    // For now, using mock data
    integrationStatus.value = {
      repository_connected: true,
      branch_exists: true,
      webhook_configured: false,
      ci_status: 'passing',
      last_commit: 'abc123d'
    }
  } catch (err) {
    console.error('Failed to load integration status:', err)
  }
}

/**
 * Fetch available devices for export selection
 */
async function fetchAvailableDevices() {
  try {
    // This would normally come from a devices API
    // For now, using mock data consistent with other components
    availableDevices.value = [
      { id: 1, ip: '192.168.1.100', mac: 'aa:bb:cc:dd:ee:01', type: 'shelly1', name: 'Living Room Switch', firmware: '1.14.0', status: 'online', last_seen: new Date().toISOString() },
      { id: 2, ip: '192.168.1.101', mac: 'aa:bb:cc:dd:ee:02', type: 'shelly25', name: 'Kitchen Roller', firmware: '1.14.0', status: 'online', last_seen: new Date().toISOString() },
      { id: 3, ip: '192.168.1.102', mac: 'aa:bb:cc:dd:ee:03', type: 'shellyplug', name: 'Office Plug', firmware: '1.14.0', status: 'offline', last_seen: new Date().toISOString() }
    ]
  } catch (err) {
    console.error('Failed to load devices:', err)
  }
}

/**
 * Refresh all data
 */
async function refreshData() {
  await Promise.all([
    fetchExports(),
    fetchStatistics(),
    fetchAvailableDevices(),
    fetchIntegrationStatus()
  ])
  showMessage('Data refreshed successfully', 'success')
}

/**
 * Handle GitOps export creation
 */
async function handleCreateExport(request: GitOpsExportRequest) {
  createLoading.value = true
  createError.value = ''
  
  try {
    const result = await createGitOpsExport(request)
    showMessage(`GitOps export "${request.name}" created successfully`, 'success')
    closeCreateModal()
    
    // Refresh the list to show new export
    await fetchExports()
    await fetchStatistics()
    
    // Optionally start polling for the export result
    pollExportResult(result.export_id)
  } catch (err: any) {
    createError.value = err.message || 'Failed to create GitOps export'
  } finally {
    createLoading.value = false
  }
}

/**
 * Handle export preview from config form
 */
async function handlePreviewExport(request: GitOpsExportRequest) {
  try {
    const result = await previewGitOpsExport(request)
    
    if (result.preview.success) {
      showMessage('Export preview generated successfully', 'success')
    } else {
      showMessage('Export preview completed with warnings', 'error')
    }
    
    // You could show a preview modal here with the result
    console.log('Preview result:', result)
  } catch (err: any) {
    showMessage(err.message || 'Failed to preview export', 'error')
  }
}

/**
 * Poll export result until completion
 */
function pollExportResult(exportId: string) {
  const poll = async () => {
    try {
      // This would check the export status
      // For now just refresh the list after a delay
      setTimeout(() => {
        fetchExports()
      }, 2000)
    } catch (err) {
      console.error('Polling error:', err)
    }
  }
  
  poll()
}

/**
 * Download an export file
 */
async function downloadExport(exportId: string, exportName: string) {
  downloading.value = exportId
  
  try {
    const blob = await downloadGitOpsExport(exportId)
    
    // Create download link
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${exportName}-${exportId}.zip`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    
    showMessage('GitOps export downloaded successfully', 'success')
  } catch (err: any) {
    showMessage(err.message || 'Failed to download export', 'error')
  } finally {
    downloading.value = ''
  }
}

/**
 * Preview export details
 */
async function previewExport(exportItem: GitOpsExportItem) {
  try {
    previewExportItem.value = exportItem
    previewData.value = await getGitOpsExportResult(exportItem.export_id)
    showPreviewModal.value = true
  } catch (err: any) {
    showMessage(err.message || 'Failed to load export details', 'error')
  }
}

/**
 * Confirm export deletion
 */
function confirmDelete(exportItem: GitOpsExportItem) {
  deleteConfirm.value = exportItem
}

/**
 * Perform export deletion
 */
async function performDelete() {
  if (!deleteConfirm.value) return
  
  try {
    await deleteGitOpsExport(deleteConfirm.value.export_id)
    showMessage(`GitOps export "${deleteConfirm.value.name}" deleted successfully`, 'success')
    deleteConfirm.value = null
    
    // Refresh the list
    await fetchExports()
    await fetchStatistics()
  } catch (err: any) {
    showMessage(err.message || 'Failed to delete export', 'error')
  }
}

/**
 * Close create modal
 */
function closeCreateModal() {
  showCreateForm.value = false
  createError.value = ''
}

/**
 * Close preview modal
 */
function closePreviewModal() {
  showPreviewModal.value = false
  previewExportItem.value = null
  previewData.value = null
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
  max-width: 900px;
  width: 100%;
  max-height: 90vh;
  overflow: auto;
}

.create-modal {
  max-width: 800px;
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

.modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
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

  .modal-content {
    margin: 8px;
  }
}
</style>