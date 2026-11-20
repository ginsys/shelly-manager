<template>
  <div class="filters-section">
    <label class="filter-label">Status:</label>
    <select
      :value="filters.success === undefined ? '' : String(filters.success)"
      @change="onStatus(($event.target as HTMLSelectElement).value)"
      class="filter-select"
    >
      <option value="">All statuses</option>
      <option value="true">Success only</option>
      <option value="false">Failed only</option>
    </select>
    <button @click="emit('refresh')" class="refresh-button" :disabled="loading">🔄 Refresh</button>
  </div>
</template>

<script setup lang="ts">
interface Filters {
  success?: boolean
}

const props = defineProps<{ filters: Filters; loading?: boolean }>()

const emit = defineEmits<{
  'update:filters': [filters: Filters]
  'filter-change': []
  refresh: []
}>()

function onStatus(value: string) {
  emit('update:filters', { ...props.filters, success: value === '' ? undefined : value === 'true' })
  emit('filter-change')
}
</script>

<style scoped>
.filters-section { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.filter-label { font-size: 0.875rem; color: #374151; }
.filter-select { padding: 6px 8px; border: 1px solid #d1d5db; border-radius: 4px; }
.refresh-button { padding: 6px 12px; border: 1px solid #d1d5db; border-radius: 4px; background: white; cursor: pointer; }
</style>
