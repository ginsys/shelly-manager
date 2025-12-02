<template>
  <div class="category-section">
    <div class="category-header">
      <div class="category-title">
        <span class="category-icon">{{ info(categoryName).icon }}</span>
        <h2>{{ info(categoryName).display_name }}</h2>
        <span class="category-count">({{ plugins.length }})</span>
      </div>
      <p class="category-description">{{ info(categoryName).description }}</p>
    </div>

    <div class="plugins-grid" data-testid="plugin-list">
      <PluginCard
        v-for="plugin in plugins"
        :key="plugin.name"
        :plugin="plugin"
        :status-class="statusClass(plugin.status)"
        :status-text="statusText(plugin.status)"
        :status-icon="statusIcon(plugin.status)"
        :current-loading="currentLoading"
        :is-testing="isTesting(plugin.name)"
        :test-result="getTestResult(plugin.name)"
        @open-config="$emit('open-config', plugin)"
        @toggle="$emit('toggle', plugin)"
        @test="$emit('test', plugin)"
        @view-details="$emit('view-details', plugin)"
      />
    </div>

    <div v-if="plugins.length === 0" class="empty-category">
      <p>No plugins found in this category matching current filters.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import PluginCard from '@/components/plugins/PluginCard.vue'
import { getPluginCategoryInfo, formatPluginStatus, type Plugin } from '@/api/plugin'

defineProps<{
  categoryName: string
  plugins: Plugin[]
  currentLoading: boolean
  isTesting: (name: string) => boolean
  getTestResult: (name: string) => any
  statusClass: (s: Plugin['status']) => string
}>()

defineEmits<{ 'open-config': [Plugin]; 'toggle': [Plugin]; 'test': [Plugin]; 'view-details': [Plugin] }>()

function info(name: string) { return getPluginCategoryInfo(name) }
function statusText(s: Plugin['status']) { return formatPluginStatus(s).text }
function statusIcon(s: Plugin['status']) { return formatPluginStatus(s).icon }
</script>

<style scoped>
.category-section{margin-top:16px}
.category-header{display:flex;flex-direction:column;gap:4px;margin-bottom:8px}
.category-title{display:flex;align-items:center;gap:8px}
.category-icon{font-size:1.2rem}
.category-description{color:#6b7280;margin:0}
.category-count{color:#6b7280;font-size:.875rem;font-weight:500}
.plugins-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(320px,1fr));gap:16px}
.empty-category{text-align:center;padding:24px;color:#6b7280;font-style:italic}
</style>

