<template>
  <main style="padding:16px">
    <h1>Metrics Dashboard</h1>
    <div class="cards">
      <div class="card">Enabled: {{ store.status?.enabled ?? '—' }}</div>
      <div class="card">Uptime: {{ formatUptime(store.status?.uptime_seconds) }}</div>
      <div class="card" :class="connectionClass">
        {{ connectionText }}
        <span v-if="store.wsReconnectAttempts > 0" class="reconnect-info">
          (attempt {{ store.wsReconnectAttempts }})
        </span>
      </div>
      <div class="card" v-if="store.lastMessageAt">
        Last update: {{ formatLastMessage(store.lastMessageAt) }}
      </div>
    </div>
    <section class="charts">
      <div class="panel">
        <h3>System Metrics 
          <span v-if="store.isRealtimeActive" class="realtime-badge">LIVE</span>
        </h3>
        <Suspense>
          <template #default>
            <LineChart v-if="chartsLoaded" :options="systemOptions" />
          </template>
          <template #fallback>
            <div class="chart-loading">Loading chart...</div>
          </template>
        </Suspense>
      </div>
      <div class="panel">
        <h3>Devices</h3>
        <Suspense>
          <template #default>
            <BarChart v-if="chartsLoaded" :options="devicesOptions" />
          </template>
          <template #fallback>
            <div class="chart-loading">Loading chart...</div>
          </template>
        </Suspense>
      </div>
      <div class="panel">
        <h3>Drift</h3>
        <Suspense>
          <template #default>
            <BarChart v-if="chartsLoaded" :options="driftOptions" />
          </template>
          <template #fallback>
            <div class="chart-loading">Loading chart...</div>
          </template>
        </Suspense>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, computed, ref, defineAsyncComponent } from 'vue'
import { useMetricsStore } from '@/stores/metrics'

// Lazy load chart components only when needed with loading states
const LineChart = defineAsyncComponent({
  loader: () => import('@/components/charts/LineChart.vue'),
  loadingComponent: { template: '<div class="chart-loading">Loading chart...</div>' },
  delay: 200,              // 200ms delay before showing spinner
  timeout: 5000,           // 5s timeout for loading
})

const BarChart = defineAsyncComponent({
  loader: () => import('@/components/charts/BarChart.vue'),
  loadingComponent: { template: '<div class="chart-loading">Loading chart...</div>' },
  delay: 200,
  timeout: 5000,
})

const chartsLoaded = ref(true) // Allow charts to render when loaded

const store = useMetricsStore()
const currentTime = ref(Date.now())

// Update current time every second for relative timestamps
const timeInterval = setInterval(() => {
  currentTime.value = Date.now()
}, 1000)

onMounted(() => { 
  store.fetchStatus()
  store.fetchHealth() 
  store.startPolling()
  store.connectWS()
})

onUnmounted(() => {
  clearInterval(timeInterval)
  store.cleanup()
})

// Connection status computed properties
const connectionClass = computed(() => ({
  'connection-connected': store.wsConnected && store.isRealtimeActive,
  'connection-connecting': !store.wsConnected && store.wsReconnectAttempts > 0,
  'connection-disconnected': !store.wsConnected && store.wsReconnectAttempts === 0
}))

const connectionText = computed(() => {
  if (store.wsConnected && store.isRealtimeActive) return 'WebSocket: Connected'
  if (store.wsConnected && !store.isRealtimeActive) return 'WebSocket: Timeout'
  if (!store.wsConnected && store.wsReconnectAttempts > 0) return 'WebSocket: Reconnecting'
  return 'WebSocket: Disconnected'
})

// Enhanced chart options with multiple series
const systemOptions = computed(() => {
  if (!store.system) {
    return {
      xAxis: { type: 'category', data: [] },
      yAxis: { type: 'value', max: 100 },
      series: []
    }
  }

  const series = [
    { 
      type: 'line', 
      name: 'CPU %', 
      data: store.system.cpu,
      smooth: true,
      lineStyle: { color: '#3b82f6' }
    },
    { 
      type: 'line', 
      name: 'Memory %', 
      data: store.system.memory,
      smooth: true,
      lineStyle: { color: '#ef4444' }
    }
  ]
  
  if (store.system.disk) {
    series.push({
      type: 'line',
      name: 'Disk %',
      data: store.system.disk,
      smooth: true,
      lineStyle: { color: '#10b981' }
    })
  }

  return {
    xAxis: { 
      type: 'category', 
      data: store.system.timestamps.map(ts => {
        const date = new Date(ts)
        return date.toLocaleTimeString()
      })
    },
    yAxis: { type: 'value', max: 100, min: 0 },
    legend: { show: true },
    tooltip: { trigger: 'axis' },
    animation: !store.isRealtimeActive, // Disable animation for real-time updates
    series
  }
})

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

// Utility functions
function formatUptime(seconds?: number): string {
  if (!seconds) return '—'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  return `${hours}h ${minutes}m ${secs}s`
}

function formatLastMessage(timestamp: number): string {
  const diff = currentTime.value - timestamp
  if (diff < 1000) return 'just now'
  if (diff < 60000) return `${Math.floor(diff / 1000)}s ago`
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  return `${Math.floor(diff / 3600000)}h ago`
}
</script>

<style scoped>
.cards { display:flex; gap:12px; flex-wrap: wrap; }
.card { border:1px solid #e5e7eb; padding:12px; border-radius:6px; min-width: 120px; }

/* Connection status indicators */
.connection-connected {
  border-color: #10b981;
  background-color: #ecfdf5;
  color: #047857;
}

.connection-connecting {
  border-color: #f59e0b;
  background-color: #fffbeb;
  color: #d97706;
}

.connection-disconnected {
  border-color: #ef4444;
  background-color: #fef2f2;
  color: #dc2626;
}

.reconnect-info {
  font-size: 0.875rem;
  opacity: 0.7;
}

/* Realtime indicator */
.realtime-badge {
  font-size: 0.75rem;
  background-color: #10b981;
  color: white;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: bold;
  margin-left: 8px;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}

/* Chart layout */
.charts { 
  display: grid; 
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); 
  gap: 12px; 
  margin-top: 16px; 
}

.panel { 
  border: 1px solid #e5e7eb; 
  padding: 12px; 
  border-radius: 6px; 
}

.panel h3 {
  margin-top: 0;
  margin-bottom: 12px;
  display: flex;
  align-items: center;
}

/* Chart loading state */
.chart-loading {
  height: 260px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #6b7280;
  font-style: italic;
}

/* Responsive design */
@media (max-width: 768px) {
  .cards {
    flex-direction: column;
  }
  
  .charts {
    grid-template-columns: 1fr;
  }
}
</style>
