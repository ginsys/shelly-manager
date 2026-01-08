<template>
  <div class="config-view">
    <div v-if="!config || Object.keys(config).length === 0" class="empty-state">
      No configuration data
    </div>

    <div v-else class="config-sections">
      <div 
        v-for="(value, key) in config" 
        :key="key"
        class="config-section"
      >
        <div 
          class="section-header"
          @click="toggleSection(key as string)"
        >
          <span class="section-icon">{{ getSectionIcon(key as string) }}</span>
          <span class="section-title">{{ formatSectionTitle(key as string) }}</span>
          <span v-if="showSourceBadges && sources?.[key]" class="source-badge">
            {{ sources[key] }}
          </span>
          <span class="expand-icon">{{ expandedSections[key] ? '−' : '+' }}</span>
        </div>

        <div v-if="expandedSections[key]" class="section-content">
          <template v-if="isObject(value)">
            <div 
              v-for="(fieldValue, fieldKey) in value" 
              :key="fieldKey"
              class="field-row"
            >
              <span class="field-label">{{ formatLabel(fieldKey as string) }}:</span>
              <span class="field-value" :class="getValueClass(fieldValue)">
                {{ formatValue(fieldValue) }}
              </span>
              <span v-if="showSourceBadges && sources?.[`${key}.${fieldKey}`]" class="source-badge small">
                {{ sources[`${key}.${fieldKey}`] }}
              </span>
            </div>
          </template>
          <template v-else>
            <div class="field-row">
              <span class="field-value" :class="getValueClass(value)">
                {{ formatValue(value) }}
              </span>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

interface Props {
  config: Record<string, any> | null
  sources?: Record<string, string>
  showSourceBadges?: boolean
  expandAll?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  showSourceBadges: false,
  expandAll: false
})

const expandedSections = ref<Record<string, boolean>>({})

watch(() => props.config, (newConfig) => {
  if (newConfig) {
    Object.keys(newConfig).forEach(key => {
      if (expandedSections.value[key] === undefined) {
        expandedSections.value[key] = props.expandAll
      }
    })
  }
}, { immediate: true })

function toggleSection(key: string) {
  expandedSections.value[key] = !expandedSections.value[key]
}

function getSectionIcon(key: string): string {
  const icons: Record<string, string> = {
    wifi: '📶',
    mqtt: '📡',
    auth: '🔐',
    system: '⚙️',
    cloud: '☁️',
    location: '📍',
    relay: '🔌',
    led: '💡',
    power_metering: '⚡',
    input: '🔘',
    coiot: '🔗',
    dimming: '🌗',
    roller: '🪟',
    color: '🎨',
    temp_protection: '🌡️',
    schedule: '📅',
    energy_meter: '📊',
    motion: '👁️',
    sensor: '🌡️'
  }
  return icons[key] || '📋'
}

function formatSectionTitle(key: string): string {
  const titles: Record<string, string> = {
    wifi: 'WiFi',
    mqtt: 'MQTT',
    auth: 'Authentication',
    system: 'System',
    cloud: 'Cloud',
    location: 'Location',
    relay: 'Relay',
    led: 'LED Indicator',
    power_metering: 'Power Metering',
    input: 'Input',
    coiot: 'CoIoT',
    dimming: 'Dimming',
    roller: 'Roller/Shutter',
    color: 'Color/RGBW',
    temp_protection: 'Temperature Protection',
    schedule: 'Schedule',
    energy_meter: 'Energy Meter',
    motion: 'Motion',
    sensor: 'Sensor'
  }
  return titles[key] || formatLabel(key)
}

function formatLabel(key: string): string {
  return key
    .replace(/_/g, ' ')
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, str => str.toUpperCase())
    .trim()
}

function isObject(value: any): boolean {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function formatValue(value: any): string {
  if (value === null || value === undefined) return '—'
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (Array.isArray(value)) {
    if (value.length === 0) return '(empty)'
    return JSON.stringify(value)
  }
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function getValueClass(value: any): string {
  if (typeof value === 'boolean') return value ? 'value-true' : 'value-false'
  if (value === null || value === undefined) return 'value-null'
  return ''
}
</script>

<style scoped>
.config-view {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.empty-state {
  padding: 24px;
  text-align: center;
  color: #64748b;
  background: #f9fafb;
  border-radius: 6px;
}

.config-sections {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.config-section {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  overflow: hidden;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: #f9fafb;
  cursor: pointer;
  user-select: none;
}

.section-header:hover {
  background: #f1f5f9;
}

.section-icon {
  font-size: 16px;
}

.section-title {
  font-weight: 600;
  flex: 1;
}

.expand-icon {
  color: #64748b;
  font-weight: 600;
  font-size: 18px;
}

.section-content {
  padding: 12px;
  background: white;
  border-top: 1px solid #e5e7eb;
}

.field-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid #f1f5f9;
}

.field-row:last-child {
  border-bottom: none;
}

.field-label {
  font-size: 13px;
  color: #64748b;
  min-width: 140px;
}

.field-value {
  font-size: 13px;
  font-family: ui-monospace, monospace;
  word-break: break-all;
}

.value-true {
  color: #059669;
}

.value-false {
  color: #dc2626;
}

.value-null {
  color: #9ca3af;
}

.source-badge {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  background: #dbeafe;
  color: #1e40af;
  margin-left: auto;
}

.source-badge.small {
  font-size: 10px;
  padding: 1px 6px;
}
</style>
