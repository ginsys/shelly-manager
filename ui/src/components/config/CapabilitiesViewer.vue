<template>
  <div class="capabilities-viewer">
    <div class="capabilities-header">
      <h3>Device Capabilities</h3>
      <span v-if="capabilities.firmwareVersion" class="firmware-version">
        Firmware: {{ capabilities.firmwareVersion }}
      </span>
    </div>

    <div class="capabilities-section">
      <h4>Device Type</h4>
      <span class="device-type">{{ capabilities.deviceType }}</span>
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

    <div class="capabilities-section">
      <h4>Supported Features</h4>
      <table class="features-table">
        <thead>
          <tr>
            <th>Feature</th>
            <th>Supported</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(supported, feature) in capabilities.supportedFeatures"
            :key="feature"
            :class="{ supported }"
          >
            <td>{{ formatFeatureName(feature) }}</td>
            <td>
              <span v-if="supported" class="status-icon success">✓</span>
              <span v-else class="status-icon">✗</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { DeviceCapabilities } from '@/api/typedConfig'

interface Props {
  capabilities: DeviceCapabilities
}

defineProps<Props>()

function formatFeatureName(feature: string): string {
  return feature
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, str => str.toUpperCase())
    .trim()
}
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

.features-table {
  width: 100%;
  border-collapse: collapse;
}

.features-table thead {
  background: #f9fafb;
}

.features-table th {
  padding: 10px 12px;
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid #e5e7eb;
}

.features-table td {
  padding: 10px 12px;
  border-bottom: 1px solid #f3f4f6;
  font-size: 14px;
}

.features-table tbody tr:hover {
  background: #f9fafb;
}

.features-table tbody tr.supported {
  background: #f0fdf4;
}

.status-icon {
  display: inline-block;
  width: 20px;
  height: 20px;
  text-align: center;
  line-height: 20px;
  border-radius: 50%;
  font-weight: bold;
  font-size: 14px;
}

.status-icon.success {
  background: #dcfce7;
  color: #16a34a;
}

.status-icon:not(.success) {
  background: #fee2e2;
  color: #dc2626;
}
</style>
