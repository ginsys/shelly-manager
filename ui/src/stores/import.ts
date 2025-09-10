import { defineStore } from 'pinia'
import { listImportHistory, getImportStatistics, type ImportHistoryItem } from '@/api/import'
import { 
  importSMAFile, 
  getSMAImportResult, 
  previewSMAFile,
  type SMAImportRequest,
  type SMAImportResult,
  type SMAPreview
} from '@/api/export'
import type { Metadata } from '@/api/types'

export const useImportStore = defineStore('import', {
  state: () => ({
    items: [] as ImportHistoryItem[],
    loading: false,
    error: '' as string | '',
    page: 1,
    pageSize: 20,
    plugin: '' as string,
    success: undefined as boolean | undefined,
    meta: undefined as Metadata | undefined,
    stats: { total: 0, success: 0, failure: 0, by_plugin: {} as Record<string, number> },
    // SMA-specific state
    smaImportLoading: false,
    smaImportError: '' as string | '',
    smaImportResult: null as SMAImportResult | null,
    smaPreviewLoading: false,
    smaPreview: null as SMAPreview | null,
  }),
  actions: {
    async fetchHistory() {
      this.loading = true
      this.error = ''
      try {
        const { items, meta } = await listImportHistory({
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
      try { this.stats = await getImportStatistics() } catch {}
    },
    setPlugin(val: string) { this.plugin = val; this.page = 1 },
    setSuccess(val: boolean | undefined) { this.success = val; this.page = 1 },
    setPage(page: number) { this.page = page },
    setPageSize(size: number) { this.pageSize = size },

    // SMA-specific actions
    async previewSMAFile(file: File, options?: { 
      validate_checksums?: boolean 
      validate_structure?: boolean 
    }): Promise<SMAPreview> {
      this.smaPreviewLoading = true
      this.smaImportError = ''
      this.smaPreview = null

      try {
        const result = await previewSMAFile(file, options)
        this.smaPreview = result
        return result
      } catch (e: any) {
        this.smaImportError = e?.message || 'Failed to preview SMA file'
        throw e
      } finally {
        this.smaPreviewLoading = false
      }
    },

    async importSMAFile(request: SMAImportRequest): Promise<string> {
      this.smaImportLoading = true
      this.smaImportError = ''
      this.smaImportResult = null

      try {
        const result = await importSMAFile(request)
        return result.import_id
      } catch (e: any) {
        this.smaImportError = e?.message || 'Failed to import SMA file'
        throw e
      } finally {
        this.smaImportLoading = false
      }
    },

    async fetchSMAImportResult(importId: string): Promise<SMAImportResult> {
      try {
        const result = await getSMAImportResult(importId)
        this.smaImportResult = result
        return result
      } catch (e: any) {
        this.smaImportError = e?.message || 'Failed to fetch SMA import result'
        throw e
      }
    },

    clearSMAImportState() {
      this.smaImportLoading = false
      this.smaImportError = ''
      this.smaImportResult = null
      this.smaPreviewLoading = false
      this.smaPreview = null
    },
  },
})

