<template>
  <main style="padding:16px">
    <div class="page-header">
      <h1>GitOps Export</h1>
      <button class="primary-button" @click="openCreateForm">🚀 Create Export</button>
    </div>

    <GitOpsStatistics :statistics="statistics" />

    <GitOpsFilterBar
      v-model:filters="filters"
      :loading="loading"
      @filter-change="reloadFromFirstPage"
      @refresh="refreshData"
    />

    <GitOpsExportList
      :exports="exports"
      :loading="loading"
      :error="error"
      @details="showDetails"
      @download="downloadZip"
      @delete="deleteConfirm = $event"
    />

    <PaginationBar
      v-if="meta?.pagination"
      :page="meta.pagination.page"
      :totalPages="meta.pagination.total_pages"
      :hasNext="meta.pagination.has_next"
      :hasPrev="meta.pagination.has_previous"
      @update:page="(p: number) => { currentPage = p; fetchExports() }"
    />

    <div v-if="showCreateForm" class="modal-overlay" @click="showCreateForm = false">
      <div class="modal-content" @click.stop>
        <GitOpsConfigForm
          :available-devices="availableDevices"
          :loading="createLoading"
          :error="createError"
          :preview="previewText"
          @submit="handleCreateExport"
          @preview="handlePreviewExport"
          @preview-stale="dropPreview"
          @cancel="showCreateForm = false"
        />
      </div>
    </div>

    <GitOpsPreviewModal
      :show="detailsItem !== null"
      :exportItem="detailsItem"
      :result="detailsResult"
      @close="detailsItem = null"
    />

    <div v-if="deleteConfirm" class="modal-overlay" @click="deleteConfirm = null">
      <div class="modal-content confirm-modal" @click.stop>
        <h3>Delete export record</h3>
        <p>Delete the record of <strong>{{ deleteConfirm.name || deleteConfirm.export_id }}</strong>?</p>
        <p class="hint">The exported files stay on the server where they were written.</p>
        <div class="modal-actions">
          <button class="secondary-button" @click="deleteConfirm = null">Cancel</button>
          <button class="danger-button" @click="performDelete">Delete record</button>
        </div>
      </div>
    </div>

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
  listExportHistory,
  getExportResult,
  countGitOpsExports,
  createGitOpsExport,
  deleteGitOpsExport,
  downloadGitOpsExportWithName,
  previewGitOpsExport,
  decodePreviewSample,
  type ExportHistoryItem,
  type ExportResult,
  type GitOpsExportOptions,
} from '@/api/export'
import { listDevices } from '@/api/devices'
import type { Device, Metadata } from '@/api/types'
import PaginationBar from '@/components/PaginationBar.vue'
import GitOpsConfigForm from '@/components/GitOpsConfigForm.vue'
import GitOpsStatistics from '@/components/gitops/GitOpsStatistics.vue'
import GitOpsFilterBar from '@/components/gitops/GitOpsFilterBar.vue'
import GitOpsExportList from '@/components/gitops/GitOpsExportList.vue'
import GitOpsPreviewModal from '@/components/gitops/GitOpsPreviewModal.vue'

const exports = ref<ExportHistoryItem[]>([])
const statistics = ref({ total: 0, success: 0, failure: 0 })
const availableDevices = ref<Device[]>([])
const meta = ref<Metadata>()
const loading = ref(false)
const { error: errorObj, setError, clearError } = useError()
const error = computed(() => errorObj.value?.message || '')

const filters = reactive({ success: undefined as boolean | undefined })
const currentPage = ref(1)
const pageSize = 20

const showCreateForm = ref(false)
const createLoading = ref(false)
const createError = ref('')
const previewText = ref('')
const deleteConfirm = ref<ExportHistoryItem | null>(null)
const detailsItem = ref<ExportHistoryItem | null>(null)
const detailsResult = ref<ExportResult | null>(null)
const message = reactive({ text: '', type: 'success' as 'success' | 'error' })

// Only the newest request of each kind may set its state: an older answer that arrives late
// (a filter changed, another row opened) is dropped.
let listRequest = 0
let detailsRequest = 0
let previewRequest = 0

onMounted(() => {
  fetchExports()
  fetchStatistics()
})

async function fetchExports() {
  const mine = ++listRequest
  loading.value = true
  clearError()
  try {
    const result = await listExportHistory({
      page: currentPage.value,
      pageSize,
      plugin: 'gitops',
      success: filters.success,
    })
    if (mine !== listRequest) return
    exports.value = result.items
    meta.value = result.meta
  } catch (err: any) {
    if (mine !== listRequest) return
    setError(err, { action: 'Loading GitOps exports', resource: 'GitOps Export' })
  } finally {
    if (mine === listRequest) loading.value = false
  }
}

