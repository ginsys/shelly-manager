<template>
  <div
    class="plugin-card"
    :data-testid="`plugin-card-${plugin.name}`"
  >
    <!-- Plugin Header -->
    <div class="plugin-header">
      <div class="plugin-title">
        <h3>{{ plugin.display_name }}</h3>
        <span class="plugin-version">v{{ plugin.version }}</span>
      </div>

      <div class="plugin-status">
        <!-- Backend hardcodes status; present registration only, not
             configured/enabled state (#266). -->
        <span class="status-indicator registered" title="Registered">Registered</span>
      </div>
    </div>

    <!-- Plugin Description -->
    <p class="plugin-description">{{ plugin.description }}</p>

    <!-- Plugin Capabilities -->
    <div class="plugin-capabilities">
      <span
        v-for="capability in plugin.capabilities.slice(0, 3)"
        :key="capability"
        class="capability-badge"
      >
        {{ capability }}
      </span>
      <span
        v-if="plugin.capabilities.length > 3"
        class="capability-badge more"
      >
        +{{ plugin.capabilities.length - 3 }} more
      </span>
    </div>

    <!-- Test Result Display -->
    <div v-if="testResult" class="test-result">
      <div
        class="test-indicator"
        :class="{ success: testResult.success, error: !testResult.success }"
      >
        <span class="test-icon">
          {{ testResult.success ? '✅' : '❌' }}
        </span>
        <span class="test-text">
          Test: {{ testResult.success ? 'Passed' : 'Failed' }}
          <span v-if="testResult.duration_ms">
            ({{ testResult.duration_ms }}ms)
          </span>
        </span>
      </div>

      <div v-if="testResult.message" class="test-message">
        {{ testResult.message }}
      </div>
    </div>

    <!-- Plugin Actions -->
    <div class="plugin-actions">
      <button
        v-if="plugin.status.available"
        class="action-button configure-btn"
        @click="emit('configure', plugin)"
        :disabled="currentLoading"
      >
        ⚙️ {{ plugin.status.configured ? 'Reconfigure' : 'Configure' }}
      </button>

      <button
        v-if="plugin.status.configured"
        class="action-button toggle-btn"
        :class="{ enabled: plugin.status.enabled }"
        @click="emit('toggle', plugin)"
        :disabled="currentLoading"
      >
        {{ plugin.status.enabled ? '⏸️ Disable' : '▶️ Enable' }}
      </button>

      <button
        v-if="plugin.status.available"
        class="action-button test-btn"
        @click="emit('test', plugin)"
        :disabled="isPluginTesting || currentLoading"
      >
        <span v-if="isPluginTesting">⏳</span>
        <span v-else>🧪</span>
        Test
      </button>

      <button
        class="action-button details-btn"
        @click="emit('details', plugin)"
      >
        👁️ Details
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { type Plugin } from '@/api/plugin'

interface TestResult {
  success: boolean
  message?: string
  duration_ms?: number
}

interface Props {
  plugin: Plugin
  testResult?: TestResult
  isPluginTesting: boolean
  currentLoading: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  configure: [plugin: Plugin]
  toggle: [plugin: Plugin]
  test: [plugin: Plugin]
  details: [plugin: Plugin]
}>()
</script>

<style scoped>
.plugin-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 20px;
  background: #ffffff;
  transition: all 0.2s;
  position: relative;
}

.plugin-card:hover {
  border-color: #d1d5db;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.plugin-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
}

.plugin-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.plugin-title h3 {
  margin: 0;
  color: #1f2937;
  font-size: 1.125rem;
}

.plugin-version {
  color: #6b7280;
  font-size: 0.75rem;
  font-weight: 500;
}

.plugin-status {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-indicator.registered {
  font-size: 0.75rem;
  font-weight: 600;
  color: #475569;
  background: #e2e8f0;
  padding: 2px 8px;
  border-radius: 999px;
}

.plugin-description {
  color: #4b5563;
  font-size: 0.875rem;
  margin: 0 0 16px 0;
  line-height: 1.5;
}

.plugin-capabilities {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 16px;
}

.capability-badge {
  background: #f3f4f6;
  color: #374151;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.capability-badge.more {
  background: #e5e7eb;
  color: #6b7280;
}

.test-result {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 4px;
  padding: 8px;
  margin-bottom: 16px;
}

.test-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.75rem;
  margin-bottom: 4px;
}

.test-indicator.success .test-text {
  color: #10b981;
}

.test-indicator.error .test-text {
  color: #ef4444;
}

.test-message {
  font-size: 0.75rem;
  color: #6b7280;
  line-height: 1.4;
}

.plugin-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.action-button {
  background: #f3f4f6;
  border: 1px solid #d1d5db;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
}

.action-button:hover:not(:disabled) {
  background: #e5e7eb;
}

.action-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.configure-btn:hover:not(:disabled) {
  background: #dbeafe;
  border-color: #3b82f6;
  color: #1e40af;
}

.toggle-btn.enabled:hover:not(:disabled) {
  background: #fef3c7;
  border-color: #f59e0b;
  color: #92400e;
}

.toggle-btn:not(.enabled):hover:not(:disabled) {
  background: #dcfce7;
  border-color: #10b981;
  color: #065f46;
}

.test-btn:hover:not(:disabled) {
  background: #ede9fe;
  border-color: #8b5cf6;
  color: #6b21a8;
}

.details-btn:hover:not(:disabled) {
  background: #f0f9ff;
  border-color: #0ea5e9;
  color: #0c4a6e;
}
</style>
