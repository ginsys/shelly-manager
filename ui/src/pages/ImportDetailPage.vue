<template>
  <div class="page">
    <div class="toolbar">
      <router-link to="/import/history" class="btn" data-testid="back-link">← Import history</router-link>
      <h1 class="title">Import Result</h1>
    </div>
    <div v-if="error" class="card err" role="alert">{{ error }}</div>
    <div v-else-if="!data" class="card state">Loading…</div>
    <template v-else>
      <div class="grid">
        <div class="card"><strong>ID:</strong> {{ data.import_id }}</div>
        <div class="card"><strong>Plugin:</strong> {{ data.plugin_name }}</div>
        <div class="card"><strong>Format:</strong> {{ data.format }}</div>
        <div class="card" data-testid="result-status">
          <strong>Status:</strong> <span :class="data.success ? 'ok' : 'err'">{{ data.success ? 'Succeeded' : 'Failed' }}</span>
        </div>
        <div class="card" data-testid="result-created"><strong>Created:</strong> {{ createdText }}</div>
        <div class="card" data-testid="result-duration"><strong>Duration:</strong> {{ formatDuration(data.duration) }}</div>
        <div class="card" data-testid="result-imported"><strong>Imported:</strong> {{ data.records_imported ?? '—' }}</div>
        <div class="card" data-testid="result-skipped"><strong>Skipped:</strong> {{ data.records_skipped ?? '—' }}</div>
      </div>
      <div v-if="data.errors?.length" class="card" data-testid="result-errors">
        <h2 class="section-title err">Errors</h2>
        <ul><li v-for="(e, i) in data.errors" :key="i">{{ e }}</li></ul>
      </div>
      <div v-if="data.warnings?.length" class="card" data-testid="result-warnings">
        <h2 class="section-title">Warnings</h2>
        <ul><li v-for="(w, i) in data.warnings" :key="i">{{ w }}</li></ul>
      </div>
      <div v-if="data.changes?.length" class="card" data-testid="result-changes">
        <h2 class="section-title">Changes</h2>
        <p class="counts">
          <span data-testid="count-create">Create: {{ counts.create }}</span>
          <span data-testid="count-update">Update: {{ counts.update }}</span>
          <span data-testid="count-delete">Delete: {{ counts.delete }}</span>
        </p>
        <table>
          <thead><tr><th>Type</th><th>Resource</th><th>ID</th><th>Field</th></tr></thead>
          <tbody>
            <tr v-for="(c, i) in data.changes" :key="i">
              <td>{{ c.type }}</td>
              <td>{{ c.resource }}</td>
              <td>{{ c.resource_id }}</td>
              <td>{{ c.field || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { getImportResult, type ImportResult } from '@/api/import'
import { formatDuration } from '@/utils/formatDuration'

const route = useRoute()
const id = String(route.params.id)
const data = ref<ImportResult | null>(null)
const error = ref('')

const createdText = computed(() => {
  const raw = data.value?.created_at
  if (!raw) return '—'
  const d = new Date(raw)
  return Number.isNaN(d.getTime()) ? raw : d.toLocaleString()
})

// The old and new values of a change can hold secrets and any amount of data, so the page shows the
// counts and which resource and field changed, never the values.
const counts = computed(() => {
  const c = { create: 0, update: 0, delete: 0 }
  for (const change of data.value?.changes ?? []) {
    if (change.type in c) c[change.type] += 1
  }
  return c
})

onMounted(async () => {
  try { data.value = await getImportResult(id) }
  catch (e: any) { error.value = e?.message || 'Failed to load result' }
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 12px; }
.title { font-size: 20px; margin: 0; }
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; padding: 16px; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 12px; }
.section-title { margin: 0 0 8px 0; font-size: 16px; font-weight: 600; }
.counts { display: flex; gap: 16px; margin: 0 0 8px 0; }
table { border-collapse: collapse; width: 100%; }
th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid #f1f5f9; }
.state { text-align: center; color: #64748b; }
.ok { color: #065f46; }
.err { color: #b91c1c; }
</style>
