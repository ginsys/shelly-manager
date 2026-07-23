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

    <!-- Plugin Actions. Configuration, testing and enable/disable were removed
         as unbacked (#264); only read-only inspection remains. -->
    <div class="plugin-actions">
      <button
        class="action-button view-schema-btn"
        @click="emit('view-schema', plugin)"
      >
        📄 View schema
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

interface Props {
  plugin: Plugin
}

defineProps<Props>()

const emit = defineEmits<{
  'view-schema': [plugin: Plugin]
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

.view-schema-btn:hover:not(:disabled) {
  background: #dbeafe;
  border-color: #3b82f6;
  color: #1e40af;
}

.details-btn:hover:not(:disabled) {
  background: #f0f9ff;
  border-color: #0ea5e9;
  color: #0c4a6e;
}
</style>
