<template>
  <div class="filters-section">
    <div class="filter-row">
      <div class="filter-group">
        <label class="filter-label">Format:</label>
        <select v-model="localFilters.format" @change="handleFilterChange" class="filter-select">
          <option value="">All formats</option>
          <option value="terraform">Terraform</option>
          <option value="ansible">Ansible</option>
          <option value="kubernetes">Kubernetes</option>
          <option value="docker-compose">Docker Compose</option>
          <option value="yaml">YAML</option>
        </select>
      </div>
      <div class="filter-group">
        <label class="filter-label">Status:</label>
        <select v-model="localFilters.success" @change="handleFilterChange" class="filter-select">
          <option :value="undefined">All statuses</option>
          <option :value="true">Success only</option>
          <option :value="false">Failed only</option>
        </select>
      </div>
      <div class="filter-actions">
        <button @click="emit('refresh')" class="refresh-button" :disabled="loading">
          🔄 Refresh
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, watch } from 'vue'

interface Filters {
  format: string
  success?: boolean
}

interface Props {
  filters: Filters
  loading?: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:filters': [filters: Filters]
  'filter-change': []
  refresh: []
}>()

const localFilters = reactive<Filters>({
  format: props.filters.format,
  success: props.filters.success
})

watch(() => props.filters, (newFilters) => {
  localFilters.format = newFilters.format
  localFilters.success = newFilters.success
}, { deep: true })

watch(localFilters, (newFilters) => {
  emit('update:filters', { ...newFilters })
}, { deep: true })

function handleFilterChange() {
  emit('filter-change')
}
</script>

<style scoped>
.filters-section {
  margin-bottom: 24px;
  padding: 16px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.filter-row {
  display: flex;
  gap: 16px;
  align-items: flex-end;
  flex-wrap: wrap;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.filter-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #374151;
}

.filter-select {
  padding: 6px 10px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  background: white;
  font-size: 0.875rem;
}

.filter-actions {
  margin-left: auto;
}

.refresh-button {
  background: #10b981;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 4px;
  font-size: 0.875rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.refresh-button:hover:not(:disabled) {
  background: #059669;
}

.refresh-button:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}
</style>
