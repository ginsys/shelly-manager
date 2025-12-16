<template>
  <main style="padding: 16px" data-testid="plugin-management-page">
    <div class="page-header">
      <h1 data-testid="page-title">Plugin Management</h1>
      <div class="header-actions">
        <button 
          class="refresh-button" 
          @click="refreshData" 
          :disabled="loading"
          title="Refresh plugin list"
        >
          🔄 Refresh
        </button>
      </div>
    </div>

    <!-- Plugin Statistics -->
    <PluginStatistics :statistics="pluginStats" />

    <!-- Filters and Search -->
    <PluginFilterBar
      v-model:selectedCategory="selectedCategory"
      v-model:statusFilter="statusFilter"
      v-model:searchQuery="searchQuery"
      :categories="categories"
      @update:selectedCategory="pluginStore.setCategory($event)"
      @update:statusFilter="pluginStore.setStatusFilter($event)"
      @update:searchQuery="pluginStore.setSearchQuery($event)"
    />

    <!-- Error Display -->
    <div v-if="error" class="error-alert">
      <span class="error-icon">⚠️</span>
      {{ error }}
      <button class="error-dismiss" @click="pluginStore.clearErrors()">✖</button>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-section">
      <div class="loading-spinner">⏳</div>
      <p>Loading plugins...</p>
    </div>

    <!-- Plugin Categories -->
    <div v-else class="plugin-categories">
      <!-- Category Headers with Plugin Lists -->
      <div 
        v-for="[categoryName, categoryPlugins] in Object.entries(pluginsByCategory)" 
        :key="categoryName"
        class="category-section"
      >
        <div class="category-header">
          <div class="category-title">
            <span class="category-icon">{{ getPluginCategoryInfo(categoryName).icon }}</span>
            <h2>{{ getPluginCategoryInfo(categoryName).display_name }}</h2>
            <span class="category-count">({{ categoryPlugins.length }})</span>
          </div>
          <p class="category-description">{{ getPluginCategoryInfo(categoryName).description }}</p>
        </div>

        <!-- Plugin Grid -->
        <div class="plugins-grid" data-testid="plugin-list">
          <PluginCard
            v-for="plugin in categoryPlugins"
            :key="plugin.name"
            :plugin="plugin"
            :statusClass="getPluginStatusClass(plugin.status)"
            :testResult="getTestResult(plugin.name)"
            :isPluginTesting="isPluginTesting(plugin.name)"
            :currentLoading="currentLoading"
            @configure="openConfigModal"
            @toggle="togglePlugin"
            @test="testPlugin"
            @details="viewPluginDetails"
          />
        </div>

        <!-- Empty State for Category -->
        <div v-if="categoryPlugins.length === 0" class="empty-category">
          <p>No plugins found in this category matching current filters.</p>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-if="!loading && filteredPlugins.length === 0" class="empty-state" data-testid="empty-state">
      <div class="empty-icon">📦</div>
      <h3>No Plugins Found</h3>
      <p v-if="searchQuery || selectedCategory || statusFilter">
        Try adjusting your search criteria or clearing filters.
      </p>
      <p v-else>
        No plugins are available. Check your plugin directory or server configuration.
      </p>
      <button class="primary-button" @click="refreshData">
        🔄 Refresh Plugin List
      </button>
    </div>

    <!-- Plugin Configuration Modal -->
    <div v-if="showConfigModal" class="modal-overlay" @click="closeConfigModal">
      <div class="modal-content config-modal" @click.stop>
        <Suspense>
          <template #default>
            <PluginConfigForm
              v-if="configModalPlugin"
              :plugin="configModalPlugin"
              @close="closeConfigModal"
              @saved="handleConfigSaved"
            />
          </template>
          <template #fallback>
            <div class="modal-loading">Loading configuration form...</div>
          </template>
        </Suspense>
      </div>
    </div>

    <!-- Plugin Details Modal -->
    <div v-if="showDetailsModal" class="modal-overlay" @click="closeDetailsModal">
      <div class="modal-content details-modal" @click.stop>
        <Suspense>
          <template #default>
            <PluginDetailsView
              v-if="detailsModalPlugin"
              :plugin="detailsModalPlugin"
              @close="closeDetailsModal"
              @configure="openConfigFromDetails"
            />
          </template>
          <template #fallback>
            <div class="modal-loading">Loading plugin details...</div>
          </template>
        </Suspense>
      </div>
    </div>

    <!-- Success/Error Messages -->
    <div v-if="message.text" :class="['message', message.type]">
      {{ message.text }}
      <button class="message-close" @click="message.text = ''">✖</button>
    </div>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, defineAsyncComponent } from 'vue'
