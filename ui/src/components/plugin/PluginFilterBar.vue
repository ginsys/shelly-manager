<template>
  <section class="filters-section">
    <div class="filter-row">
      <div class="filter-group">
        <label class="filter-label">Category:</label>
        <select
          :value="selectedCategory"
          @change="emit('update:selectedCategory', ($event.target as HTMLSelectElement).value)"
          class="filter-select"
        >
          <option value="">All Categories</option>
          <option
            v-for="category in categories"
            :key="category.name"
            :value="category.name"
          >
            {{ getCategoryDisplay(category.name) }} ({{ category.plugin_count }})
          </option>
        </select>
      </div>

      <div class="filter-group">
        <label class="filter-label">Status:</label>
        <select
          :value="statusFilter"
          @change="emit('update:statusFilter', ($event.target as HTMLSelectElement).value)"
          class="filter-select"
        >
          <option value="">All Statuses</option>
          <option value="configured">Configured & Enabled</option>
          <option value="available">Available (Not Configured)</option>
          <option value="disabled">Configured & Disabled</option>
          <option value="error">Error</option>
        </select>
      </div>

      <div class="filter-group search-group">
        <label class="filter-label">Search:</label>
        <input
          :value="searchQuery"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
          type="text"
          placeholder="Search plugins, capabilities..."
          class="search-input"
        />
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { getPluginCategoryInfo } from '@/api/plugin'

interface PluginCategory {
  name: string
  plugin_count: number
}

interface Props {
  selectedCategory: string
  statusFilter: string
  searchQuery: string
  categories: PluginCategory[]
}

defineProps<Props>()

const emit = defineEmits<{
  'update:selectedCategory': [value: string]
  'update:statusFilter': [value: string]
  'update:searchQuery': [value: string]
}>()

function getCategoryDisplay(categoryName: string): string {
  return getPluginCategoryInfo(categoryName).display_name
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

.search-group {
  flex: 1;
  min-width: 200px;
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
  min-width: 180px;
}

.search-input {
  padding: 6px 10px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  font-size: 0.875rem;
  width: 100%;
}
</style>
