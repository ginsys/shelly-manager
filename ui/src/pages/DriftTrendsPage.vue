<template>
  <div class="page">
    <div class="page-header">
      <h1>Drift Trends</h1>
      <div class="filters">
        <select v-model="days" class="select" @change="handleDaysChange">
          <option :value="7">Last 7 Days</option>
          <option :value="14">Last 14 Days</option>
          <option :value="30">Last 30 Days</option>
          <option :value="60">Last 60 Days</option>
          <option :value="90">Last 90 Days</option>
        </select>
      </div>
    </div>

    <div v-if="store.loading" class="loading">Loading trends...</div>
    <div v-else-if="store.error" class="error">{{ store.error }}</div>

    <div v-else class="content">
      <div v-if="store.trends.length === 0" class="empty">
        <p>No drift trends data available for the selected period.</p>
      </div>

      <div v-else>
        <!-- Summary Cards -->
        <div class="summary-cards">
          <div class="summary-card">
            <div class="card-icon">📊</div>
            <div class="card-content">
              <div class="card-label">Total Drifts</div>
              <div class="card-value">{{ totalDrifts }}</div>
            </div>
          </div>

          <div class="summary-card">
            <div class="card-icon success">✓</div>
            <div class="card-content">
              <div class="card-label">Resolved</div>
              <div class="card-value">{{ totalResolved }}</div>
            </div>
          </div>

          <div class="summary-card">
            <div class="card-icon warning">⚠</div>
            <div class="card-content">
              <div class="card-label">Unresolved</div>
              <div class="card-value">{{ totalUnresolved }}</div>
            </div>
          </div>

          <div class="summary-card">
            <div class="card-icon">📱</div>
            <div class="card-content">
              <div class="card-label">Avg Devices/Day</div>
              <div class="card-value">{{ avgDevices }}</div>
            </div>
          </div>
        </div>

        <!-- Trend Chart -->
        <div class="chart-container">
          <h2>Drift Detection Over Time</h2>
          <div class="chart">
            <div class="chart-y-axis">
              <div class="y-axis-label">Count</div>
              <div class="y-axis-ticks">
                <span v-for="tick in yAxisTicks" :key="tick">{{ tick }}</span>
              </div>
            </div>
            <div class="chart-content">
              <div class="chart-bars">
                <div
                  v-for="(trend, index) in store.trends"
                  :key="index"
                  class="bar-group"
                >
                  <div class="bar-stack">
                    <div
                      class="bar resolved"
                      :style="{
                        height: getBarHeight(trend.resolvedDrifts) + '%'
                      }"
                      :title="`Resolved: ${trend.resolvedDrifts}`"
                    />
                    <div
                      class="bar unresolved"
                      :style="{
                        height: getBarHeight(trend.unresolvedDrifts) + '%'
                      }"
                      :title="`Unresolved: ${trend.unresolvedDrifts}`"
                    />
                  </div>
                  <div class="bar-label">
                    {{ formatDate(trend.date) }}
                  </div>
                </div>
              </div>
              <div class="chart-legend">
                <div class="legend-item">
                  <span class="legend-color resolved"></span>
                  <span>Resolved</span>
                </div>
                <div class="legend-item">
                  <span class="legend-color unresolved"></span>
                  <span>Unresolved</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Detailed Table -->
        <div class="trends-table-container">
          <h2>Detailed Breakdown</h2>
          <table class="trends-table">
            <thead>
              <tr>
                <th>Date</th>
                <th>Total Drifts</th>
                <th>Resolved</th>
                <th>Unresolved</th>
                <th>Devices Checked</th>
                <th>Resolution Rate</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(trend, index) in store.trends" :key="index">
                <td>{{ formatDateLong(trend.date) }}</td>
                <td>{{ trend.totalDrifts }}</td>
                <td class="resolved-count">{{ trend.resolvedDrifts }}</td>
                <td class="unresolved-count">{{ trend.unresolvedDrifts }}</td>
                <td>{{ trend.deviceCount }}</td>
                <td>
                  <div class="resolution-rate">
                    <div class="rate-bar">
                      <div
                        class="rate-fill"
                        :style="{ width: getResolutionRate(trend) + '%' }"
                      ></div>
                    </div>
                    <span class="rate-text">{{ getResolutionRate(trend) }}%</span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useDriftStore } from '@/stores/drift'

const store = useDriftStore()
const days = ref(30)

const totalDrifts = computed(() => {
  return store.trends.reduce((sum, t) => sum + t.totalDrifts, 0)
})

const totalResolved = computed(() => {
  return store.trends.reduce((sum, t) => sum + t.resolvedDrifts, 0)
})

const totalUnresolved = computed(() => {
  return store.trends.reduce((sum, t) => sum + t.unresolvedDrifts, 0)
})

