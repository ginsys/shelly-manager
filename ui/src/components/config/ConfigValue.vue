<template>
  <span v-if="!isObject(value) && !isObjectList" class="field-value" :class="valueClass">{{ text }}</span>

  <div v-else-if="isObjectList" class="nested-list">
    <div v-for="(item, index) in (value as any[])" :key="index" class="nested-group" data-nested="item">
      <div class="nested-title">{{ itemTitle(item, index) }}</div>
      <ConfigValue :value="item" :path="`${path}.${index}`" :sources="sources" :show-source-badges="showSourceBadges" />
    </div>
  </div>

  <div v-else class="nested-fields">
    <div v-for="(fieldValue, fieldKey) in value" :key="fieldKey" class="field-row">
      <span class="field-label">{{ formatLabel(String(fieldKey)) }}:</span>
      <ConfigValue :value="fieldValue" :path="`${path}.${fieldKey}`" :sources="sources" :show-source-badges="showSourceBadges" />
      <span v-if="showSourceBadges && sources?.[`${path}.${fieldKey}`]" class="source-badge small">
        {{ sources[`${path}.${fieldKey}`] }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
// One value of a configuration, shown by its shape and never as JSON: an object is its fields, a list
// of objects is one group per item, a list of plain values is a comma list. `path` is the dotted key
// of the value in the server's sources map, where a list item is keyed by its position.
import { computed } from 'vue'
import { formatLabel } from '@/utils/config-form'

interface Props {
  value: any
  path: string
  sources?: Record<string, string>
  showSourceBadges?: boolean
}

const props = defineProps<Props>()

function isObject(value: any): boolean {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

const isObjectList = computed(() =>
  Array.isArray(props.value) && props.value.length > 0 && props.value.every(isObject))

const text = computed(() => {
  const value = props.value
  if (value === null || value === undefined) return '—'
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (Array.isArray(value)) return value.length === 0 ? '(empty)' : value.map(entry => formatEntry(entry)).join(', ')
  return String(value)
})

function formatEntry(entry: any): string {
  if (entry === null || entry === undefined) return '—'
  if (typeof entry === 'boolean') return entry ? 'Yes' : 'No'
  if (Array.isArray(entry)) return `[${entry.map(formatEntry).join(', ')}]`
  return String(entry)
}

const valueClass = computed(() => {
  if (typeof props.value === 'boolean') return props.value ? 'value-true' : 'value-false'
  if (props.value === null || props.value === undefined) return 'value-null'
  return ''
})

// An item is named by its name when it has one, else by its id, else by its position.
function itemTitle(item: Record<string, any>, index: number): string {
  if (typeof item.name === 'string' && item.name !== '') return item.name
  return `Item ${typeof item.id === 'number' ? item.id : index + 1}`
}
</script>

<style scoped>
.nested-fields, .nested-list { display: flex; flex-direction: column; gap: 4px; flex: 1; min-width: 0; }
.nested-fields { padding-left: 12px; border-left: 2px solid #e5e7eb; }
.nested-group { display: flex; flex-direction: column; gap: 2px; padding: 4px 0; }
.nested-title { font-weight: 600; font-size: 13px; color: #334155; }
.field-row { display: flex; align-items: baseline; gap: 8px; flex-wrap: wrap; }
.field-label { color: #64748b; font-size: 13px; }
.value-true { color: #15803d; }
.value-false { color: #b91c1c; }
.value-null { color: #94a3b8; }
.source-badge { font-size: 11px; padding: 1px 6px; border-radius: 999px; background: #e0e7ff; color: #3730a3; }
</style>
