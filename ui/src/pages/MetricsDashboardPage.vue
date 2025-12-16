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

    <!-- Collection Controls -->
    <section class="controls-section">
      <h2>Collection Controls</h2>
      <div class="controls">
        <button @click="handleEnableMetrics" class="btn btn-success">Enable Collection</button>
        <button @click="handleDisableMetrics" class="btn btn-warning">Disable Collection</button>
        <button @click="handleCollectMetrics" class="btn btn-primary">Trigger Collection</button>
        <button @click="handleExportPrometheus" class="btn btn-secondary">Export Prometheus</button>
        <button @click="handleTestAlert" class="btn btn-info">Send Test Alert</button>
      </div>
    </section>

    <!-- Dashboard Summary -->
    <section v-if="dashboardSummary" class="summary-section">
      <h2>Dashboard Summary</h2>
      <div class="summary-grid">
        <div class="summary-card">
          <h4>Devices</h4>
          <p>Total: {{ dashboardSummary.devices.total }}</p>
          <p>Online: {{ dashboardSummary.devices.online }}</p>
          <p>Offline: {{ dashboardSummary.devices.offline }}</p>
        </div>
        <div class="summary-card">
          <h4>Exports</h4>
          <p>Total: {{ dashboardSummary.exports.total }}</p>
          <p>Recent: {{ dashboardSummary.exports.recent }}</p>
        </div>
        <div class="summary-card">
          <h4>Imports</h4>
          <p>Total: {{ dashboardSummary.imports.total }}</p>
          <p>Recent: {{ dashboardSummary.imports.recent }}</p>
        </div>
        <div class="summary-card">
          <h4>Drifts</h4>
          <p>Total: {{ dashboardSummary.drifts.total }}</p>
          <p>Unresolved: {{ dashboardSummary.drifts.unresolved }}</p>
        </div>
        <div class="summary-card">
          <h4>Notifications</h4>
          <p>Sent: {{ dashboardSummary.notifications.sent }}</p>
          <p>Failed: {{ dashboardSummary.notifications.failed }}</p>
        </div>
      </div>
    </section>

    <!-- Security Metrics -->
    <section v-if="securityMetrics" class="metrics-section">
      <h2>Security Metrics</h2>
      <div class="metrics-grid">
        <div class="metric-card">
          <h4>Auth Attempts</h4>
          <p>Successful: {{ securityMetrics.authAttempts.successful }}</p>
          <p>Failed: {{ securityMetrics.authAttempts.failed }}</p>
        </div>
        <div class="metric-card">
          <h4>API Calls</h4>
          <p>Total: {{ securityMetrics.apiCalls.total }}</p>
          <p>Errors: {{ securityMetrics.apiCalls.errors }}</p>
        </div>
        <div class="metric-card">
          <h4>Rate Limiting</h4>
          <p>Triggered: {{ securityMetrics.rateLimit.triggered }}</p>
          <p>Blocked: {{ securityMetrics.rateLimit.blocked }}</p>
        </div>
      </div>
    </section>

    <!-- Resolution Metrics -->
    <section v-if="resolutionMetrics" class="metrics-section">
      <h2>Resolution Metrics</h2>
      <div class="metrics-grid">
        <div class="metric-card">
          <h4>Overview</h4>
          <p>Total Resolved: {{ resolutionMetrics.totalResolved }}</p>
          <p>Avg Resolution Time: {{ formatSeconds(resolutionMetrics.averageResolutionTime) }}</p>
        </div>
        <div class="metric-card">
          <h4>By Type</h4>
          <div v-for="(count, type) in resolutionMetrics.byType" :key="type">
            <p>{{ type }}: {{ count }}</p>
          </div>
        </div>
        <div class="metric-card">
          <h4>By User</h4>
          <div v-for="(count, user) in resolutionMetrics.byUser" :key="user">
            <p>{{ user }}: {{ count }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- Notification Metrics -->
    <section v-if="notificationMetrics" class="metrics-section">
      <h2>Notification Metrics</h2>
      <div class="metrics-grid">
        <div class="metric-card">
          <h4>Overview</h4>
          <p>Total Sent: {{ notificationMetrics.totalSent }}</p>
          <p>Total Failed: {{ notificationMetrics.totalFailed }}</p>
        </div>
        <div class="metric-card">
          <h4>By Channel</h4>
          <div v-for="(stats, channel) in notificationMetrics.byChannel" :key="channel">
            <p><strong>{{ channel }}:</strong> {{ stats.sent }} sent, {{ stats.failed }} failed</p>
          </div>
        </div>
        <div class="metric-card">
          <h4>Recent Notifications</h4>
          <div v-for="(notif, idx) in notificationMetrics.recentNotifications.slice(0, 5)" :key="idx">
            <p>{{ notif.channel }} - {{ notif.status }} ({{ new Date(notif.timestamp).toLocaleString() }})</p>
          </div>
        </div>
      </div>
    </section>

    <div v-if="loading" class="loading">Loading advanced metrics...</div>
    <div v-if="error" class="error">{{ error }}</div>

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
import { useError } from '@/composables/useError'
import {
  enableMetrics,
  disableMetrics,
  collectMetrics,
  getPrometheusMetrics,
  getDashboardSummary,
  sendTestAlert,
  getNotificationMetrics,
  getResolutionMetrics,
  getSecurityMetrics,
  type DashboardSummary,
  type NotificationMetrics,
  type ResolutionMetrics,
  type SecurityMetrics
} from '@/api/metrics'

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

// Error handling with structured context
const { error: errorObj, hasError, setError, clearError } = useError()
const error = computed(() => errorObj.value?.message || null)

// Advanced metrics state
const dashboardSummary = ref<DashboardSummary | null>(null)
const notificationMetrics = ref<NotificationMetrics | null>(null)
const resolutionMetrics = ref<ResolutionMetrics | null>(null)
const securityMetrics = ref<SecurityMetrics | null>(null)
const loading = ref(false)

// Update current time every second for relative timestamps
const timeInterval = setInterval(() => {
  currentTime.value = Date.now()
}, 1000)

onMounted(async () => {
  store.fetchStatus()
  store.fetchHealth()
  store.startPolling()
  store.connectWS()
  await fetchAdvancedMetrics()
})

onUnmounted(() => {
  clearInterval(timeInterval)
  store.cleanup()
})

// Fetch all advanced metrics
async function fetchAdvancedMetrics() {
  loading.value = true
  clearError()
  try {
    const [summary, notifications, resolution, security] = await Promise.all([
      getDashboardSummary(),
      getNotificationMetrics(),
      getResolutionMetrics(),
      getSecurityMetrics()
    ])
    dashboardSummary.value = summary
    notificationMetrics.value = notifications
    resolutionMetrics.value = resolution
    securityMetrics.value = security
  } catch (err) {
    setError(err, { action: 'Loading advanced metrics', resource: 'Dashboard metrics' })
  } finally {
    loading.value = false
  }
}

// Collection controls
async function handleEnableMetrics() {
  try {
    await enableMetrics()
    await store.fetchStatus()
    alert('Metrics collection enabled')
  } catch (e: any) {
    alert(e?.message || 'Failed to enable metrics')
  }
}

async function handleDisableMetrics() {
  try {
    await disableMetrics()
    await store.fetchStatus()
    alert('Metrics collection disabled')
  } catch (e: any) {
    alert(e?.message || 'Failed to disable metrics')
  }
}

async function handleCollectMetrics() {
  try {
    await collectMetrics()
    await fetchAdvancedMetrics()
    alert('Metrics collection triggered')
  } catch (e: any) {
    alert(e?.message || 'Failed to trigger collection')
  }
}

async function handleExportPrometheus() {
  try {
    const data = await getPrometheusMetrics()
    const blob = new Blob([data], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `prometheus-metrics-${Date.now()}.txt`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e: any) {
    alert(e?.message || 'Failed to export Prometheus metrics')
  }
}

async function handleTestAlert() {
  try {
    const result = await sendTestAlert()
    if (result.success) {
      alert(`Success: ${result.message}`)
    } else {
      alert(`Failed: ${result.message}`)
    }
  } catch (e: any) {
    alert(e?.message || 'Failed to send test alert')
  }
}

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

function formatSeconds(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
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

/* Controls section */
.controls-section {
  margin-top: 24px;
}

.controls {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 12px;
}

.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  transition: opacity 0.2s;
}

.btn:hover {
  opacity: 0.8;
}

.btn-success {
  background-color: #10b981;
  color: white;
}

.btn-warning {
  background-color: #f59e0b;
  color: white;
}

.btn-primary {
  background-color: #3b82f6;
  color: white;
}

.btn-secondary {
  background-color: #6b7280;
  color: white;
}

.btn-info {
  background-color: #06b6d4;
  color: white;
}

/* Summary and metrics sections */
.summary-section,
.metrics-section {
  margin-top: 24px;
}

.summary-grid,
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
  margin-top: 12px;
}

.summary-card,
.metric-card {
  border: 1px solid #e5e7eb;
  padding: 16px;
  border-radius: 6px;
  background-color: #f9fafb;
}

.summary-card h4,
.metric-card h4 {
  margin-top: 0;
  margin-bottom: 12px;
  color: #374151;
  font-size: 16px;
}

.summary-card p,
.metric-card p {
  margin: 4px 0;
  color: #6b7280;
}

/* Loading and error states */
.loading {
  text-align: center;
  padding: 24px;
  color: #6b7280;
  font-style: italic;
}

.error {
  text-align: center;
  padding: 24px;
  color: #ef4444;
  background-color: #fef2f2;
  border: 1px solid #fca5a5;
  border-radius: 6px;
  margin-top: 16px;
}

/* Responsive design */
@media (max-width: 768px) {
  .cards {
    flex-direction: column;
  }

  .charts {
    grid-template-columns: 1fr;
  }

  .controls {
    flex-direction: column;
  }

  .summary-grid,
  .metrics-grid {
    grid-template-columns: 1fr;
  }
}
</style>
