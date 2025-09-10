import { defineStore } from 'pinia'
import { 
  listExportHistory, 
  getExportStatistics, 
  createSMAExport,
  getSMAExportResult,
  downloadSMAExport,
  previewSMAExport,
  type ExportHistoryItem,
  type SMAExportRequest,
  type SMAExportResult
} from '@/api/export'
import type { Metadata } from '@/api/types'

export const useExportStore = defineStore('export', {
  state: () => ({
    items: [] as ExportHistoryItem[],
    loading: false,
    error: '' as string | '',
    page: 1,
    pageSize: 20,
    plugin: '' as string,
    success: undefined as boolean | undefined,
    meta: undefined as Metadata | undefined,
    stats: { total: 0, success: 0, failure: 0, by_plugin: {} as Record<string, number> },
    // SMA-specific state
    smaExportLoading: false,
    smaExportError: '' as string | '',
    smaExportResult: null as SMAExportResult | null,
  }),
  actions: {
    async fetchHistory() {
      this.loading = true
      this.error = ''
      try {
        const { items, meta } = await listExportHistory({
          page: this.page,
          pageSize: this.pageSize,
          plugin: this.plugin || undefined,
          success: this.success,
        })
        this.items = items
        this.meta = meta
      } catch (e: any) {
        this.error = e?.message || 'Failed to load history'
      } finally {
        this.loading = false
      }
    },
    async fetchStats() {
      try {
        this.stats = await getExportStatistics()
      } catch (e) {
        // ignore for now; UI can display N/A
      }
    },
    setPlugin(val: string) {
      this.plugin = val
      this.page = 1
    },
    setSuccess(val: boolean | undefined) {
      this.success = val
      this.page = 1
    },
    setPage(page: number) { this.page = page },
    setPageSize(size: number) { this.pageSize = size },
    
    // SMA-specific actions
    async createSMAExport(request: SMAExportRequest): Promise<string> {
      this.smaExportLoading = true
      this.smaExportError = ''
      this.smaExportResult = null
      
      try {
        const result = await createSMAExport(request)
        return result.export_id
      } catch (e: any) {
        this.smaExportError = e?.message || 'Failed to create SMA export'
        throw e
      } finally {
        this.smaExportLoading = false
      }
    },

    async fetchSMAExportResult(exportId: string): Promise<SMAExportResult> {
      try {
        const result = await getSMAExportResult(exportId)
        this.smaExportResult = result
        return result
      } catch (e: any) {
        this.smaExportError = e?.message || 'Failed to fetch SMA export result'
        throw e
      }
    },

    async downloadSMAExport(exportId: string, filename?: string): Promise<void> {
      try {
        const blob = await downloadSMAExport(exportId)
        
        // Create download link
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = filename || `shelly-manager-backup-${exportId}.sma`
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(url)
      } catch (e: any) {
        this.smaExportError = e?.message || 'Failed to download SMA export'
        throw e
      }
    },

    async previewSMAExport(request: SMAExportRequest) {
      try {
        return await previewSMAExport(request)
      } catch (e: any) {
        this.smaExportError = e?.message || 'Failed to preview SMA export'
        throw e
      }
    },

    clearSMAExportState() {
      this.smaExportLoading = false
      this.smaExportError = ''
      this.smaExportResult = null
    },
  },
})

