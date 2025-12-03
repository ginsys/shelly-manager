<template>
  <div class="column-toggle" data-testid="column-toggle">
    <button
      @click="isOpen = !isOpen"
      class="toggle-btn"
      data-testid="toggle-btn"
    >
      ⚙️ Columns
      <span class="toggle-icon">{{ isOpen ? '▲' : '▼' }}</span>
    </button>

    <div v-if="isOpen" class="dropdown" data-testid="column-dropdown">
      <div class="dropdown-header">
        <h4>Show/Hide Columns</h4>
        <button @click="selectAll" class="action-link" data-testid="select-all">All</button>
        <button @click="selectNone" class="action-link" data-testid="select-none">None</button>
      </div>
      <div class="column-list">
        <label
          v-for="col in columns"
          :key="col.key"
          class="column-item"
          :data-testid="`column-${col.key}`"
        >
          <input
            type="checkbox"
            :checked="visibleColumns[col.key]"
            @change="toggleColumn(col.key)"
          />
          <span>{{ col.label }}</span>
        </label>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

interface Column {
  key: string
  label: string
}

const props = defineProps<{
  columns: Column[]
  visibleColumns: Record<string, boolean>
}>()

const emit = defineEmits<{
  toggle: [columnKey: string]
  selectAll: []
  selectNone: []
}>()

const isOpen = ref(false)

function toggleColumn(key: string) {
  emit('toggle', key)
}

function selectAll() {
  emit('selectAll')
}

function selectNone() {
  emit('selectNone')
}
</script>

<style scoped>
.column-toggle {
  position: relative;
  display: inline-block;
}

.toggle-btn {
  padding: 0.5rem 1rem;
  background: white;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: #374151;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  transition: all 0.2s;
}

.toggle-btn:hover {
  background: #f9fafb;
  border-color: #9ca3af;
}

.toggle-icon {
  font-size: 0.75rem;
  margin-left: 0.25rem;
}

.dropdown {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 0.5rem;
  background: white;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
  z-index: 50;
  min-width: 200px;
}

.dropdown-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #e5e7eb;
}

.dropdown-header h4 {
  margin: 0;
  font-size: 0.875rem;
  font-weight: 600;
  color: #111827;
  flex: 1;
}

.action-link {
  background: none;
  border: none;
  color: #3b82f6;
  font-size: 0.75rem;
  cursor: pointer;
  padding: 0.25rem 0.5rem;
}

.action-link:hover {
  color: #2563eb;
  text-decoration: underline;
}

.column-list {
  padding: 0.5rem;
  max-height: 300px;
  overflow-y: auto;
}

.column-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem;
  cursor: pointer;
  border-radius: 0.25rem;
  transition: background-color 0.2s;
}

.column-item:hover {
  background: #f3f4f6;
}

.column-item input[type="checkbox"] {
  cursor: pointer;
}

.column-item span {
  font-size: 0.875rem;
  color: #374151;
}
</style>
