<template>
  <div class="page">
    <div class="toolbar">
      <h1 class="title">Devices</h1>
      <div class="spacer" />
      <input
        class="search"
        v-model="search"
        type="text"
        placeholder="Search (name, IP, MAC, type)"
      />
      <select v-model.number="pageSize" class="select">
        <option :value="10">10</option>
        <option :value="25">25</option>
        <option :value="50">50</option>
      </select>
    </div>

    <div class="card">
      <div v-if="loading" class="state">Loading...</div>
      <div v-else-if="error" class="state error">{{ error }}</div>

      <table v-else class="table">
        <thead>
          <tr>
            <th @click="toggleSort('name')">Name <SortIcon :field="'name'" :sort="sort" /></th>
            <th @click="toggleSort('ip')">IP <SortIcon :field="'ip'" :sort="sort" /></th>
            <th @click="toggleSort('mac')">MAC <SortIcon :field="'mac'" :sort="sort" /></th>
            <th @click="toggleSort('type')">Type <SortIcon :field="'type'" :sort="sort" /></th>
            <th @click="toggleSort('status')">Status <SortIcon :field="'status'" :sort="sort" /></th>
            <th @click="toggleSort('last_seen')">Last Seen <SortIcon :field="'last_seen'" :sort="sort" /></th>
            <th>Firmware</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in pagedSortedFiltered" :key="d.id">
            <td>
              <router-link :to="`/devices/${d.id}`" class="rowlink">{{ d.name || '-' }}</router-link>
            </td>
            <td>{{ d.ip || '-' }}</td>
            <td class="mono">{{ d.mac }}</td>
            <td>{{ d.type }}</td>
            <td>
              <span :class="['chip', d.status]">{{ d.status || 'unknown' }}</span>
            </td>
            <td>{{ formatDate(d.last_seen) }}</td>
            <td class="mono small">{{ d.firmware || '-' }}</td>
          </tr>
          <tr v-if="pagedSortedFiltered.length === 0">
            <td colspan="7" class="state">No devices found</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="pagination">
      <button class="btn" :disabled="page <= 1" @click="prevPage">Prev</button>
      <span>Page {{ page }} / {{ totalPages || 1 }}</span>
      <button class="btn" :disabled="!hasNext" @click="nextPage">Next</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { listDevices } from '../api/devices'
import type { Device } from '../api/types'

// Local sort descriptor
type Sort = { field: keyof Device | 'last_seen'; dir: 'asc' | 'desc' } | null

const loading = ref(false)
const error = ref<string | null>(null)
const items = ref<Device[]>([])
const page = ref(1)
const pageSize = ref(25)
const totalPages = ref<number | null>(null)
const hasNext = ref(false)
const search = ref('')
const sort = ref<Sort>(null)

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    const { items: list, meta } = await listDevices({ page: page.value, pageSize: pageSize.value })
    items.value = list
    const p = meta?.pagination
    totalPages.value = p?.total_pages ?? null
    hasNext.value = !!p?.has_next
  } catch (e: any) {
    error.value = e?.message || 'Failed to load devices'
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
watch([page, pageSize], fetchData)

function prevPage() { if (page.value > 1) page.value -= 1 }
function nextPage() { if (hasNext.value) page.value += 1 }

function toggleSort(field: Sort['field']) {
  if (!sort.value || sort.value.field !== field) {
    sort.value = { field, dir: 'asc' }
  } else if (sort.value.dir === 'asc') {
    sort.value.dir = 'desc'
  } else {
    sort.value = null
  }
}

function formatDate(iso?: string) {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter(d => {
    return (
      (d.name || '').toLowerCase().includes(q) ||
      (d.ip || '').toLowerCase().includes(q) ||
      (d.mac || '').toLowerCase().includes(q) ||
      (d.type || '').toLowerCase().includes(q)
    )
  })
})

const sorted = computed(() => {
  if (!sort.value) return filtered.value
  const { field, dir } = sort.value
  const copy = filtered.value.slice()
  copy.sort((a: any, b: any) => {
    const av = a?.[field]
    const bv = b?.[field]
    if (av == null && bv == null) return 0
    if (av == null) return 1
    if (bv == null) return -1
    if (av < bv) return dir === 'asc' ? -1 : 1
    if (av > bv) return dir === 'asc' ? 1 : -1
    return 0
  })
  return copy
})

// The backend already returns a page slice; but we keep local guard in case of future changes
const pagedSortedFiltered = computed(() => sorted.value)

</script>

<script lang="ts">
// Local presentational helper (inline component)
export default {
  components: {
    SortIcon: {
      props: { field: { type: String, required: true }, sort: { type: Object, required: false } },
      template: `<span class="sort" v-if="sort && sort.field === field">{{ sort.dir === 'asc' ? '▲' : '▼' }}</span>`
    }
  }
}
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }
.search { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px; min-width: 260px; }
.select { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; }
.state { padding: 16px; text-align: center; color: #64748b; }
.state.error { color: #b91c1c; }
.table { width: 100%; border-collapse: collapse; }
.table th, .table td { text-align: left; padding: 10px 12px; border-bottom: 1px solid #f1f5f9; }
.table th { background: #f8fafc; cursor: pointer; user-select: none; white-space: nowrap; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.small { font-size: 12px; color: #475569; }
.chip { padding: 2px 8px; border-radius: 999px; font-size: 12px; background: #e2e8f0; color: #334155; }
.chip.online { background: #dcfce7; color: #065f46; }
.chip.offline { background: #fee2e2; color: #991b1b; }
.sort { margin-left: 4px; font-size: 12px; color: #64748b; }
.pagination { display: flex; align-items: center; gap: 8px; justify-content: center; padding: 8px; color: #334155; }
.btn { padding: 6px 10px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.rowlink { color: #2563eb; text-decoration: none; }
.rowlink:hover { text-decoration: underline; }
</style>
