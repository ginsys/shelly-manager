<template>
  <div class="capabilities-viewer">
    <div class="capabilities-header">
      <h3>Device Capabilities</h3>
      <span class="firmware-version">Generation {{ capabilities.generation }}</span>
    </div>

    <div class="capabilities-section">
      <h4>Device Model</h4>
      <span class="device-type">{{ capabilities.device_model }}</span>
    </div>

    <div class="capabilities-section">
      <h4>Available Capabilities</h4>
      <div class="capabilities-list">
        <span
          v-for="capability in capabilities.capabilities"
          :key="capability"
          class="capability-badge"
        >
          {{ capability }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { DeviceCapabilities } from '@/api/typedConfig'

interface Props {
  capabilities: DeviceCapabilities
}

defineProps<Props>()
</script>

<style scoped>
.capabilities-viewer {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.capabilities-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 12px;
  border-bottom: 1px solid #e5e7eb;
}

.capabilities-header h3 {
  margin: 0;
  font-size: 18px;
  color: #1f2937;
}

.firmware-version {
  padding: 4px 12px;
  background: #f3f4f6;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  color: #6b7280;
}

.capabilities-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.capabilities-section h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: #374151;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.device-type {
  padding: 6px 12px;
  background: #e0e7ff;
  color: #3730a3;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 600;
  display: inline-block;
  width: fit-content;
}

.capabilities-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.capability-badge {
  padding: 6px 12px;
  background: #dbeafe;
  color: #1e40af;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 500;
}

</style>
