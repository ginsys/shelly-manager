<template>
  <div class="plugin-card" :class="cardStateClass">
    <!-- Header -->
    <div class="plugin-header">
      <div class="plugin-title">
        <h3>{{ plugin.display_name }}</h3>
        <span class="plugin-version">v{{ plugin.version }}</span>
      </div>
      <div class="plugin-status">
        <span class="status-indicator" :class="statusClass" :title="statusText">
          {{ statusIcon }}
        </span>
      </div>
    </div>

    <!-- Description -->
    <p class="plugin-description">{{ plugin.description }}</p>

    <!-- Capabilities -->
    <div class="plugin-capabilities">
      <span v-for="cap in plugin.capabilities.slice(0,3)" :key="cap" class="capability-badge">{{ cap }}</span>
      <span v-if="plugin.capabilities.length > 3" class="capability-badge more">+{{ plugin.capabilities.length - 3 }} more</span>
    </div>

    <!-- Health -->
    <div v-if="plugin.health" class="plugin-health">
      <div class="health-indicator" :class="{ healthy: plugin.health.healthy }">
        {{ plugin.health.healthy ? '💚' : '💔' }}
        <span class="health-text">{{ plugin.health.healthy ? 'Healthy' : 'Issues Detected' }}</span>
      </div>
      <div v-if="plugin.health.issues?.length" class="health-issues">
        <div class="issues-summary">⚠️ {{ plugin.health.issues.length }} issue{{ plugin.health.issues.length !== 1 ? 's' : '' }}</div>
      </div>
    </div>

    <!-- Error -->
    <div v-if="plugin.status.error" class="plugin-error">
      <span class="error-icon">⚠️</span>
      <span class="error-text">{{ plugin.status.error }}</span>
    </div>

    <!-- Test Result -->
    <div v-if="testResult" class="test-result">
      <div class="test-indicator" :class="{ success: testResult.success, error: !testResult.success }">
        <span class="test-icon">{{ testResult.success ? '✅' : '❌' }}</span>
        <span class="test-text">
          Test: {{ testResult.success ? 'Passed' : 'Failed' }}
          <span v-if="testResult.duration_ms">({{ testResult.duration_ms }}ms)</span>
        </span>
      </div>
      <div v-if="testResult.message" class="test-message">{{ testResult.message }}</div>
    </div>

    <!-- Actions -->
    <div class="plugin-actions">
      <button v-if="plugin.status.available" class="action-button configure-btn" @click="$emit('open-config')" :disabled="currentLoading">
        ⚙️ {{ plugin.status.configured ? 'Reconfigure' : 'Configure' }}
      </button>
      <button v-if="plugin.status.configured" class="action-button toggle-btn" :class="{ enabled: plugin.status.enabled }" @click="$emit('toggle')" :disabled="currentLoading">
        {{ plugin.status.enabled ? '⏸️ Disable' : '▶️ Enable' }}
      </button>
      <button v-if="plugin.status.available" class="action-button test-btn" @click="$emit('test')" :disabled="isTesting || currentLoading">
        <span v-if="isTesting">⏳</span><span v-else>🧪</span> Test
      </button>
      <button class="action-button details-btn" @click="$emit('view-details')">👁️ Details</button>
    </div>

    <!-- Metadata -->
    <div v-if="plugin.status.last_used" class="plugin-metadata">
      <span class="metadata-item">Last used: {{ formatDate(plugin.status.last_used) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Plugin } from '@/api/plugin'

const props = defineProps<{
  plugin: Plugin
  statusClass: string
  statusText: string
  statusIcon: string
  currentLoading: boolean
  isTesting: boolean
  testResult?: { success: boolean; duration_ms?: number; message?: string } | null
}>()

defineEmits<{
  'open-config': []
  'toggle': []
  'test': []
  'view-details': []
}>()

const cardStateClass = computed(() => {
  const st = props.plugin.status
  if (st.error) return 'error'
  if (!st.available) return 'unavailable'
  if (st.configured && st.enabled) return 'ready'
  if (st.configured && !st.enabled) return 'disabled'
  if (!st.configured) return 'not-configured'
  return ''
})

function formatDate(dateString: string) {
  try { return new Date(dateString).toLocaleString() } catch { return dateString }
}
</script>

<style scoped>
.plugin-card { border: 1px solid #e5e7eb; border-radius: 8px; padding: 20px; background: #ffffff; transition: all 0.2s; position: relative; }
.plugin-card:hover { border-color: #d1d5db; box-shadow: 0 2px 4px rgba(0,0,0,.1) }
.plugin-card.ready { border-left: 4px solid #10b981 }
.plugin-card.not-configured { border-left: 4px solid #3b82f6 }
.plugin-card.disabled { border-left: 4px solid #f59e0b }
.plugin-card.error { border-left: 4px solid #ef4444 }
.plugin-card.unavailable { border-left: 4px solid #9ca3af; opacity: .7 }
.plugin-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 12px }
.plugin-title { display: flex; flex-direction: column; gap: 4px }
.plugin-title h3 { margin: 0; color: #1f2937; font-size: 1.125rem }
.plugin-version { color: #6b7280; font-size: .75rem; font-weight: 500 }
.plugin-status { display: flex; align-items: center; gap: 6px }
.status-indicator { font-size: 1.25rem; cursor: help }
.plugin-description { color: #4b5563; font-size: .875rem; margin: 0 0 16px 0; line-height: 1.5 }
.plugin-capabilities { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 16px }
.capability-badge { background: #f3f4f6; color: #374151; padding: 2px 6px; border-radius: 3px; font-size: .75rem; font-weight: 500 }
.capability-badge.more { background: #e5e7eb; color: #6b7280 }
.plugin-health { margin-bottom: 16px }
.health-indicator { display: flex; align-items: center; gap: 8px; font-size: .875rem }
.health-indicator .health-text { font-weight: 600 }
.health-indicator.healthy .health-text { color: #059669 }
.health-indicator:not(.healthy) .health-text { color: #dc2626 }
.plugin-error { display: grid; grid-template-columns: 20px 1fr; gap: 8px; align-items: flex-start; padding: 8px; background: #fff7ed; border-radius: 4px; border: 1px solid #fecaca; margin-bottom: 16px }
.plugin-error .error-icon { color: #ef4444; font-size: .875rem; margin-top: 1px }
.plugin-error .error-text { color: #dc2626; font-size: .75rem; line-height: 1.4 }
.test-result { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 4px; padding: 8px; margin-bottom: 16px }
.test-indicator { display: flex; align-items: center; gap: 6px; font-size: .75rem; margin-bottom: 4px }
.test-indicator.success .test-text { color: #10b981 }
.test-indicator.error .test-text { color: #ef4444 }
.test-message { font-size: .75rem; color: #6b7280; line-height: 1.4 }
.plugin-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px }
.action-button { background: #f3f4f6; border: 1px solid #d1d5db; padding: 6px 12px; border-radius: 4px; font-size: .75rem; font-weight: 500; cursor: pointer; transition: all .2s; display: flex; align-items: center; gap: 4px }
.action-button:hover:not(:disabled) { background: #e5e7eb }
.action-button:disabled { opacity: .5; cursor: not-allowed }
.configure-btn:hover:not(:disabled) { background: #dbeafe; border-color: #3b82f6; color: #1e40af }
.toggle-btn.enabled:hover:not(:disabled) { background: #fef3c7; border-color: #f59e0b; color: #92400e }
.toggle-btn:not(.enabled):hover:not(:disabled) { background: #dcfce7; border-color: #10b981; color: #065f46 }
.test-btn:hover:not(:disabled) { background: #ede9fe; border-color: #8b5cf6; color: #6b21a8 }
.details-btn:hover:not(:disabled) { background: #f0f9ff; border-color: #0ea5e9; color: #0c4a6e }
.plugin-metadata { display: flex; gap: 12px; padding-top: 8px; border-top: 1px solid #f3f4f6 }
.metadata-item { color: #6b7280; font-size: .75rem }
</style>
