<template>
  <div v-if="show" class="modal-overlay" @click="emit('close')">
    <div class="modal-content" @click.stop>
      <div class="modal-header">
        <h3>GitOps Export: {{ exportItem?.name || exportItem?.export_id }}</h3>
        <button class="close-button" @click="emit('close')">✖</button>
      </div>

      <div class="details" v-if="result">
        <div><strong>Status:</strong> {{ result.success ? 'Success' : 'Failed' }}</div>
        <div v-if="result.output_path"><strong>Written to:</strong> <code>{{ result.output_path }}</code></div>
        <div><strong>Files:</strong> {{ result.record_count ?? 0 }}</div>
        <div v-if="result.file_size"><strong>Size:</strong> {{ result.file_size }} bytes</div>
        <div v-if="result.created_at"><strong>Created:</strong> {{ new Date(result.created_at).toLocaleString() }}</div>
        <p class="hint">
          The export wrote these files on the server. Deleting the record does not remove them.
        </p>
        <ul v-if="result.errors?.length" class="problems">
          <li v-for="e in result.errors" :key="e">{{ e }}</li>
        </ul>
        <ul v-if="result.warnings?.length" class="warnings">
          <li v-for="w in result.warnings" :key="w">{{ w }}</li>
        </ul>
      </div>

      <div class="modal-actions">
        <button class="secondary-button" @click="emit('close')">Close</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ExportHistoryItem, ExportResult } from '@/api/export'

defineProps<{
  show: boolean
  exportItem: ExportHistoryItem | null
  result: ExportResult | null
}>()

const emit = defineEmits<{ close: [] }>()
</script>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; padding: 16px; }
.modal-content { background: white; border-radius: 8px; max-width: 640px; width: 100%; max-height: 90vh; overflow: auto; }
.modal-header { display: flex; justify-content: space-between; align-items: center; padding: 24px; border-bottom: 1px solid #e5e7eb; }
.modal-header h3 { margin: 0; color: #1f2937; }
.close-button { background: none; border: none; cursor: pointer; font-size: 1.2rem; color: #6b7280; }
.details { padding: 24px; display: flex; flex-direction: column; gap: 8px; font-size: 0.875rem; }
.hint { margin: 8px 0 0; color: #6b7280; }
.problems { color: #dc2626; }
.warnings { color: #d97706; }
.modal-actions { display: flex; justify-content: flex-end; padding: 0 24px 24px; }
.secondary-button { padding: 8px 16px; border: 1px solid #d1d5db; border-radius: 6px; background: white; cursor: pointer; }
</style>