import { usePluginStore } from '@/stores/plugin'
import {
  getPluginCategoryInfo,
  formatPluginStatus,
  type Plugin
} from '@/api/plugin'
import PluginStatistics from '@/components/plugin/PluginStatistics.vue'
import PluginFilterBar from '@/components/plugin/PluginFilterBar.vue'
import PluginCard from '@/components/plugin/PluginCard.vue'

// Lazy load heavy form components
const PluginConfigForm = defineAsyncComponent(() => import('@/components/PluginConfigForm.vue'))
const PluginDetailsView = defineAsyncComponent(() => import('@/components/PluginDetailsView.vue'))

// Store
const pluginStore = usePluginStore()

// Computed properties from store
const plugins = computed(() => pluginStore.plugins)
const categories = computed(() => pluginStore.categories)
const filteredPlugins = computed(() => pluginStore.filteredPlugins)
const pluginsByCategory = computed(() => pluginStore.pluginsByCategory)
const pluginStats = computed(() => pluginStore.pluginStats)
const loading = computed(() => pluginStore.loading)
const error = computed(() => pluginStore.error)
const currentLoading = computed(() => pluginStore.currentLoading)

// Reactive filters
const selectedCategory = ref('')
const statusFilter = ref('')
const searchQuery = ref('')

// Modal state
const showConfigModal = ref(false)
const showDetailsModal = ref(false)
const configModalPlugin = ref<Plugin | null>(null)
const detailsModalPlugin = ref<Plugin | null>(null)

// Message state
const message = reactive({ 
  text: '', 
  type: 'success' as 'success' | 'error' 
})

// Initialize with non-blocking data loading
onMounted(() => {
  // Load data asynchronously without blocking page render
  refreshData()
})

/**
 * Refresh all plugin data (non-blocking)
 */
function refreshData() {
  // Fire and forget - don't block UI rendering
  pluginStore.refresh().catch(err => {
    console.warn('Plugin refresh failed:', err)
    showMessage(err.message || 'Failed to refresh plugin list', 'error')
  })
}

/**
 * Open configuration modal for a plugin
 */
function openConfigModal(plugin: Plugin) {
  configModalPlugin.value = plugin
  showConfigModal.value = true
}

/**
 * Close configuration modal
 */
function closeConfigModal() {
  showConfigModal.value = false
  configModalPlugin.value = null
}

/**
 * Handle configuration saved
 */
function handleConfigSaved(plugin: Plugin) {
  showMessage(`Configuration saved for ${plugin.display_name}`, 'success')
  closeConfigModal()
  refreshData() // Refresh to get updated status
}

/**
 * Open configuration modal from details view
 */
function openConfigFromDetails(plugin: Plugin) {
  closeDetailsModal()
  openConfigModal(plugin)
}

/**
 * View plugin details
 */
function viewPluginDetails(plugin: Plugin) {
  detailsModalPlugin.value = plugin
  showDetailsModal.value = true
}

/**
 * Close details modal
 */
function closeDetailsModal() {
  showDetailsModal.value = false
  detailsModalPlugin.value = null
}

/**
 * Toggle plugin enabled/disabled state
 */
async function togglePlugin(plugin: Plugin) {
  try {
    await pluginStore.togglePlugin(plugin.name)
    const action = plugin.status.enabled ? 'disabled' : 'enabled'
    showMessage(`${plugin.display_name} ${action} successfully`, 'success')
  } catch (err: any) {
    showMessage(err.message || `Failed to toggle ${plugin.display_name}`, 'error')
  }
}

/**
 * Test plugin configuration
 */
