<template>
  <div class="page">
    <div class="toolbar">
      <router-link to="/export/history" class="btn" data-testid="back-link">← Export history</router-link>
      <h1 class="title">Export Result</h1>
    </div>
    <div v-if="error" class="card err" role="alert">{{ error }}</div>
    <div v-else-if="!data" class="card state">Loading…</div>
    <template v-else>
      <div class="grid">
        <div class="card"><strong>ID:</strong> {{ data.export_id }}</div>
        <div class="card"><strong>Plugin:</strong> {{ data.plugin_name }}</div>
        <div class="card"><strong>Format:</strong> {{ data.format }}</div>
        <div class="card" data-testid="result-status">
          <strong>Status:</strong>
          <span :class="data.success ? 'ok' : 'err'">{{ data.success ? 'Succeeded' : 'Failed' }}</span>
        </div>
        <div class="card" data-testid="result-created"><strong>Created:</strong> {{ createdText }}</div>
        <div class="card" data-testid="result-duration"><strong>Duration:</strong> {{ formatDuration(data.duration) }}</div>
        <div class="card"><strong>Records:</strong> {{ data.record_count ?? '—' }}</div>
        <div class="card"><strong>File size:</strong> {{ data.file_size != null ? `${data.file_size} bytes` : '—' }}</div>
        <div class="card"><strong>Checksum:</strong> {{ data.checksum || '—' }}</div>
      </div>
      <div v-if="data.errors?.length" class="card" data-testid="result-errors">
        <h2 class="section-title err">Errors</h2>
        <ul><li v-for="(e, i) in data.errors" :key="i">{{ e }}</li></ul>
      </div>
      <div v-if="data.warnings?.length" class="card" data-testid="result-warnings">
        <h2 class="section-title">Warnings</h2>
        <ul><li v-for="(w, i) in data.warnings" :key="i">{{ w }}</li></ul>
      </div>
      <div class="card">
        <button class="btn" type="button" data-testid="download-btn" :disabled="downloading" @click="download">
          {{ downloading ? 'Downloading…' : 'Download' }}
        </button>
        <div v-if="downloadError" class="err" role="alert" data-testid="download-error">{{ downloadError }}</div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { getExportResult, downloadExportWithName, type ExportResult } from '@/api/export'
import { formatDuration } from '@/utils/formatDuration'

const route = useRoute()
const id = String(route.params.id)
const data = ref<ExportResult | null>(null)
const error = ref('')
const downloading = ref(false)
const downloadError = ref('')

const createdText = computed(() => {
  const raw = data.value?.created_at
  if (!raw) return '—'
  const d = new Date(raw)
  return Number.isNaN(d.getTime()) ? raw : d.toLocaleString()
})

onMounted(async () => {
  try { data.value = await getExportResult(id) }
  catch (e: any) { error.value = e?.message || 'Failed to load result' }
})

// The download route needs the admin key, which a plain link cannot send: fetch the file through the
// API client and save the blob.
async function download() {
  downloading.value = true
  downloadError.value = ''
  try {
    const { blob, filename } = await downloadExportWithName(id)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename || `export-${id}`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e: any) {
    downloadError.value = e?.message || 'Failed to download export'
  } finally {
    downloading.value = false
  }
}
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 12px; }
.title { font-size: 20px; margin: 0; }
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; padding: 16px; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 12px; }
.section-title { margin: 0 0 8px 0; font-size: 16px; font-weight: 600; }
.state { text-align: center; color: #64748b; }
.ok { color: #065f46; }
.err { color: #b91c1c; }
</style>
