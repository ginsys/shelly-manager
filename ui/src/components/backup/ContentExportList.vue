<template>
  <section class="content-exports">
    <h2>Content Exports (JSON / YAML / SMA)</h2>
    <DataTable :rows="rows" :loading="false" :error="''" :cols="6" :rowKey="(row: any) => row.export_id">
      <template #header>
        <th>Plugin</th>
        <th>Format</th>
        <th>Records</th>
        <th>Size</th>
        <th>Created</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>{{ row.plugin_name }}</td>
        <td><span class="format-badge">{{ row.format?.toUpperCase?.() || row.plugin_name?.toUpperCase?.() }}</span></td>
        <td>{{ row.record_count ?? '—' }}</td>
        <td>
          <span v-if="row.file_size">{{ formatFileSize(row.file_size) }}</span>
          <span v-else class="no-data">—</span>
        </td>
        <td>{{ formatDate(row.created_at) }}</td>
        <td>
          <button class="action-btn download-btn" @click="$emit('download', row.export_id)">⬇ Download</button>
        </td>
      </template>
    </DataTable>
  </section>
</template>

<script setup lang="ts">
import DataTable from '@/components/DataTable.vue'
import { formatFileSize, formatDate } from '@/utils/format'

defineProps<{ rows: any[] }>()
defineEmits<{ download: [string] }>()
</script>

<style scoped>
.content-exports { margin-top: 24px }
.format-badge { background: #eef2ff; color: #3730a3; padding: 2px 6px; border-radius: 4px; font-size: .75rem }
.no-data { color: #9ca3af }
.action-btn { padding: 4px 8px; border: 1px solid #e5e7eb; border-radius: 4px; background: #fff; font-size: .75rem; cursor: pointer }
.download-btn:hover { background: #f0f9ff; border-color: #0ea5e9; color: #0369a1 }
</style>
