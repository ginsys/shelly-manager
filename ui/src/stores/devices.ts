import { defineStore } from 'pinia'
import { listDevices } from '@/api/devices'

export const useDevicesStore = defineStore('devices', {
  state: () => ({
    items: [] as any[],
    page: 1,
    pageSize: 25,
    meta: undefined as any,
    search: '' as string,
    columns: {
      name: true,
      ip: true,
      mac: true,
      type: true,
      status: true,
      last_seen: true,
      firmware: true,
    } as Record<string, boolean>,
  }),
  actions: {
    async fetch() {
      const r = await listDevices({ page: this.page, pageSize: this.pageSize })
      this.items = r.items
      this.meta = r.meta
    },
    setPageFromQuery(v: string) {
      const n = parseInt(v as any, 10)
      this.page = Number.isFinite(n) && n > 0 ? n : 1
    },
    setPageSizeFromQuery(v: string) {
      const n = parseInt(v as any, 10)
      this.pageSize = Number.isFinite(n) && n > 0 ? n : this.pageSize
    },
    setPageSize(n: number) {
      if (!Number.isFinite(n) || n <= 0) return
      this.pageSize = n
      try { localStorage.setItem('devices.pageSize', String(n)) } catch {}
    },
    setColumns(cols: Record<string, boolean>) {
      this.columns = { ...this.columns, ...cols }
      try { localStorage.setItem('devices.columns', JSON.stringify(this.columns)) } catch {}
    },
    initializeFromStorage() {
      try {
        const p = localStorage.getItem('devices.pageSize')
        if (p) this.pageSize = parseInt(p, 10) || this.pageSize
      } catch {}
      try {
        const c = localStorage.getItem('devices.columns')
        if (c) this.columns = { ...this.columns, ...(JSON.parse(c) || {}) }
      } catch {}
    },
  },
})
