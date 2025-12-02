<template>
  <section class="filters-section">
    <div class="filter-row">
      <div class="filter-group">
        <label class="filter-label">Category:</label>
        <select class="filter-select" :value="selectedCategory" @change="onCategory">
          <option value="">All Categories</option>
          <option v-for="category in categories" :key="category.name" :value="category.name">
            {{ info(category.name).display_name }} ({{ category.plugin_count }})
          </option>
        </select>
      </div>

      <div class="filter-group">
        <label class="filter-label">Status:</label>
        <select class="filter-select" :value="statusFilter" @change="onStatus">
          <option value="">All Statuses</option>
          <option value="configured">Configured & Enabled</option>
          <option value="available">Available (Not Configured)</option>
          <option value="disabled">Configured & Disabled</option>
          <option value="error">Error</option>
        </select>
      </div>

      <div class="filter-group search-group">
        <label class="filter-label">Search:</label>
        <input class="search-input" type="text" :value="searchQuery" @input="onSearch" placeholder="Search plugins, capabilities..." />
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { getPluginCategoryInfo } from '@/api/plugin'

defineProps<{ 
  categories: Array<{ name: string; plugin_count: number }>
  selectedCategory: string
  statusFilter: string
  searchQuery: string
}>()

const emit = defineEmits<{
  'update:selectedCategory': [string]
  'update:statusFilter': [string]
  'update:searchQuery': [string]
}>()

function onCategory(e: Event) { emit('update:selectedCategory', (e.target as HTMLSelectElement).value) }
function onStatus(e: Event) { emit('update:statusFilter', (e.target as HTMLSelectElement).value) }
function onSearch(e: Event) { emit('update:searchQuery', (e.target as HTMLInputElement).value) }

function info(name: string) { return getPluginCategoryInfo(name) }
</script>

<style scoped>
.filters-section { margin: 16px 0 }
.filter-row { display: flex; gap: 12px; flex-wrap: wrap; align-items: flex-end }
.filter-group { display: grid; gap: 6px }
.filter-label { font-size: .875rem; color: #4b5563 }
.filter-select, .search-input { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px }
.search-input { min-width: 240px }
</style>

