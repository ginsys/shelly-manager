<template>
  <main style="padding:16px">
    <h1>Import History</h1>
    <FilterBar
      :plugin="store.plugin"
      :success="store.success"
      @update:plugin="(v:string)=>{ store.setPlugin(v); store.fetchHistory() }"
      @update:success="(v:boolean|undefined)=>{ store.setSuccess(v); store.fetchHistory() }"
    />
    <ImportPreviewForm />
    <DataTable
      :rows="store.items"
      :loading="store.loading"
      :error="store.error"
      :cols="6"
      :rowKey="row => row.import_id"
    >
      <template #header>
        <th>Time</th>
        <th>Import ID</th>
        <th>Plugin</th>
        <th>Format</th>
        <th>Success</th>
        <th>Imported</th>
      </template>
      <template #row="{ row }">
        <td>{{ new Date(row.created_at).toLocaleString() }}</td>
        <td><router-link :to="`/import/${row.import_id}`">{{ row.import_id }}</router-link></td>
        <td>{{ row.plugin_name }}</td>
        <td>{{ row.format }}</td>
        <td>{{ row.success ? 'Yes':'No' }}</td>
        <td>{{ row.records_imported ?? '-' }}</td>
      </template>
    </DataTable>
    <PaginationBar
      v-if="store.meta?.pagination"
      :page="store.meta.pagination.page"
      :totalPages="store.meta.pagination.total_pages"
      :hasNext="store.meta.pagination.has_next"
      :hasPrev="store.meta.pagination.has_previous"
      @update:page="(p:number)=>{ store.setPage(p); store.fetchHistory() }"
    />

    <section style="margin-top:16px">
      <h2>Statistics</h2>
      <div class="stats" data-testid="history-stats">
        <div v-if="store.statsError" class="card err" role="alert">Statistics unavailable: {{ store.statsError }}</div>
        <template v-else>
          <div class="card">Total: {{ store.stats.total }}</div>
          <div class="card">Success: {{ store.stats.success }}</div>
          <div class="card">Failure: {{ store.stats.failure }}</div>
        </template>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useImportStore } from '@/stores/import'
import DataTable from '@/components/DataTable.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import FilterBar from '@/components/FilterBar.vue'
import ImportPreviewForm from '@/components/ImportPreviewForm.vue'

const store = useImportStore()
onMounted(() => { store.fetchHistory(); store.fetchStats() })
</script>

<style scoped>
.stats { display:flex; gap:12px; }
.card { border:1px solid #e5e7eb; padding:8px 12px; border-radius:6px; }
.err { color:#b91c1c }
</style>
