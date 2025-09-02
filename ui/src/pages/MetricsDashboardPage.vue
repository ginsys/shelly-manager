<template>
  <main style="padding:16px">
    <h1>Metrics Dashboard</h1>
    <div class="cards">
      <div class="card">Enabled: {{ store.status?.enabled ?? '—' }}</div>
      <div class="card">Uptime: {{ store.status?.uptime_seconds ?? '—' }}s</div>
      <div class="card">WS: {{ store.wsConnected ? 'Connected' : 'Disconnected' }}</div>
    </div>
    <!-- Placeholder for charts integration with ECharts -->
    <div class="chart">(charts to be added)</div>
  </main>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useMetricsStore } from '@/stores/metrics'
const store = useMetricsStore()
onMounted(() => { store.fetchStatus(); store.fetchHealth(); store.connectWS() })
</script>

<style scoped>
.cards { display:flex; gap:12px; }
.card { border:1px solid #e5e7eb; padding:12px; border-radius:6px }
.chart { margin-top:16px; border:1px dashed #cbd5e1; padding:24px; text-align:center; color:#64748b }
</style>

