import { defineStore } from 'pinia'
import { listDevices } from '@/api/devices'
import type { Device } from '@/api/types'

type SortField = keyof Device | 'last_seen'
type SortDir = 'asc' | 'desc'

// What the bulk dialog shows about a picked device. It is taken from the row at the moment of
// the tick, so a device on another page or filtered out of the list is still named.
export interface SelectedDevice {
  id: number
  name: string
  ip: string
}

interface DevicesState {
  items: Device[]
  page: number
  pageSize: number
  totalPages: number | null
  hasNext: boolean
  loading: boolean
  error: string | null
  search: string
  sort: { field: SortField; dir: SortDir } | null
  // Devices picked for a bulk operation, by id, in the order they were ticked. The paging is
  // server-side, so the selection outlives a page change.
  selected: Map<number, { name: string; ip: string }>
  // Column visibility preferences
  visibleColumns: {
    select: boolean
    name: boolean
    ip: boolean
    mac: boolean
    type: boolean
    status: boolean
    last_seen: boolean
    firmware: boolean
    actions: boolean
  }
}

// The load whose answer may still be applied (see `fetchDevices`). Not state: nothing renders it.
let latestLoad = 0

export const useDevicesStore = defineStore('devices', {
  state: (): DevicesState => ({
    items: [],
    page: 1,
    pageSize: 25,
    totalPages: null,
    hasNext: false,
    loading: false,
    error: null,
    search: '',
    sort: null,
    selected: new Map(),
    // All columns visible by default
    visibleColumns: {
      select: true,
      name: true,
      ip: true,
      mac: true,
      type: true,
      status: true,
      last_seen: true,
      firmware: true,
      actions: true
    }
  }),

  getters: {
    selectedIds(state): number[] {
      return [...state.selected.keys()]
    },

    selectedCount(state): number {
      return state.selected.size
    },

    selectedDevices(state): SelectedDevice[] {
      return [...state.selected].map(([id, d]) => ({ id, name: d.name, ip: d.ip }))
    },

    filteredItems(state): Device[] {
      if (!state.search) return state.items
      const s = state.search.toLowerCase()
      return state.items.filter(d => {
        return (
          d.name?.toLowerCase().includes(s) ||
          d.ip?.toLowerCase().includes(s) ||
          d.mac?.toLowerCase().includes(s) ||
          d.type?.toLowerCase().includes(s)
        )
      })
    },

    sortedItems(): Device[] {
      const items = [...this.filteredItems]
      if (!this.sort) return items

      const { field, dir } = this.sort
      return items.sort((a, b) => {
        let aVal: any = a[field as keyof Device]
        let bVal: any = b[field as keyof Device]

        // Handle last_seen special case
        if (field === 'last_seen') {
          aVal = a.last_seen ? new Date(a.last_seen).getTime() : 0
          bVal = b.last_seen ? new Date(b.last_seen).getTime() : 0
        }

        if (aVal == null) return dir === 'asc' ? 1 : -1
        if (bVal == null) return dir === 'asc' ? -1 : 1

        if (typeof aVal === 'string') {
          const cmp = aVal.localeCompare(bVal)
          return dir === 'asc' ? cmp : -cmp
        }

        return dir === 'asc' ? (aVal > bVal ? 1 : -1) : (aVal < bVal ? 1 : -1)
      })
    }
  },

  actions: {
    async fetchDevices() {
      // Only the newest load may land: a page change or a refresh while an older load is still
      // running must not be overwritten by the older answer.
      const mine = ++latestLoad
      this.loading = true
      this.error = null
      try {
        const { items, meta } = await listDevices({
          page: this.page,
          pageSize: this.pageSize
        })
        if (mine !== latestLoad) return
        this.items = items
        const p = meta?.pagination
        this.totalPages = p?.total_pages ?? null
        this.hasNext = !!p?.has_next
      } catch (e: any) {
        if (mine !== latestLoad) return
        this.error = e?.message || 'Failed to load devices'
      } finally {
        if (mine === latestLoad) this.loading = false
      }
    },

    setPage(page: number) {
      if (page > 0) this.page = page
    },

    setPageFromQuery(pageStr: string | null | undefined) {
      if (!pageStr) {
        this.page = 1
        return
      }
      const parsed = parseInt(pageStr, 10)
      if (isNaN(parsed) || parsed < 1) {
        this.page = 1
      } else {
        this.page = parsed
      }
    },

    setPageSize(size: number) {
      if ([10, 25, 50, 100].includes(size)) {
        this.pageSize = size
        this.page = 1 // Reset to first page when changing page size
        // Persist to localStorage
        localStorage.setItem('devices.pageSize', String(size))
      }
    },

    setSearch(search: string) {
      this.search = search
    },

    toggleSort(field: SortField) {
      if (!this.sort || this.sort.field !== field) {
        this.sort = { field, dir: 'asc' }
      } else if (this.sort.dir === 'asc') {
        this.sort = { field, dir: 'desc' }
      } else {
        this.sort = null
      }
    },

    isSelected(id: number): boolean {
      return this.selected.has(id)
    },

    toggleSelect(device: Device) {
      if (this.selected.has(device.id)) {
        this.selected.delete(device.id)
      } else {
        this.selected.set(device.id, { name: device.name || '', ip: device.ip || '' })
      }
    },

    // Adds the given devices to the selection; the others stay as they were.
    selectAll(devices: Device[]) {
      for (const d of devices) {
        if (!this.selected.has(d.id)) {
          this.selected.set(d.id, { name: d.name || '', ip: d.ip || '' })
        }
      }
    },

    deselect(ids: number[]) {
      for (const id of ids) this.selected.delete(id)
    },

    clearSelection() {
      this.selected.clear()
    },

    toggleColumn(column: keyof DevicesState['visibleColumns']) {
      this.visibleColumns[column] = !this.visibleColumns[column]
      // A selection nobody can see must not stay armed for a bulk operation.
      if (column === 'select' && !this.visibleColumns.select) this.clearSelection()
      // Persist to localStorage
      localStorage.setItem('devices.visibleColumns', JSON.stringify(this.visibleColumns))
    },

    loadPreferences() {
      // Load page size preference
      const savedPageSize = localStorage.getItem('devices.pageSize')
      if (savedPageSize) {
        const size = parseInt(savedPageSize, 10)
        if ([10, 25, 50, 100].includes(size)) {
          this.pageSize = size
        }
      }

      // Load column visibility preferences
      const savedColumns = localStorage.getItem('devices.visibleColumns')
      if (savedColumns) {
        try {
          const cols = JSON.parse(savedColumns)
          this.visibleColumns = { ...this.visibleColumns, ...cols }
        } catch (e) {
          // Ignore parse errors
        }
      }
    },

    nextPage() {
      if (this.hasNext) this.page += 1
    },

    prevPage() {
      if (this.page > 1) this.page -= 1
    },

    clearError() {
      this.error = null
    }
  }
})
