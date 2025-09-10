<template>
  <div class="schedule-filters">
    <label>
      Plugin:
      <input 
        v-model="localPlugin" 
        placeholder="Filter by plugin name" 
        @input="onPlugin" 
        class="filter-input"
      />
    </label>
    <label>
      Status:
      <select v-model="localEnabled" @change="onEnabled" class="filter-select">
        <option value="">All</option>
        <option value="true">Enabled</option>
        <option value="false">Disabled</option>
      </select>
    </label>
    <div class="filter-actions">
      <button 
        v-if="hasFilters" 
        @click="clearFilters"
        class="clear-button"
        title="Clear all filters"
      >
        ✖ Clear
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'

const props = defineProps<{ 
  plugin?: string
  enabled?: boolean | undefined 
}>()

const emit = defineEmits<{ 
  'update:plugin': [string]
  'update:enabled': [boolean | undefined] 
}>()

const localPlugin = ref(props.plugin || '')
const localEnabled = ref(props.enabled === undefined ? '' : String(props.enabled))

const hasFilters = computed(() => {
  return localPlugin.value !== '' || localEnabled.value !== ''
})

function onPlugin() {
  emit('update:plugin', localPlugin.value)
}

function onEnabled() {
  if (localEnabled.value === '') {
    emit('update:enabled', undefined)
  } else {
    emit('update:enabled', localEnabled.value === 'true')
  }
}

function clearFilters() {
  localPlugin.value = ''
  localEnabled.value = ''
  emit('update:plugin', '')
  emit('update:enabled', undefined)
}

// Watch for external changes
watch(() => props.plugin, (value) => {
  localPlugin.value = value || ''
})

watch(() => props.enabled, (value) => {
  localEnabled.value = value === undefined ? '' : String(value)
})
</script>

<style scoped>
.schedule-filters {
  display: flex;
  gap: 16px;
  align-items: center;
  padding: 16px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.schedule-filters label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
  color: #374151;
  font-size: 0.875rem;
}

.filter-input,
.filter-select {
  padding: 6px 12px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  font-size: 0.875rem;
  transition: border-color 0.2s, box-shadow 0.2s;
  background: white;
}

.filter-input:focus,
.filter-select:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.filter-input {
  min-width: 200px;
}

.filter-select {
  min-width: 120px;
  cursor: pointer;
}

.filter-actions {
  margin-left: auto;
  display: flex;
  gap: 8px;
}

.clear-button {
  background: #f3f4f6;
  color: #6b7280;
  border: 1px solid #d1d5db;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 0.875rem;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
}

.clear-button:hover {
  background: #e5e7eb;
  color: #374151;
}

/* Responsive design */
@media (max-width: 768px) {
  .schedule-filters {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .schedule-filters label {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .filter-input,
  .filter-select {
    width: 100%;
  }

  .filter-actions {
    margin-left: 0;
    align-self: flex-start;
  }
}
</style>