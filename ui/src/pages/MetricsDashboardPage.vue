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
    <section v-if="dashboardSummary" class="summary-section" data-testid="panel-summary">
      <h2>Dashboard Summary</h2>
      <div class="summary-grid">
        <div class="summary-card">
          <h4>Devices</h4>
          <p>Total: {{ dashboardSummary.system_status.total_devices }}</p>
          <p>Online: {{ dashboardSummary.system_status.online_devices }}</p>
          <p>Offline: {{ dashboardSummary.system_status.total_devices - dashboardSummary.system_status.online_devices }}</p>
          <p>With drift: {{ dashboardSummary.system_status.devices_with_drift }}</p>
        </div>
        <div class="summary-card">
          <h4>Drift</h4>
          <p>Issues: {{ dashboardSummary.drift_metrics.total_drift_issues }}</p>
        </div>
        <div class="summary-card">
          <h4>Notifications</h4>
          <p>Sent: {{ dashboardSummary.notification_metrics.total_sent }}</p>
          <p>Failed: {{ dashboardSummary.notification_metrics.total_failed }}</p>
        </div>
      </div>
    </section>

    <!-- Security Metrics -->
    <section v-if="securityMetrics" class="metrics-section" data-testid="panel-security">
      <h2>Security Metrics</h2>
      <div class="metrics-grid">
        <div class="metric-card">
          <h4>Requests</h4>
          <p>Total: {{ securityMetrics.total_requests }}</p>
          <p>Blocked: {{ securityMetrics.blocked_requests }}</p>
          <p>Suspicious: {{ securityMetrics.suspicious_requests }}</p>
        </div>
        <div class="metric-card">
          <h4>Violations</h4>
          <p>Rate limit: {{ securityMetrics.rate_limit_violations }}</p>
          <p>Validation failures: {{ securityMetrics.validation_failures }}</p>
        </div>
        <div class="metric-card">
          <h4>Threats</h4>
          <p>Blocked IPs: {{ securityMetrics.blocked_ips }}</p>
          <p>Active threats: {{ securityMetrics.active_threats }}</p>
        </div>
      </div>
    </section>

    <!-- Resolution Metrics -->
    <section v-if="resolutionMetrics" class="metrics-section" data-testid="panel-resolution">
      <h2>Resolution Metrics</h2>
      <div class="metrics-grid">
        <div class="metric-card">
          <h4>Overview</h4>
          <p>Total Resolved: {{ resolutionMetrics.total_resolutions }}</p>
          <p>Avg Review Time: {{ formatSeconds(resolutionMetrics.average_review_time_seconds) }}</p>
        </div>
        <div class="metric-card">
          <h4>By Category</h4>
          <div v-for="(count, category) in resolutionMetrics.resolutions_by_category" :key="category">
            <p>{{ category }}: {{ count }}</p>
          </div>
        </div>
        <div class="metric-card">
          <h4>Auto-fix Success Rate</h4>
          <div v-for="(rate, category) in resolutionMetrics.auto_fix_success_rate" :key="category">
            <p>{{ category }}: {{ rate }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- Notification Metrics -->
    <section v-if="notificationMetrics" class="metrics-section" data-testid="panel-notifications">
      <h2>Notification Metrics</h2>
      <div class="metrics-grid">
        <div class="metric-card">
          <h4>Overview</h4>
          <p>Total Sent: {{ notificationMetrics.total_sent }}</p>
          <p>Total Failed: {{ notificationMetrics.total_failed }}</p>
          <p>Avg Latency: {{ formatSeconds(notificationMetrics.average_latency_seconds) }}</p>
        </div>
        <div class="metric-card">
          <h4>By Channel</h4>
          <div v-for="(count, channel) in notificationMetrics.channel_breakdown" :key="channel">
            <p>{{ channel }}: {{ count }}</p>
          </div>
        </div>
        <div class="metric-card">
          <h4>By Alert Level</h4>
          <div v-for="(count, level) in notificationMetrics.alert_level_breakdown" :key="level">
            <p>{{ level }}: {{ count }}</p>
          </div>
        </div>
      </div>
    </section>

    <div v-if="loading" class="loading">Loading advanced metrics...</div>
    <div v-if="error" class="error">{{ error }}</div>

    <!-- Live Events feed (alert / device status change / drift detected) -->
    <section class="events-section">
      <h2>Live Events
        <span v-if="store.isRealtimeActive" class="realtime-badge">LIVE</span>
      </h2>
      <div v-if="recentEvents.length === 0" class="events-empty">No live events yet.</div>
      <ul v-else class="events-list">
        <li
          v-for="ev in recentEvents"
          :key="ev.id"
          class="event-item"
          :class="`event-${ev.severity}`"
        >
          <span class="event-kind">{{ ev.kind }}</span>
          <span class="event-msg">{{ ev.message }}</span>
          <span class="event-time">{{ new Date(ev.timestamp).toLocaleTimeString() }}</span>
        </li>
      </ul>
    </section>

    <section class="charts">
      <div class="panel">
        <h3>Device Counts
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
  type SecurityMetrics
} from '@/api/metrics'
import type {
  DashboardMetrics,
  NotificationMetrics,
  ResolutionMetrics
} from '@/api/metricsContract'

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
const { error: errorObj, setError, clearError } = useError()
const error = computed(() => errorObj.value?.message || null)

