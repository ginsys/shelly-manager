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
        <td>{{ row.import_id }}</td>
        <td>{{ row.plugin_name }}</td>
        <td>{{ row.format }}</td>
        <td>{{ row.success ? 'Yes':'No' }}</td>
        <td>{{ row.records_imported ?? '-' }}</td>
      </template>
    </DataTable>
    <ErrorDisplay
      v-if="store.error && !store.loading"
      :error="{ code: 'IMPORT_HISTORY_FAILED', message: store.error, retryable: true }"
      title="Failed to load import history"
      @retry="store.fetchHistory"
      @dismiss="() => (store.error = '')"
    />
    <PaginationBar
      v-if="store.meta?.pagination"
      :page="store.meta.pagination.page"
      :totalPages="store.meta.pagination.total_pages"
      :hasNext="store.meta.pagination.has_next"
      :hasPrev="store.meta.pagination.has_previous"
      @update:page="(p:number)=>{ store.setPage(p); store.fetchHistory() }"
    />
  </main>
  
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useImportStore } from '@/stores/import'
import DataTable from '@/components/DataTable.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import FilterBar from '@/components/FilterBar.vue'
import ImportPreviewForm from '@/components/ImportPreviewForm.vue'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'

const store = useImportStore()
onMounted(() => { store.fetchHistory(); store.fetchStats() })
</script>