const avgDevices = computed(() => {
  if (store.trends.length === 0) return 0
  const total = store.trends.reduce((sum, t) => sum + t.deviceCount, 0)
  return Math.round(total / store.trends.length)
})

const maxDrifts = computed(() => {
  return Math.max(...store.trends.map(t => t.totalDrifts), 1)
})

const yAxisTicks = computed(() => {
  const max = maxDrifts.value
  const step = Math.ceil(max / 5)
  return Array.from({ length: 6 }, (_, i) => max - i * step).filter(n => n >= 0)
})

function getBarHeight(value: number): number {
  if (maxDrifts.value === 0) return 0
  return (value / maxDrifts.value) * 100
}

function getResolutionRate(trend: { totalDrifts: number; resolvedDrifts: number }): number {
  if (trend.totalDrifts === 0) return 0
  return Math.round((trend.resolvedDrifts / trend.totalDrifts) * 100)
}

function formatDate(dateStr: string): string {
  const date = new Date(dateStr)
  return `${date.getMonth() + 1}/${date.getDate()}`
}

function formatDateLong(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString()
}

async function handleDaysChange() {
  await store.fetchTrends(days.value)
}

onMounted(() => {
  store.fetchTrends(days.value)
})
</script>

<style scoped>
.page {
  padding: 20px;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0;
  font-size: 24px;
  color: #1f2937;
}

.select {
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 14px;
  background: white;
}

.loading,
.error,
.empty {
  padding: 32px;
  text-align: center;
  color: #64748b;
}

.error {
  color: #b91c1c;
  background: #fee2e2;
  border-radius: 8px;
}

.summary-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 32px;
}

.summary-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.card-icon {
  font-size: 32px;
  flex-shrink: 0;
}

.card-icon.success {
  color: #16a34a;
}

.card-icon.warning {
  color: #ea580c;
}

.card-content {
  flex: 1;
}

.card-label {
  font-size: 13px;
  color: #6b7280;
  margin-bottom: 4px;
}

.card-value {
  font-size: 28px;
  font-weight: 700;
  color: #1f2937;
}

.chart-container {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 32px;
}

.chart-container h2 {
  margin: 0 0 24px 0;
  font-size: 18px;
  color: #1f2937;
}

.chart {
  display: flex;
  gap: 16px;
}

.chart-y-axis {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 60px;
}

.y-axis-label {
  font-size: 12px;
  font-weight: 600;
  color: #6b7280;
  text-align: center;
}

.y-axis-ticks {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  height: 300px;
  font-size: 12px;
  color: #6b7280;
}

.chart-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.chart-bars {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  height: 300px;
  padding: 0 8px;
  border-left: 1px solid #e5e7eb;
  border-bottom: 1px solid #e5e7eb;
}

.bar-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  min-width: 40px;
}

.bar-stack {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column-reverse;
  justify-content: flex-start;
}

.bar {
  width: 100%;
  min-height: 2px;
  border-radius: 4px 4px 0 0;
  transition: opacity 0.2s;
}

.bar:hover {
  opacity: 0.8;
}

.bar.resolved {
  background: #86efac;
}

.bar.unresolved {
  background: #fbbf24;
}

.bar-label {
  font-size: 11px;
  color: #6b7280;
  text-align: center;
  transform: rotate(-45deg);
  transform-origin: center;
  white-space: nowrap;
  margin-top: 20px;
}

.chart-legend {
  display: flex;
  gap: 16px;
  justify-content: center;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #6b7280;
}

.legend-color {
  width: 16px;
  height: 16px;
  border-radius: 4px;
}

.legend-color.resolved {
  background: #86efac;
}

.legend-color.unresolved {
  background: #fbbf24;
}

.trends-table-container {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 24px;
}

.trends-table-container h2 {
  margin: 0 0 16px 0;
  font-size: 18px;
  color: #1f2937;
}

.trends-table {
  width: 100%;
  border-collapse: collapse;
}

.trends-table thead {
  background: #f9fafb;
}

.trends-table th {
  padding: 12px;
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: #6b7280;
  text-transform: uppercase;
  border-bottom: 1px solid #e5e7eb;
}

.trends-table td {
  padding: 12px;
  border-bottom: 1px solid #f3f4f6;
  font-size: 14px;
}

.trends-table tbody tr:hover {
  background: #f9fafb;
}

.resolved-count {
  color: #16a34a;
  font-weight: 600;
}

.unresolved-count {
  color: #ea580c;
  font-weight: 600;
}

.resolution-rate {
  display: flex;
  align-items: center;
  gap: 8px;
}

.rate-bar {
  flex: 1;
  height: 8px;
  background: #e5e7eb;
  border-radius: 4px;
  overflow: hidden;
}

.rate-fill {
  height: 100%;
  background: #86efac;
  transition: width 0.3s;
}

.rate-text {
  font-size: 13px;
  font-weight: 600;
  color: #6b7280;
  min-width: 40px;
}
</style>
