import { defineStore } from 'pinia'
import { listExportHistory, getExportStatistics, type ExportHistoryItem } from '@/api/export'
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
  },
})

