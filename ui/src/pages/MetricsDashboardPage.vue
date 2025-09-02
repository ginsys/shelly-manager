<template>
  <main style="padding:16px">
    <h1>Metrics Dashboard</h1>
    <div class="cards">
      <div class="card">Enabled: {{ store.status?.enabled ?? '—' }}</div>
      <div class="card">Uptime: {{ store.status?.uptime_seconds ?? '—' }}s</div>
      <div class="card">WS: {{ store.wsConnected ? 'Connected' : 'Disconnected' }}</div>
    </div>
    <section class="charts">
      <div class="panel">
        <h3>System</h3>
        <LineChart :options="systemOptions" />
      </div>
      <div class="panel">
        <h3>Devices</h3>
        <BarChart :options="devicesOptions" />
      </div>
      <div class="panel">
        <h3>Drift</h3>
        <BarChart :options="driftOptions" />
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useMetricsStore } from '@/stores/metrics'
import LineChart from '@/components/charts/LineChart.vue'
import BarChart from '@/components/charts/BarChart.vue'
const store = useMetricsStore()
onMounted(() => { store.fetchStatus(); store.fetchHealth(); store.startPolling(); store.connectWS() })

const systemOptions = computed(() => ({
  xAxis: { type: 'category', data: (store.system?.timestamps ?? []) },
  yAxis: { type: 'value' },
  series: [{ type: 'line', name:'CPU', data: (store.system?.cpu ?? []) }]
}))
const devicesOptions = computed(() => ({
  xAxis: { type: 'category', data: Object.keys(store.devices ?? {}) },
  yAxis: { type: 'value' },
  series: [{ type: 'bar', data: Object.values(store.devices ?? {}) }]
}))
const driftOptions = computed(() => ({
  xAxis: { type: 'category', data: Object.keys(store.drift ?? {}) },
  yAxis: { type: 'value' },
  series: [{ type: 'bar', data: Object.values(store.drift ?? {}) }]
}))
</script>

<style scoped>
.cards { display:flex; gap:12px; }
.card { border:1px solid #e5e7eb; padding:12px; border-radius:6px }
.chart { margin-top:16px; border:1px dashed #cbd5e1; padding:24px; text-align:center; color:#64748b }
.charts { display:grid; grid-template-columns: repeat(auto-fit, minmax(300px,1fr)); gap:12px; margin-top: 16px }
.panel { border:1px solid #e5e7eb; padding:8px; border-radius:6px }
</style>
