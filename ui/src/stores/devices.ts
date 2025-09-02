import { defineStore } from 'pinia'
import { listDevices } from '@/api/devices'

export const useDevicesStore = defineStore('devices', {
  state: () => ({ items: [] as any[], page: 1, pageSize: 25, meta: undefined as any }),
  actions: {
    async fetch(){ const r = await listDevices({ page: this.page, pageSize: this.pageSize }); this.items = r.items; this.meta = r.meta },
    setPageFromQuery(v: string){ const n = parseInt(v as any, 10); this.page = Number.isFinite(n) && n > 0 ? n : 1 },
    setPageSizeFromQuery(v: string){ const n = parseInt(v as any, 10); this.pageSize = Number.isFinite(n) && n > 0 ? n : this.pageSize },
  }
})