async function testPlugin(plugin: Plugin) {
  try {
    const result = await pluginStore.testPluginConfiguration(plugin.name)
    
    if (result.success) {
      showMessage(`${plugin.display_name} test passed successfully`, 'success')
    } else {
      showMessage(`${plugin.display_name} test failed: ${result.message || 'Unknown error'}`, 'error')
    }
  } catch (err: any) {
    showMessage(err.message || `Failed to test ${plugin.display_name}`, 'error')
  }
}

/**
 * Get plugin status CSS class
 */
function getPluginStatusClass(status: Plugin['status']) {
  return pluginStore.getPluginStatusClass(status)
}

/**
 * Check if plugin is being tested
 */
function isPluginTesting(name: string) {
  return pluginStore.isPluginTesting(name)
}

/**
 * Get test result for plugin
 */
function getTestResult(name: string) {
  return pluginStore.getTestResult(name)
}

/**
 * Show message
 */
function showMessage(text: string, type: 'success' | 'error') {
  message.text = text
  message.type = type
  
  // Auto-hide success messages
  if (type === 'success') {
    setTimeout(() => {
      if (message.text === text) {
        message.text = ''
      }
    }, 5000)
  }
}
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0;
  color: #1f2937;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.refresh-button {
  background: #10b981;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 0.875rem;
  cursor: pointer;
  transition: background-color 0.2s;
  display: flex;
  align-items: center;
  gap: 6px;
}

.refresh-button:hover:not(:disabled) {
  background: #059669;
}

.refresh-button:disabled {
  background: #9ca3af;
  cursor: not-allowed;
}

.error-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: #fee2e2;
  color: #dc2626;
  border: 1px solid #fecaca;
  border-radius: 6px;
  margin-bottom: 24px;
}

.error-icon {
  font-size: 1.125rem;
}

.error-dismiss {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  margin-left: auto;
  font-size: 1rem;
  padding: 2px;
}

.loading-section {
  text-align: center;
  padding: 40px;
  color: #6b7280;
}

.loading-spinner {
  font-size: 2rem;
  margin-bottom: 12px;
}

.plugin-categories {
  display: flex;
  flex-direction: column;
  gap: 32px;
}

.category-section {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 24px;
}

.category-header {
  margin-bottom: 20px;
}

.category-title {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.category-icon {
  font-size: 1.5rem;
}

.category-title h2 {
  margin: 0;
  color: #1f2937;
  font-size: 1.5rem;
}

.category-count {
  color: #6b7280;
  font-size: 0.875rem;
  font-weight: 500;
}

.category-description {
  color: #6b7280;
  margin: 0;
  font-size: 0.875rem;
}

.plugins-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.empty-category {
  text-align: center;
  padding: 40px;
  color: #6b7280;
  font-style: italic;
}

.empty-state {
  text-align: center;
  padding: 60px 40px;
  color: #6b7280;
}

.empty-icon {
  font-size: 4rem;
  margin-bottom: 16px;
}

.empty-state h3 {
  color: #374151;
  margin: 0 0 8px 0;
}

.empty-state p {
  margin: 0 0 24px 0;
}

.primary-button {
  background: #3b82f6;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  transition: background-color 0.2s;
}

.primary-button:hover {
  background: #2563eb;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 16px;
}

.modal-content {
  background: white;
  border-radius: 8px;
  max-width: 90vw;
  max-height: 90vh;
  overflow: auto;
}

.config-modal {
  width: 100%;
  max-width: 800px;
}

.details-modal {
  width: 100%;
  max-width: 700px;
}

.modal-loading {
  padding: 60px 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #6b7280;
  font-style: italic;
  min-height: 200px;
}

.message {
  position: fixed;
  top: 20px;
  right: 20px;
  padding: 12px 16px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  gap: 12px;
  z-index: 1001;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
}

.message.success {
  background: #dcfce7;
  color: #166534;
  border: 1px solid #bbf7d0;
}

.message.error {
  background: #fee2e2;
  color: #dc2626;
  border: 1px solid #fecaca;
}

.message-close {
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  font-size: 1.1rem;
  padding: 0;
  line-height: 1;
}

/* Responsive design */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }

  .plugins-grid {
    grid-template-columns: 1fr;
  }

  .modal-content {
    margin: 8px;
    max-width: none;
    width: calc(100% - 16px);
  }
}
</style>