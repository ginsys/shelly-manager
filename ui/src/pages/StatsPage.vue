<template>
  <main style="padding:16px">
    <h1>Statistics</h1>
    <section class="grid">
      <div class="card">
        <h2>Export</h2>
        <div>Total: {{ exportStats?.total ?? '—' }}</div>
        <div>Success: {{ exportStats?.success ?? '—' }}</div>
        <div>Failure: {{ exportStats?.failure ?? '—' }}</div>
      </div>
      <div class="card">
        <h2>Import</h2>
        <div>Total: {{ importStats?.total ?? '—' }}</div>
        <div>Success: {{ importStats?.success ?? '—' }}</div>
        <div>Failure: {{ importStats?.failure ?? '—' }}</div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getExportStatistics } from '@/api/export'
import { getImportStatistics } from '@/api/import'

const exportStats = ref<any>(null)
const importStats = ref<any>(null)
onMounted(async () => {
  try { exportStats.value = await getExportStatistics() } catch {}
  try { importStats.value = await getImportStatistics() } catch {}
})
</script>

<style scoped>
.grid { display:grid; grid-template-columns: repeat(auto-fit, minmax(240px,1fr)); gap:12px; margin-top:12px }
.card { border:1px solid #e5e7eb; padding:12px; border-radius:6px }
</style>

