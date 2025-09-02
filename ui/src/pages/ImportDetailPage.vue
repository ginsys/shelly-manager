<template>
  <main style="padding:16px">
    <h1>Import Result</h1>
    <div v-if="error" class="err">{{ error }}</div>
    <div v-else-if="!data">Loading…</div>
    <div v-else class="grid">
      <div class="card"><strong>ID:</strong> {{ data.import_id }}</div>
      <div class="card"><strong>Plugin:</strong> {{ data.plugin_name }}</div>
      <div class="card"><strong>Format:</strong> {{ data.format }}</div>
      <div class="card"><strong>Imported:</strong> {{ data.records_imported ?? '—' }}</div>
      <div class="card"><strong>Skipped:</strong> {{ data.records_skipped ?? '—' }}</div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { getImportResult } from '@/api/import'
const route = useRoute()
const id = String(route.params.id)
const data = ref<any>(null)
const error = ref('')
onMounted(async () => {
  try { data.value = await getImportResult(id) } catch (e:any) { error.value = e?.message || 'Failed to load result' }
})
</script>

<style scoped>
.grid { display:grid; grid-template-columns: repeat(auto-fit, minmax(220px,1fr)); gap:12px; }
.card { border:1px solid #e5e7eb; padding:12px; border-radius:6px }
.err { color:#b91c1c }
</style>