async function fetchStatistics() {
  try {
    const [total, success] = await Promise.all([countGitOpsExports(), countGitOpsExports(true)])
    statistics.value = { total, success, failure: total - success }
  } catch (err) {
    console.error('Failed to load GitOps export counts:', err)
  }
}

async function openCreateForm() {
  previewRequest++
  previewText.value = ''
  createError.value = ''
  showCreateForm.value = true
  try {
    const all: Device[] = []
    for (let page = 1; ; page++) {
      const { items, meta } = await listDevices({ page, pageSize: 500 })
      all.push(...items)
      if (!meta?.pagination?.has_next) break
    }
    availableDevices.value = all
  } catch (err) {
    console.error('Failed to load devices:', err)
  }
}

function reloadFromFirstPage() {
  currentPage.value = 1
  fetchExports()
}

async function refreshData() {
  await Promise.all([fetchExports(), fetchStatistics()])
  showMessage('Data refreshed', 'success')
}

async function handleCreateExport(options: GitOpsExportOptions) {
  createLoading.value = true
  createError.value = ''
  try {
    await createGitOpsExport(options)
    showMessage(`GitOps export "${options.name}" created`, 'success')
    showCreateForm.value = false
    await Promise.all([fetchExports(), fetchStatistics()])
  } catch (err: any) {
    createError.value = err.message || 'Failed to create GitOps export'
  } finally {
    createLoading.value = false
  }
}

function dropPreview() {
  previewRequest++
  previewText.value = ''
}

async function handlePreviewExport(options: GitOpsExportOptions) {
  const mine = ++previewRequest
  createError.value = ''
  try {
    const { preview } = await previewGitOpsExport(options)
    if (mine === previewRequest) previewText.value = decodePreviewSample(preview.sample_data || '')
  } catch (err: any) {
    if (mine !== previewRequest) return
    previewText.value = ''
    createError.value = err.message || 'Failed to preview the export'
  }
}

async function showDetails(item: ExportHistoryItem) {
  const mine = ++detailsRequest
  try {
    const result = await getExportResult(item.export_id)
    if (mine !== detailsRequest) return
    detailsResult.value = result
    detailsItem.value = item
  } catch (err: any) {
    if (mine === detailsRequest) showMessage(err.message || 'Failed to load the export record', 'error')
  }
}

// The download route needs the admin key, which a plain link cannot send: fetch the zip through the
// API client and save the blob.
async function downloadZip(item: ExportHistoryItem) {
  try {
    const { blob, filename } = await downloadGitOpsExportWithName(item.export_id)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename || `${item.name || item.export_id}.zip`
    a.click()
    URL.revokeObjectURL(url)
  } catch (err: any) {
    showMessage(err.message || 'Failed to download the export', 'error')
  }
}

async function performDelete() {
  const item = deleteConfirm.value
  if (!item) return
  try {
    await deleteGitOpsExport(item.export_id)
    showMessage(`Record of "${item.name || item.export_id}" deleted`, 'success')
    deleteConfirm.value = null
    await Promise.all([fetchExports(), fetchStatistics()])
  } catch (err: any) {
    showMessage(err.message || 'Failed to delete the record', 'error')
  }
}

function showMessage(text: string, type: 'success' | 'error') {
  message.text = text
  message.type = type
  if (type === 'success') {
    setTimeout(() => {
      if (message.text === text) message.text = ''
    }, 5000)
  }
}
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
.page-header h1 { margin: 0; color: #1f2937; }
.primary-button { background-color: #3b82f6; color: white; border: none; padding: 10px 20px; border-radius: 6px; font-weight: 500; cursor: pointer; }
.primary-button:hover { background-color: #2563eb; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; padding: 16px; }
.modal-content { background: white; border-radius: 8px; max-width: 800px; width: 100%; max-height: 90vh; overflow: auto; }
.confirm-modal { padding: 24px; max-width: 400px; }
.confirm-modal h3 { margin: 0 0 16px 0; color: #1f2937; }
.hint { color: #6b7280; font-size: 0.875rem; }
.modal-actions { display: flex; gap: 12px; justify-content: flex-end; }
.secondary-button { background: #f3f4f6; color: #374151; border: 1px solid #d1d5db; padding: 8px 16px; border-radius: 4px; cursor: pointer; }
.danger-button { background: #dc2626; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer; }
.message { position: fixed; top: 20px; right: 20px; padding: 12px 16px; border-radius: 6px; display: flex; align-items: center; gap: 12px; z-index: 1001; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1); }
.message.success { background: #dcfce7; color: #166534; border: 1px solid #bbf7d0; }
.message.error { background: #fee2e2; color: #dc2626; border: 1px solid #fecaca; }
.message-close { background: none; border: none; color: inherit; cursor: pointer; font-size: 1.1rem; padding: 0; }
</style>
