<template>
  <main style="padding:16px">
    <div class="page-header">
      <h1>GitOps Export</h1>
      <button class="primary-button" @click="showCreateForm = true">
        🚀 Create Export
      </button>
    </div>

    <!-- Export Statistics -->
    <section class="stats-section">
      <div class="stats">
        <div class="card">
          <span class="stat-label">Total:</span>
          <span class="stat-value">{{ statistics.total || 0 }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Success:</span>
          <span class="stat-value success">{{ statistics.success || 0 }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Failed:</span>
          <span class="stat-value failure">{{ statistics.failure || 0 }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Files:</span>
          <span class="stat-value">{{ statistics.total_files || 0 }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Total Size:</span>
          <span class="stat-value">{{ formatFileSize(statistics.total_size || 0) }}</span>
        </div>
      </div>
    </section>

    <!-- Integration Status Dashboard -->
    <section class="integration-status" v-if="integrationStatus">
      <h3>Git Integration Status</h3>
      <div class="status-grid">
        <div class="status-item" :class="{ connected: integrationStatus.repository_connected }">
          <span class="status-icon">{{ integrationStatus.repository_connected ? '✅' : '❌' }}</span>
          <span>Repository Connection</span>
        </div>
        <div class="status-item" :class="{ connected: integrationStatus.branch_exists }">
          <span class="status-icon">{{ integrationStatus.branch_exists ? '✅' : '❌' }}</span>
          <span>Branch Available</span>
        </div>
        <div class="status-item" :class="{ connected: integrationStatus.webhook_configured }">
          <span class="status-icon">{{ integrationStatus.webhook_configured ? '✅' : '❌' }}</span>
          <span>Webhook Configured</span>
        </div>
        <div class="status-item" :class="{ 
          connected: integrationStatus.ci_status === 'passing',
          warning: integrationStatus.ci_status === 'failing'
        }">
          <span class="status-icon">
            {{ integrationStatus.ci_status === 'passing' ? '✅' : 
               integrationStatus.ci_status === 'failing' ? '❌' : '❓' }}
          </span>
          <span>CI Status: {{ integrationStatus.ci_status || 'Unknown' }}</span>
        </div>
      </div>
    </section>

    <!-- Filters -->
    <div class="filters-section">
      <div class="filter-row">
        <div class="filter-group">
          <label class="filter-label">Format:</label>
          <select v-model="filters.format" @change="fetchExports" class="filter-select">
            <option value="">All formats</option>
            <option value="terraform">Terraform</option>
            <option value="ansible">Ansible</option>
            <option value="kubernetes">Kubernetes</option>
            <option value="docker-compose">Docker Compose</option>
            <option value="yaml">YAML</option>
          </select>
        </div>
        <div class="filter-group">
          <label class="filter-label">Status:</label>
          <select v-model="filters.success" @change="fetchExports" class="filter-select">
            <option :value="undefined">All statuses</option>
            <option :value="true">Success only</option>
            <option :value="false">Failed only</option>
          </select>
        </div>
        <div class="filter-actions">
          <button @click="refreshData" class="refresh-button" :disabled="loading">
            🔄 Refresh
          </button>
        </div>
      </div>
    </div>

    <!-- GitOps Exports Table -->
    <DataTable
      :rows="exports"
      :loading="loading"
      :error="error"
      :cols="8"
      :rowKey="(row: any) => row.export_id"
    >
      <template #header>
        <th>Name</th>
        <th>Format</th>
        <th>Structure</th>
        <th>Files</th>
        <th>Size</th>
        <th>Status</th>
        <th>Created</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>
          <div class="export-name">
            <strong>{{ row.name }}</strong>
            <div class="export-description" v-if="row.description">{{ row.description }}</div>
            <div class="export-id">ID: {{ row.export_id }}</div>
          </div>
        </td>
        <td>
          <span class="format-badge" :class="formatClass(row.format)">{{ formatLabel(row.format) }}</span>
        </td>
        <td>
          <span class="structure-badge">{{ structureLabel(row.repository_structure) }}</span>
        </td>
        <td>
          <div class="file-count">
            {{ row.file_count }} files
            <div class="device-count">{{ row.device_count }} devices</div>
          </div>
        </td>
        <td>
          <div v-if="row.total_size" class="file-size">
            {{ formatFileSize(row.total_size) }}
          </div>
          <span v-else class="no-data">—</span>
        </td>
        <td>
          <span :class="['status-badge', row.success ? 'success' : 'failure']">
            {{ row.success ? 'Success' : 'Failed' }}
          </span>
          <div v-if="!row.success && row.error_message" class="error-message">
            {{ row.error_message }}
          </div>
        </td>
        <td>
          <div class="time-info">
            {{ formatDate(row.created_at) }}
            <div class="created-by" v-if="row.created_by">
              by {{ row.created_by }}
            </div>
          </div>
        </td>
        <td>
          <div class="action-buttons">
            <button 
              v-if="row.success"
              class="action-btn preview-btn" 
              @click="previewExport(row)"
              title="Preview structure"
            >
              👁️
            </button>
            <button 
              v-if="row.success"
              class="action-btn download-btn" 
              @click="downloadExport(row.export_id, row.name)"
              :disabled="downloading === row.export_id"
              title="Download export"
            >
              <span v-if="downloading === row.export_id">⏳</span>
              <span v-else>📥</span>
            </button>
            <button 
              class="action-btn delete-btn" 
              @click="confirmDelete(row)"
              title="Delete export"
            >
              🗑️
            </button>
          </div>
        </td>
      </template>
    </DataTable>

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
    <div v-if="showPreviewModal" class="modal-overlay" @click="closePreviewModal">
      <div class="modal-content preview-modal" @click.stop>
        <div class="modal-header">
          <h3>Export Preview: {{ previewExportItem?.name }}</h3>
          <button class="close-button" @click="closePreviewModal">✖</button>
        </div>
        
        <div class="preview-content" v-if="previewData">
          <div class="preview-summary">
            <div class="summary-item">
              <strong>Format:</strong> {{ formatLabel(previewExportItem?.format || '') }}
            </div>
            <div class="summary-item">
              <strong>Structure:</strong> {{ structureLabel(previewExportItem?.repository_structure || '') }}
            </div>
            <div class="summary-item">
              <strong>Files:</strong> {{ previewData.file_count }} files
            </div>
            <div class="summary-item">
              <strong>Size:</strong> {{ formatFileSize(previewData.total_size) }}
            </div>
          </div>

          <div v-if="previewData.files?.length" class="file-structure">
            <h4>File Structure</h4>
            <div class="file-tree">
              <div 
                v-for="file in previewData.files" 
                :key="file.path" 
                class="file-item"
                :class="'file-type-' + file.type"
              >
                <span class="file-icon">{{ getFileIcon(file.type) }}</span>
                <span class="file-path">{{ file.path }}</span>
                <span class="file-size">{{ formatFileSize(file.size) }}</span>
                <div v-if="file.description" class="file-description">{{ file.description }}</div>
              </div>
            </div>
          </div>

          <div v-if="previewData.git_integration" class="git-integration">
            <h4>Git Integration</h4>
            <div class="integration-details">
              <div class="integration-item" :class="{ active: previewData.git_integration.repository_connected }">
                <span class="integration-icon">
                  {{ previewData.git_integration.repository_connected ? '🔗' : '🔌' }}
                </span>
                Repository: {{ previewData.git_integration.repository_connected ? 'Connected' : 'Not Connected' }}
              </div>
              <div v-if="previewData.git_integration.last_commit" class="integration-item">
                <span class="integration-icon">📝</span>
                Last Commit: {{ previewData.git_integration.last_commit }}
              </div>
            </div>
          </div>

          <div v-if="previewData.warnings?.length" class="warnings-section">
            <h4>⚠️ Warnings</h4>
            <ul class="warnings-list">
              <li v-for="warning in previewData.warnings" :key="warning" class="warning-item">
                {{ warning }}
              </li>
            </ul>
          </div>
        </div>

        <div class="modal-actions">
          <button class="secondary-button" @click="closePreviewModal">Close</button>
          <button 
            v-if="previewExportItem?.success"
            class="primary-button" 
            @click="downloadExport(previewExportItem.export_id, previewExportItem.name)"
          >
            📥 Download
          </button>
        </div>
      </div>
    </div>

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
import { onMounted, ref, reactive } from 'vue'
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
import DataTable from '@/components/DataTable.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import GitOpsConfigForm from '@/components/GitOpsConfigForm.vue'

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
const error = ref('')

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
  error.value = ''
  
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
    error.value = err.message || 'Failed to load GitOps exports'
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

/**
 * Get format label
 */
function formatLabel(format: string): string {
  const labels: Record<string, string> = {
    terraform: 'Terraform',
    ansible: 'Ansible',
    kubernetes: 'Kubernetes',
    'docker-compose': 'Docker Compose',
    yaml: 'YAML'
  }
  return labels[format] || format.toUpperCase()
}

/**
 * Get format CSS class
 */
function formatClass(format: string): string {
  return `format-${format.replace(/[^a-zA-Z0-9]/g, '-').toLowerCase()}`
}

/**
 * Get structure label
 */
function structureLabel(structure: string): string {
  const labels: Record<string, string> = {
    monorepo: 'Monorepo',
    hierarchical: 'Hierarchical',
    'per-device': 'Per Device',
    flat: 'Flat'
  }
  return labels[structure] || structure
}

/**
 * Get file type icon
 */
function getFileIcon(type: string): string {
  const icons: Record<string, string> = {
    config: '⚙️',
    template: '📋',
    variable: '🔢',
    readme: '📖',
    script: '📜'
  }
  return icons[type] || '📄'
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

.integration-status {
  margin-bottom: 24px;
  padding: 20px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.integration-status h3 {
  margin: 0 0 16px 0;
  color: #1f2937;
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-size: 0.875rem;
}

.status-item.connected {
  border-color: #10b981;
  background: #ecfdf5;
  color: #065f46;
}

.status-item.warning {
  border-color: #f59e0b;
  background: #fffbeb;
  color: #92400e;
}

.status-icon {
  font-size: 1rem;
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

.export-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.export-description {
  font-size: 0.75rem;
  color: #6b7280;
  font-style: italic;
}

.export-id {
  font-size: 0.75rem;
  color: #6b7280;
  font-family: monospace;
}

.format-badge {
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.format-terraform {
  background: #ede9fe;
  color: #6b21a8;
}

.format-ansible {
  background: #fef3c7;
  color: #92400e;
}

.format-kubernetes {
  background: #dbeafe;
  color: #1e40af;
}

.format-docker-compose {
  background: #dcfce7;
  color: #166534;
}

.format-yaml {
  background: #f3f4f6;
  color: #374151;
}

.structure-badge {
  background: #e5e7eb;
  color: #374151;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.file-count {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.device-count {
  font-size: 0.75rem;
  color: #6b7280;
}

.file-size {
  font-weight: 500;
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

.preview-btn:hover:not(:disabled) {
  background: #dbeafe;
  border-color: #3b82f6;
}

.download-btn:hover:not(:disabled) {
  background: #dcfce7;
  border-color: #10b981;
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
  max-width: 900px;
  width: 100%;
  max-height: 90vh;
  overflow: auto;
}

.create-modal {
  max-width: 800px;
}

.preview-modal {
  max-width: 700px;
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

.preview-content {
  padding: 24px;
}

.preview-summary {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  margin-bottom: 24px;
}

.summary-item {
  padding: 12px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-size: 0.875rem;
}

.file-structure {
  margin-bottom: 24px;
}

.file-structure h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
}

.file-tree {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 16px;
  max-height: 300px;
  overflow-y: auto;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid #f1f5f9;
  font-size: 0.875rem;
}

.file-item:last-child {
  border-bottom: none;
}

.file-icon {
  font-size: 1rem;
  width: 20px;
  text-align: center;
}

.file-path {
  flex: 1;
  font-family: monospace;
}

.file-size {
  color: #6b7280;
  font-size: 0.75rem;
}

.file-description {
  font-size: 0.75rem;
  color: #6b7280;
  font-style: italic;
  margin-left: 28px;
}

.git-integration {
  margin-bottom: 24px;
}

.git-integration h4 {
  margin: 0 0 12px 0;
  color: #1f2937;
}

.integration-details {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.integration-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  font-size: 0.875rem;
}

.integration-item.active {
  background: #ecfdf5;
  border-color: #10b981;
  color: #065f46;
}

.integration-icon {
  font-size: 1rem;
}

.warnings-section {
  margin-bottom: 20px;
  padding: 16px;
  background: #fffbeb;
  border: 1px solid #fed7aa;
  border-radius: 6px;
}

.warnings-section h4 {
  margin: 0 0 12px 0;
  color: #d97706;
}

.warnings-list {
  margin: 0;
  padding-left: 20px;
}

.warning-item {
  color: #92400e;
  margin-bottom: 4px;
  font-size: 0.875rem;
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

  .status-grid {
    grid-template-columns: 1fr;
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

  .preview-summary {
    grid-template-columns: 1fr;
  }
}
</style>