<template>
  <main style="padding:16px">
    <h1>Export Result</h1>
    <div v-if="error" class="err">{{ error }}</div>
    <div v-else-if="!data">Loading…</div>
    <div v-else class="grid">
      <div class="card"><strong>ID:</strong> {{ data.export_id }}</div>
      <div class="card"><strong>Plugin:</strong> {{ data.plugin_name }}</div>
      <div class="card"><strong>Format:</strong> {{ data.format }}</div>
      <div class="card"><strong>Records:</strong> {{ data.record_count ?? '—' }}</div>
      <div class="card"><strong>File size:</strong> {{ data.file_size ?? '—' }}</div>
      <div class="card"><strong>Checksum:</strong> {{ data.checksum ?? '—' }}</div>
      <div class="card">
        <a :href="downloadUrl" target="_blank" rel="noreferrer">Download</a>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { getExportResult } from '@/api/export'

const route = useRoute()
const id = String(route.params.id)
const data = ref<any>(null)
const error = ref('')
const base = (window as any).__API_BASE__ || '/api/v1'
const downloadUrl = `${base}/export/${id}/download`

onMounted(async () => {
  try { data.value = await getExportResult(id) }
  catch (e:any) { error.value = e?.message || 'Failed to load result' }
})
</script>

<style scoped>
.grid { display:grid; grid-template-columns: repeat(auto-fit, minmax(220px,1fr)); gap:12px; }
.card { border:1px solid #e5e7eb; padding:12px; border-radius:6px }
.err { color:#b91c1c }
</style>

