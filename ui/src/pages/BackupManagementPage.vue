<template>
  <main style="padding:16px" data-testid="backup-management-page">
    <div class="page-header">
      <h1 data-testid="page-title">Backup Management</h1>
    </div>

    <!-- In-page Create Backup Panel -->
    <section id="create-backup" class="create-section">
      <h2>Create Backup or Content Export</h2>
      <div class="grid-2">
        <div class="form-field">
          <label class="field-label">Create Type</label>
          <select v-model="createType" class="form-select">
            <option value="backup">Backup (Provider Snapshot)</option>
            <option value="json">Content Export: JSON</option>
            <option value="yaml">Content Export: YAML</option>
            <option value="sma">Content Export: SMA</option>
          </select>
        </div>
        <div class="form-field">
          <label class="field-label">Run Mode</label>
          <select v-model="runMode" class="form-select">
            <option value="now">Run Now</option>
            <option value="schedule">Schedule</option>
          </select>
        </div>
      </div>
      <div class="grid-2">
        <div class="form-field">
          <label class="field-label">Name</label>
          <input v-model="createName" class="form-input" placeholder="e.g. Pre-maintenance snapshot" />
        </div>
        <div class="form-field">
          <label class="field-label">Description</label>
          <input v-model="createDesc" class="form-input" placeholder="Optional description" />
        </div>
      </div>
      <!-- Backup options -->
      <div class="grid-2" v-if="createType === 'backup'">
        <div class="form-field">
          <label class="field-label">Compression</label>
          <select v-model="createCompression" class="form-select">
            <option value="none">None</option>
            <option value="gzip">Gzip</option>
            <option value="zip">Zip</option>
          </select>
          <div class="field-help" v-if="providerName">
            Database: {{ providerName }} {{ providerVersion }}
          </div>
        </div>
        <div class="form-field">
          <label class="field-label">Output Directory</label>
          <input v-model="createOutputDir" class="form-input" placeholder="./data/backups" />
        </div>
      </div>

      <!-- Schedule options -->
      <div class="grid-2" v-if="runMode === 'schedule'">
        <div class="form-field">
          <label class="field-label">Schedule Interval</label>
          <select v-model="schedulePreset" class="form-select" @change="applyIntervalPreset">
            <option value="">Custom…</option>
            <option value="15 minutes">Every 15 minutes</option>
            <option value="1 hour">Every hour</option>
            <option value="6 hours">Every 6 hours</option>
            <option value="24 hours">Daily</option>
          </select>
          <input v-model="scheduleInterval" class="form-input" placeholder="e.g. 1 hour, 24 hours" style="margin-top:8px" />
          <div class="field-help">Use format like "15 minutes", "1 hour", or "1 day".</div>
        </div>
        <div class="form-field">
          <label class="field-label">Enabled</label>
          <select v-model="scheduleEnabled" class="form-select">
            <option :value="true">Enabled</option>
            <option :value="false">Disabled</option>
          </select>
        </div>
      </div>

      <!-- Content export options -->
      <div class="grid-2" v-if="createType === 'json'">
        <div class="form-field">
          <label class="field-label">JSON Options</label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="jsonOptions.pretty" />
            <span>Pretty-print JSON</span>
          </label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="jsonOptions.include_discovered" />
            <span>Include discovered devices</span>
          </label>
          <label class="field-label" style="margin-top:8px">Compression</label>
          <select v-model="jsonCompression" class="form-select">
            <option value="none">None</option>
            <option value="gzip">Gzip</option>
            <option value="zip">Zip</option>
          </select>
        </div>
        <div class="form-field">
          <label class="field-label">Output Directory</label>
          <input v-model="exportOutputDir" class="form-input" placeholder="./data/exports" />
        </div>
      </div>
      <div class="grid-2" v-if="createType === 'yaml'">
        <div class="form-field">
          <label class="field-label">YAML Options</label>
          <label class="checkbox-label">
            <input type="checkbox" v-model="yamlOptions.include_discovered" />
            <span>Include discovered devices</span>
          </label>
          <label class="field-label" style="margin-top:8px">Compression</label>
          <select v-model="yamlCompression" class="form-select">
            <option value="none">None</option>
            <option value="gzip">Gzip</option>
            <option value="zip">Zip</option>
          </select>
        </div>
        <div class="form-field">
          <label class="field-label">Output Directory</label>
          <input v-model="exportOutputDir" class="form-input" placeholder="./data/exports" />
        </div>
      </div>
      <div class="grid-2" v-if="createType === 'sma'">
        <div class="form-field">
          <label class="field-label">SMA Options</label>
          <div class="grid-2">
            <div>
              <label class="field-label">Compression level (1-9)</label>
              <input class="form-input" type="number" min="1" max="9" v-model.number="smaOptions.compression_level" />
            </div>
          </div>
          <div class="grid-2">
            <label class="checkbox-label">
              <input type="checkbox" v-model="smaOptions.include_discovered" />
              <span>Include discovered devices</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" v-model="smaOptions.include_network_settings" />
              <span>Include network settings</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" v-model="smaOptions.include_plugin_configs" />
              <span>Include plugin configurations</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" v-model="smaOptions.include_system_settings" />
              <span>Include system settings</span>
            </label>
            <label class="checkbox-label">
              <input type="checkbox" v-model="smaOptions.exclude_sensitive" />
              <span>Exclude sensitive data</span>
            </label>
          </div>
          <div class="field-help">SMA exports are compressed archives with integrity data suitable for full content migration.</div>
        </div>
        <div class="form-field">
          <label class="field-label">Output Directory</label>
          <input v-model="exportOutputDir" class="form-input" placeholder="./data/exports" />
        </div>
      </div>
      <div class="form-actions">
        <button class="primary-button" :disabled="createSubmitting" @click="createBackupPanel">
          {{ createSubmitting ? 'Creating...' : 'Create' }}
        </button>
        <span v-if="createError2" class="form-error" style="margin-left:12px"><strong>Error:</strong> {{ createError2 }}</span>
      </div>
    </section>

    <!-- Backup Statistics -->
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
          <span class="stat-label">Total Size:</span> 
          <span class="stat-value">{{ formatFileSize(statistics.total_size || 0) }}</span>
        </div>
      </div>
    </section>

    <!-- Filters -->
    <div class="filters-section">
      <div class="filter-row">
        <div class="filter-group">
          <label class="filter-label">Format:</label>
          <select v-model="filters.format" @change="fetchBackups" class="filter-select">
            <option value="">All formats</option>
            <option value="json">JSON</option>
            <option value="sma">SMA</option>
            <option value="yaml">YAML</option>
          </select>
        </div>
        <div class="filter-group">
          <label class="filter-label">Status:</label>
          <select v-model="filters.success" @change="fetchBackups" class="filter-select">
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

    <!-- Backups Table -->
    <DataTable
      :rows="backups"
      :loading="loading"
      :error="error"
      :cols="8"
      :rowKey="(row: any) => row.backup_id"
    >
      <template #header>
        <th>Name</th>
        <th>Format</th>
        <th>Devices</th>
        <th>Size</th>
        <th>Status</th>
        <th>Encrypted</th>
        <th>Created</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>
          <div class="backup-name">
            <strong>{{ row.name }}</strong>
            <div class="backup-description" v-if="row.description">{{ row.description }}</div>
            <div class="backup-id">ID: {{ row.backup_id }}</div>
          </div>
        </td>
        <td>
          <span class="format-badge">{{ row.format.toUpperCase() }}</span>
        </td>
        <td>{{ row.device_count }}</td>
        <td>
          <div v-if="row.file_size" class="file-size">
            {{ formatFileSize(row.file_size) }}
            <div class="checksum" v-if="row.checksum">
              {{ row.checksum.substring(0, 8) }}...
            </div>
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
          <span :class="['encryption-badge', row.encrypted ? 'encrypted' : 'plain']">
            {{ row.encrypted ? '🔒 Yes' : '🔓 No' }}
          </span>
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
              class="action-btn download-btn" 
              @click="downloadBackup(row.backup_id, row.name)"
              :disabled="downloading === row.backup_id"
              title="Download backup"
            >
              <span v-if="downloading === row.backup_id">⏳ Downloading...</span>
              <span v-else>⬇ Download</span>
            </button>
            <button 
              v-if="row.success"
              class="action-btn restore-btn" 
              @click="startRestore(row)"
              title="Restore from backup"
            >
              ↩ Restore
            </button>
            <button 
              class="action-btn delete-btn" 
              @click="confirmDelete(row)"
              title="Delete backup"
            >
              🗑️
            </button>
          </div>
        </td>
      </template>
    </DataTable>

    <!-- Content Exports Table -->
    <section style="margin-top:24px">
      <h2>Content Exports (JSON / YAML / SMA)</h2>
      <DataTable
        :rows="contentExports"
        :loading="false"
        :error="''"
        :cols="6"
        :rowKey="(row: any) => row.export_id"
      >
        <template #header>
          <th>Plugin</th>
          <th>Format</th>
          <th>Records</th>
          <th>Size</th>
          <th>Created</th>
          <th>Actions</th>
        </template>
        <template #row="{ row }">
          <td>{{ row.plugin_name }}</td>
          <td><span class="format-badge">{{ row.format?.toUpperCase?.() || row.plugin_name?.toUpperCase?.() }}</span></td>
          <td>{{ row.record_count ?? '—' }}</td>
          <td>
            <span v-if="row.file_size">{{ formatFileSize(row.file_size) }}</span>
            <span v-else class="no-data">—</span>
          </td>
          <td>{{ formatDate(row.created_at) }}</td>
          <td>
            <button class="action-btn download-btn" @click="downloadContent(row.export_id)">⬇ Download</button>
          </td>
        </template>
      </DataTable>
    </section>


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
    <div v-if="showRestoreModal" class="modal-overlay" @click="closeRestoreModal">
      <div class="modal-content restore-modal" @click.stop>
        <div class="modal-header">
          <h3>Restore from Backup</h3>
          <button class="close-button" @click="closeRestoreModal">✖</button>
        </div>
        
        <div class="restore-content">
          <div class="backup-info">
            <h4>{{ restoreBackup?.name }}</h4>
            <p>{{ restoreBackup?.description }}</p>
            <div class="backup-details">
              <span>Format: {{ restoreBackup?.format.toUpperCase() }}</span> • 
              <span>Devices: {{ restoreBackup?.device_count }}</span> • 
              <span>Size: {{ formatFileSize(restoreBackup?.file_size || 0) }}</span>
            </div>
          </div>

          <form @submit.prevent="executeRestore" class="restore-form">
            <div class="form-section">
              <h4>Restore Options</h4>
              
              <label class="checkbox-label">
                <input v-model="restoreOptions.include_settings" type="checkbox" />
                <span>Restore Device Settings</span>
              </label>
              
              <label class="checkbox-label">
                <input v-model="restoreOptions.include_schedules" type="checkbox" />
                <span>Restore Schedules</span>
              </label>
              
              <label class="checkbox-label">
                <input v-model="restoreOptions.include_metrics" type="checkbox" />
                <span>Restore Historical Metrics</span>
              </label>
              
              <label class="checkbox-label">
                <input v-model="restoreOptions.dry_run" type="checkbox" />
                <span>Dry Run (Preview only)</span>
              </label>
            </div>

            <div v-if="restorePreview" class="restore-preview">
              <h4>Restore Preview</h4>
              <div class="preview-stats">
                <div>Devices: {{ restorePreview.device_count }}</div>
                <div>Settings: {{ restorePreview.settings_count }}</div>
                <div>Schedules: {{ restorePreview.schedules_count }}</div>
                <div>Metrics: {{ restorePreview.metrics_count }}</div>
              </div>
              
              <div v-if="restorePreview.warnings.length" class="warnings">
                <h5>⚠️ Warnings</h5>
                <ul>
                  <li v-for="warning in restorePreview.warnings" :key="warning">
                    {{ warning }}
                  </li>
                </ul>
              </div>
              
              <div v-if="restorePreview.conflicts.length" class="conflicts">
                <h5>❌ Conflicts</h5>
                <ul>
                  <li v-for="conflict in restorePreview.conflicts" :key="conflict">
                    {{ conflict }}
                  </li>
                </ul>
              </div>
            </div>

            <div v-if="restoreError" class="form-error">
              <strong>Error:</strong> {{ restoreError }}
            </div>

            <div class="modal-actions">
              <button type="button" @click="previewRestore" class="secondary-button" :disabled="restoreLoading">
                Preview Changes
              </button>
              <button type="button" @click="closeRestoreModal" class="secondary-button">
                Cancel
              </button>
              <button 
                type="submit" 
                class="primary-button" 
                :disabled="restoreLoading || (restorePreview?.conflicts.length > 0)"
              >
                {{ restoreLoading ? 'Restoring...' : (restoreOptions.dry_run ? 'Run Preview' : 'Execute Restore') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <div v-if="deleteConfirm" class="modal-overlay" @click="deleteConfirm = null">
      <div class="modal-content confirm-modal" @click.stop>
        <h3>Confirm Delete</h3>
        <p>Are you sure you want to delete backup <strong>{{ deleteConfirm.name }}</strong>?</p>
        <p class="warning">This action cannot be undone.</p>
        <div class="modal-actions">
          <button class="secondary-button" @click="deleteConfirm = null">Cancel</button>
          <button class="danger-button" @click="performDelete">Delete Backup</button>
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
const error = ref('')

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
  error.value = ''
  
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
    error.value = err.message || 'Failed to load backups'
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
