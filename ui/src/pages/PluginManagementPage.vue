<template>
  <main style="padding: 16px">
    <div class="page-header">
      <h1>Plugin Management</h1>
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
    <section class="stats-section">
      <div class="stats">
        <div class="card">
          <span class="stat-label">Total Plugins:</span>
          <span class="stat-value">{{ pluginStats.total }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Configured:</span>
          <span class="stat-value success">{{ pluginStats.configured }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Available:</span>
          <span class="stat-value available">{{ pluginStats.available }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Disabled:</span>
          <span class="stat-value disabled">{{ pluginStats.disabled }}</span>
        </div>
        <div class="card">
          <span class="stat-label">Issues:</span>
          <span class="stat-value error">{{ pluginStats.error }}</span>
        </div>
      </div>
    </section>

    <!-- Filters and Search -->
    <section class="filters-section">
      <div class="filter-row">
        <div class="filter-group">
          <label class="filter-label">Category:</label>
          <select 
            v-model="selectedCategory" 
            @change="pluginStore.setCategory($event.target.value)" 
            class="filter-select"
          >
            <option value="">All Categories</option>
            <option 
              v-for="category in categories" 
              :key="category.name" 
              :value="category.name"
            >
              {{ getPluginCategoryInfo(category.name).display_name }} ({{ category.plugin_count }})
            </option>
          </select>
        </div>

        <div class="filter-group">
          <label class="filter-label">Status:</label>
          <select 
            v-model="statusFilter" 
            @change="pluginStore.setStatusFilter($event.target.value)" 
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
            v-model="searchQuery"
            @input="pluginStore.setSearchQuery($event.target.value)"
            type="text"
            placeholder="Search plugins, capabilities..."
            class="search-input"
          />
        </div>
      </div>
    </section>

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
        <div class="plugins-grid">
          <div 
            v-for="plugin in categoryPlugins" 
            :key="plugin.name"
            class="plugin-card"
            :class="getPluginStatusClass(plugin.status)"
          >
            <!-- Plugin Header -->
            <div class="plugin-header">
              <div class="plugin-title">
                <h3>{{ plugin.display_name }}</h3>
                <span class="plugin-version">v{{ plugin.version }}</span>
              </div>
              
              <div class="plugin-status">
                <span 
                  class="status-indicator"
                  :class="getPluginStatusClass(plugin.status)"
                  :title="formatPluginStatus(plugin.status).text"
                >
                  {{ formatPluginStatus(plugin.status).icon }}
                </span>
              </div>
            </div>

            <!-- Plugin Description -->
            <p class="plugin-description">{{ plugin.description }}</p>

            <!-- Plugin Capabilities -->
            <div class="plugin-capabilities">
              <span 
                v-for="capability in plugin.capabilities.slice(0, 3)" 
                :key="capability"
                class="capability-badge"
              >
                {{ capability }}
              </span>
              <span 
                v-if="plugin.capabilities.length > 3" 
                class="capability-badge more"
              >
                +{{ plugin.capabilities.length - 3 }} more
              </span>
            </div>

            <!-- Plugin Health (if available) -->
            <div v-if="plugin.health" class="plugin-health">
              <div class="health-indicator" :class="{ healthy: plugin.health.healthy }">
                {{ plugin.health.healthy ? '💚' : '💔' }}
                <span class="health-text">
                  {{ plugin.health.healthy ? 'Healthy' : 'Issues Detected' }}
                </span>
              </div>
              
              <div v-if="plugin.health.issues?.length" class="health-issues">
                <div class="issues-summary">
                  ⚠️ {{ plugin.health.issues.length }} issue{{ plugin.health.issues.length !== 1 ? 's' : '' }}
                </div>
              </div>
            </div>

            <!-- Error Display -->
            <div v-if="plugin.status.error" class="plugin-error">
              <span class="error-icon">⚠️</span>
              <span class="error-text">{{ plugin.status.error }}</span>
            </div>

            <!-- Test Result Display -->
            <div v-if="getTestResult(plugin.name)" class="test-result">
              <div 
                class="test-indicator"
                :class="{ success: getTestResult(plugin.name)?.success, error: !getTestResult(plugin.name)?.success }"
              >
                <span class="test-icon">
                  {{ getTestResult(plugin.name)?.success ? '✅' : '❌' }}
                </span>
                <span class="test-text">
                  Test: {{ getTestResult(plugin.name)?.success ? 'Passed' : 'Failed' }}
                  <span v-if="getTestResult(plugin.name)?.duration_ms">
                    ({{ getTestResult(plugin.name)?.duration_ms }}ms)
                  </span>
                </span>
              </div>
              
              <div v-if="getTestResult(plugin.name)?.message" class="test-message">
                {{ getTestResult(plugin.name)?.message }}
              </div>
            </div>

            <!-- Plugin Actions -->
            <div class="plugin-actions">
              <button
                v-if="plugin.status.available"
                class="action-button configure-btn"
                @click="openConfigModal(plugin)"
                :disabled="currentLoading"
              >
                ⚙️ {{ plugin.status.configured ? 'Reconfigure' : 'Configure' }}
              </button>

              <button
                v-if="plugin.status.configured"
                class="action-button toggle-btn"
                :class="{ enabled: plugin.status.enabled }"
                @click="togglePlugin(plugin)"
                :disabled="currentLoading"
              >
                {{ plugin.status.enabled ? '⏸️ Disable' : '▶️ Enable' }}
              </button>

              <button
                v-if="plugin.status.available"
                class="action-button test-btn"
                @click="testPlugin(plugin)"
                :disabled="isPluginTesting(plugin.name) || currentLoading"
              >
                <span v-if="isPluginTesting(plugin.name)">⏳</span>
                <span v-else>🧪</span>
                Test
              </button>

              <button
                class="action-button details-btn"
                @click="viewPluginDetails(plugin)"
              >
                👁️ Details
              </button>
            </div>

            <!-- Last Used Info -->
            <div v-if="plugin.status.last_used" class="plugin-metadata">
              <span class="metadata-item">
                Last used: {{ formatDate(plugin.status.last_used) }}
              </span>
            </div>
          </div>
        </div>

        <!-- Empty State for Category -->
        <div v-if="categoryPlugins.length === 0" class="empty-category">
          <p>No plugins found in this category matching current filters.</p>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-if="!loading && filteredPlugins.length === 0" class="empty-state">
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
        <PluginConfigForm
          v-if="configModalPlugin"
          :plugin="configModalPlugin"
          @close="closeConfigModal"
          @saved="handleConfigSaved"
        />
      </div>
    </div>

    <!-- Plugin Details Modal -->
    <div v-if="showDetailsModal" class="modal-overlay" @click="closeDetailsModal">
      <div class="modal-content details-modal" @click.stop>
        <PluginDetailsView
          v-if="detailsModalPlugin"
          :plugin="detailsModalPlugin"
          @close="closeDetailsModal"
          @configure="openConfigFromDetails"
        />
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
import { computed, onMounted, reactive, ref } from 'vue'
import { usePluginStore } from '@/stores/plugin'
import { 
  getPluginCategoryInfo,
  formatPluginStatus,
  type Plugin
} from '@/api/plugin'
import PluginConfigForm from '@/components/PluginConfigForm.vue'
import PluginDetailsView from '@/components/PluginDetailsView.vue'

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

// Initialize
onMounted(() => {
  refreshData()
})

/**
 * Refresh all plugin data
 */
async function refreshData() {
  try {
    await pluginStore.refresh()
    showMessage('Plugin list refreshed successfully', 'success')
  } catch (err: any) {
    showMessage(err.message || 'Failed to refresh plugin list', 'error')
  }
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
 * Format date for display
 */
function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString()
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

.stats-section {
  margin-bottom: 24px;
}

.stats {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.card {
  border: 1px solid #e5e7eb;
  padding: 16px;
  border-radius: 6px;
  background: #ffffff;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 140px;
}

.stat-label {
  font-weight: 500;
  color: #6b7280;
}

.stat-value {
  font-size: 1.25rem;
  font-weight: 600;
  color: #1f2937;
}

.stat-value.success {
  color: #10b981;
}

.stat-value.available {
  color: #3b82f6;
}

.stat-value.disabled {
  color: #f59e0b;
}

.stat-value.error {
  color: #ef4444;
}

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

.plugin-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 20px;
  background: #ffffff;
  transition: all 0.2s;
  position: relative;
}

.plugin-card:hover {
  border-color: #d1d5db;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.plugin-card.ready {
  border-left: 4px solid #10b981;
}

.plugin-card.not-configured {
  border-left: 4px solid #3b82f6;
}

.plugin-card.disabled {
  border-left: 4px solid #f59e0b;
}

.plugin-card.error {
  border-left: 4px solid #ef4444;
}

.plugin-card.unavailable {
  border-left: 4px solid #9ca3af;
  opacity: 0.7;
}

.plugin-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
}

.plugin-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.plugin-title h3 {
  margin: 0;
  color: #1f2937;
  font-size: 1.125rem;
}

.plugin-version {
  color: #6b7280;
  font-size: 0.75rem;
  font-weight: 500;
}

.plugin-status {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-indicator {
  font-size: 1.25rem;
  cursor: help;
}

.plugin-description {
  color: #4b5563;
  font-size: 0.875rem;
  margin: 0 0 16px 0;
  line-height: 1.5;
}

.plugin-capabilities {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 16px;
}

.capability-badge {
  background: #f3f4f6;
  color: #374151;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.capability-badge.more {
  background: #e5e7eb;
  color: #6b7280;
}

.plugin-health {
  margin-bottom: 16px;
}

.health-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.875rem;
  margin-bottom: 4px;
}

.health-indicator.healthy .health-text {
  color: #10b981;
}

.health-indicator:not(.healthy) .health-text {
  color: #ef4444;
}

.health-issues .issues-summary {
  color: #f59e0b;
  font-size: 0.75rem;
  font-weight: 500;
}

.plugin-error {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  background: #fef2f2;
  padding: 8px;
  border-radius: 4px;
  border: 1px solid #fecaca;
  margin-bottom: 16px;
}

.plugin-error .error-icon {
  color: #ef4444;
  font-size: 0.875rem;
  margin-top: 1px;
}

.plugin-error .error-text {
  color: #dc2626;
  font-size: 0.75rem;
  line-height: 1.4;
}

.test-result {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 4px;
  padding: 8px;
  margin-bottom: 16px;
}

.test-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.75rem;
  margin-bottom: 4px;
}

.test-indicator.success .test-text {
  color: #10b981;
}

.test-indicator.error .test-text {
  color: #ef4444;
}

.test-message {
  font-size: 0.75rem;
  color: #6b7280;
  line-height: 1.4;
}

.plugin-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.action-button {
  background: #f3f4f6;
  border: 1px solid #d1d5db;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
}

.action-button:hover:not(:disabled) {
  background: #e5e7eb;
}

.action-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.configure-btn:hover:not(:disabled) {
  background: #dbeafe;
  border-color: #3b82f6;
  color: #1e40af;
}

.toggle-btn.enabled:hover:not(:disabled) {
  background: #fef3c7;
  border-color: #f59e0b;
  color: #92400e;
}

.toggle-btn:not(.enabled):hover:not(:disabled) {
  background: #dcfce7;
  border-color: #10b981;
  color: #065f46;
}

.test-btn:hover:not(:disabled) {
  background: #ede9fe;
  border-color: #8b5cf6;
  color: #6b21a8;
}

.details-btn:hover:not(:disabled) {
  background: #f0f9ff;
  border-color: #0ea5e9;
  color: #0c4a6e;
}

.plugin-metadata {
  display: flex;
  gap: 12px;
  padding-top: 8px;
  border-top: 1px solid #f3f4f6;
}

.metadata-item {
  color: #6b7280;
  font-size: 0.75rem;
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

  .stats {
    flex-direction: column;
  }

  .filter-row {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .plugins-grid {
    grid-template-columns: 1fr;
  }

  .plugin-actions {
    flex-direction: column;
  }

  .action-button {
    width: 100%;
    justify-content: center;
  }

  .modal-content {
    margin: 8px;
    max-width: none;
    width: calc(100% - 16px);
  }
}
</style>