// Advanced metrics state
const dashboardSummary = ref<DashboardMetrics | null>(null)
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

// Fetch all advanced metrics. Each call fills its own panel: a failed call
// empties only that panel and is named in the banner, so one failing endpoint
// (the security one is 404 without a security monitor) never blanks the rest.
async function fetchAdvancedMetrics() {
  loading.value = true
  clearError()
  try {
    const [summary, notifications, resolution, security] = await Promise.allSettled([
      getDashboardSummary(),
      getNotificationMetrics(),
      getResolutionMetrics(),
      getSecurityMetrics()
    ])
    dashboardSummary.value = summary.status === 'fulfilled' ? summary.value : null
    notificationMetrics.value = notifications.status === 'fulfilled' ? notifications.value : null
    resolutionMetrics.value = resolution.status === 'fulfilled' ? resolution.value : null
    securityMetrics.value = security.status === 'fulfilled' ? security.value : null

    const failures = [
      ['Dashboard summary', summary],
      ['Notification metrics', notifications],
      ['Resolution metrics', resolution],
      ['Security metrics', security]
    ].flatMap(([label, result]) =>
      (result as PromiseSettledResult<unknown>).status === 'rejected'
        ? [`${label}: ${reasonMessage((result as PromiseRejectedResult).reason)}`]
        : []
    )
    if (failures.length > 0) {
      setError(new Error(failures.join('; ')), { action: 'Loading advanced metrics', resource: 'Dashboard metrics' })
    }
  } finally {
    loading.value = false
  }
}

function reasonMessage(reason: unknown): string {
  return reason instanceof Error ? reason.message : String(reason)
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
    if (result.status === 'sent') {
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

// Device-count trend. The backend exposes no CPU/memory/disk telemetry (#247),
// so this "system" chart plots the real device counts from system_status,
// accumulated over each 5s snapshot into a bounded ring buffer.
const systemOptions = computed(() => {
  const s = store.series
  return {
    xAxis: {
      type: 'category',
      data: s.timestamps.map(ts => new Date(ts).toLocaleTimeString())
    },
    yAxis: { type: 'value', min: 0 },
    legend: { show: true },
    tooltip: { trigger: 'axis' },
    animation: !store.isRealtimeActive, // Disable animation for real-time updates
    series: [
      { type: 'line', name: 'Total devices', data: s.total, smooth: true, lineStyle: { color: '#3b82f6' } },
      { type: 'line', name: 'Online', data: s.online, smooth: true, lineStyle: { color: '#10b981' } },
      { type: 'line', name: 'With drift', data: s.drift, smooth: true, lineStyle: { color: '#ef4444' } }
    ]
  }
})

// Device status distribution (online/offline/...) from the latest snapshot.
const devicesOptions = computed(() => {
  const counts = store.deviceStatusCounts
  return {
    xAxis: { type: 'category', data: Object.keys(counts) },
    yAxis: { type: 'value' },
    tooltip: { trigger: 'axis' },
    series: [{ type: 'bar', data: Object.values(counts) }]
  }
})

// Drift by severity from drift_metrics.severity_distribution.
const driftOptions = computed(() => {
  const dist = store.drift?.severity_distribution ?? {}
  return {
    xAxis: { type: 'category', data: Object.keys(dist) },
    yAxis: { type: 'value' },
    tooltip: { trigger: 'axis' },
    series: [{ type: 'bar', data: Object.values(dist) }]
  }
})

// Most recent live events first, capped for display.
const recentEvents = computed(() => [...store.events].slice(-20).reverse())

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

/* Live events feed */
.events-section { margin-top: 24px; }
.events-empty { color: #6b7280; font-style: italic; padding: 8px 0; }
.events-list { list-style: none; margin: 12px 0 0; padding: 0; max-height: 240px; overflow-y: auto; }
.event-item {
  display: grid;
  grid-template-columns: 130px 1fr auto;
  gap: 12px;
  align-items: center;
  padding: 6px 10px;
  border-left: 3px solid #9ca3af;
  border-bottom: 1px solid #f3f4f6;
}
.event-item.event-warning { border-left-color: #f59e0b; }
.event-item.event-critical, .event-item.event-error { border-left-color: #ef4444; }
.event-kind { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.03em; color: #6b7280; }
.event-time { font-size: 0.8rem; color: #9ca3af; }

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
