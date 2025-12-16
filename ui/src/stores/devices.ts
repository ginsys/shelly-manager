import { defineStore } from 'pinia'
import { listDevices } from '@/api/devices'
import type { Device } from '@/api/types'

type SortField = keyof Device | 'last_seen'
type SortDir = 'asc' | 'desc'

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
  // Column visibility preferences
  visibleColumns: {
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
    // All columns visible by default
    visibleColumns: {
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
      this.loading = true
      this.error = null
      try {
        const { items, meta } = await listDevices({
          page: this.page,
          pageSize: this.pageSize
        })
        this.items = items
        const p = meta?.pagination
        this.totalPages = p?.total_pages ?? null
        this.hasNext = !!p?.has_next
      } catch (e: any) {
        this.error = e?.message || 'Failed to load devices'
      } finally {
        this.loading = false
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

    toggleColumn(column: keyof DevicesState['visibleColumns']) {
      this.visibleColumns[column] = !this.visibleColumns[column]
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
