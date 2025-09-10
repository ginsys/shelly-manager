<template>
  <q-dialog v-model="isOpen" @before-show="loadPluginDetails">
    <q-card style="min-width: 700px; max-width: 900px">
      <q-card-section class="row items-center">
        <div class="text-h6">Plugin Details</div>
        <q-space />
        <q-btn icon="close" flat round dense v-close-popup />
      </q-card-section>

      <q-separator />

      <q-card-section v-if="loading" class="text-center q-py-xl">
        <q-spinner color="primary" size="lg" />
        <div class="q-mt-md">Loading plugin details...</div>
      </q-card-section>

      <q-card-section v-else-if="error" class="q-py-xl">
        <q-banner class="bg-red-1 text-red-8">
          <template v-slot:avatar>
            <q-icon name="error" color="red" />
          </template>
          <div class="text-weight-medium">Failed to load plugin details</div>
          <div>{{ error }}</div>
        </q-banner>
      </q-card-section>

      <div v-else-if="plugin" class="plugin-details">
        <!-- Plugin Header -->
        <q-card-section class="row items-start q-gutter-lg">
          <div class="col">
            <div class="row items-center q-gutter-md q-mb-md">
              <q-avatar :color="getPluginColor(plugin.category)" text-color="white" size="lg">
                <q-icon :name="getPluginIcon(plugin.category)" />
              </q-avatar>
              <div>
                <div class="text-h5">{{ plugin.name }}</div>
                <div class="text-subtitle1 text-grey-7">{{ plugin.description }}</div>
              </div>
            </div>

            <div class="row q-gutter-md">
              <q-chip 
                :color="getStatusColor(plugin.status)" 
                text-color="white" 
                :icon="getStatusIcon(plugin.status)"
              >
                {{ plugin.status }}
              </q-chip>
              <q-chip outline>
                <q-icon name="category" class="q-mr-xs" />
                {{ plugin.category }}
              </q-chip>
              <q-chip outline>
                <q-icon name="info" class="q-mr-xs" />
                v{{ plugin.version }}
              </q-chip>
            </div>
          </div>

          <div class="column q-gutter-sm">
            <q-btn 
              unelevated 
              color="primary" 
              icon="settings"
              label="Configure"
              @click="$emit('configure', plugin)"
            />
            <q-btn 
              outline 
              color="primary" 
              icon="play_arrow"
              label="Test"
              @click="testPlugin"
              :loading="testing"
            />
            <q-btn 
              flat 
              :color="plugin.enabled ? 'negative' : 'positive'"
              :icon="plugin.enabled ? 'pause' : 'play_arrow'"
              :label="plugin.enabled ? 'Disable' : 'Enable'"
              @click="togglePlugin"
            />
          </div>
        </q-card-section>

        <q-separator />

        <!-- Plugin Information -->
        <q-card-section>
          <q-tabs v-model="activeTab" class="text-grey-6" active-color="primary" indicator-color="primary">
            <q-tab name="overview" icon="info" label="Overview" />
            <q-tab name="capabilities" icon="featured_play_list" label="Capabilities" />
            <q-tab name="configuration" icon="settings" label="Configuration" />
            <q-tab name="usage" icon="bar_chart" label="Usage" />
            <q-tab name="health" icon="health_and_safety" label="Health" />
          </q-tabs>

          <q-separator />

          <q-tab-panels v-model="activeTab" animated>
            <!-- Overview Tab -->
            <q-tab-panel name="overview" class="q-pa-md">
              <div class="row q-gutter-lg">
                <div class="col-12 col-md-6">
                  <q-list>
                    <q-item>
                      <q-item-section avatar>
                        <q-icon name="person" />
                      </q-item-section>
                      <q-item-section>
                        <q-item-label>Author</q-item-label>
                        <q-item-label caption>{{ plugin.author || 'Unknown' }}</q-item-label>
                      </q-item-section>
                    </q-item>

                    <q-item>
                      <q-item-section avatar>
                        <q-icon name="event" />
                      </q-item-section>
                      <q-item-section>
                        <q-item-label>Created</q-item-label>
                        <q-item-label caption>{{ formatDate(plugin.created_at) }}</q-item-label>
                      </q-item-section>
                    </q-item>

                    <q-item>
                      <q-item-section avatar>
                        <q-icon name="update" />
                      </q-item-section>
                      <q-item-section>
                        <q-item-label>Last Updated</q-item-label>
                        <q-item-label caption>{{ formatDate(plugin.updated_at) }}</q-item-label>
                      </q-item-section>
                    </q-item>

                    <q-item v-if="plugin.license">
                      <q-item-section avatar>
                        <q-icon name="gavel" />
                      </q-item-section>
                      <q-item-section>
                        <q-item-label>License</q-item-label>
                        <q-item-label caption>{{ plugin.license }}</q-item-label>
                      </q-item-section>
                    </q-item>
                  </q-list>
                </div>

                <div class="col-12 col-md-6">
                  <div class="text-subtitle2 q-mb-md">Supported Formats</div>
                  <div class="q-gutter-xs">
                    <q-chip
                      v-for="format in plugin.supported_formats"
                      :key="format"
                      outline
                      size="sm"
                    >
                      {{ format }}
                    </q-chip>
                  </div>

                  <div class="text-subtitle2 q-mt-lg q-mb-md">Tags</div>
                  <div class="q-gutter-xs">
                    <q-chip
                      v-for="tag in plugin.tags"
                      :key="tag"
                      outline
                      size="sm"
                      color="blue-grey"
                    >
                      {{ tag }}
                    </q-chip>
                  </div>
                </div>
              </div>

              <!-- Long Description -->
              <div v-if="plugin.long_description" class="q-mt-lg">
                <div class="text-subtitle2 q-mb-md">Description</div>
                <div class="text-body2" style="line-height: 1.6">
                  {{ plugin.long_description }}
                </div>
              </div>
            </q-tab-panel>

            <!-- Capabilities Tab -->
            <q-tab-panel name="capabilities" class="q-pa-md">
              <div class="row q-gutter-lg">
                <div class="col-12 col-md-6">
                  <div class="text-subtitle2 q-mb-md">Export Capabilities</div>
                  <q-list>
                    <q-item v-for="capability in plugin.capabilities?.export || []" :key="capability">
                      <q-item-section avatar>
                        <q-icon name="check_circle" color="green" />
                      </q-item-section>
                      <q-item-section>{{ capability }}</q-item-section>
                    </q-item>
                  </q-list>
                </div>

                <div class="col-12 col-md-6">
                  <div class="text-subtitle2 q-mb-md">Import Capabilities</div>
                  <q-list>
                    <q-item v-for="capability in plugin.capabilities?.import || []" :key="capability">
                      <q-item-section avatar>
                        <q-icon name="check_circle" color="blue" />
                      </q-item-section>
                      <q-item-section>{{ capability }}</q-item-section>
                    </q-item>
                  </q-list>
                </div>
              </div>

              <!-- Features -->
              <div class="q-mt-lg">
                <div class="text-subtitle2 q-mb-md">Features</div>
                <div class="row q-gutter-sm">
                  <div v-for="feature in plugin.features" :key="feature" class="col-12 col-sm-6 col-md-4">
                    <q-card flat bordered class="q-pa-md text-center">
                      <q-icon name="star" color="amber" size="md" />
                      <div class="text-body2 q-mt-sm">{{ feature }}</div>
                    </q-card>
                  </div>
                </div>
              </div>
            </q-tab-panel>

            <!-- Configuration Tab -->
            <q-tab-panel name="configuration" class="q-pa-md">
              <div v-if="plugin.configured">
                <div class="text-subtitle2 q-mb-md">Current Configuration</div>
                <q-card flat bordered>
                  <q-card-section>
                    <pre class="config-preview">{{ JSON.stringify(plugin.config, null, 2) }}</pre>
                  </q-card-section>
                </q-card>

                <div class="q-mt-md">
                  <q-btn 
                    unelevated 
                    color="primary" 
                    icon="edit"
                    label="Edit Configuration"
                    @click="$emit('configure', plugin)"
                  />
                  <q-btn 
                    flat 
                    color="negative" 
                    icon="clear"
                    label="Clear Configuration"
                    @click="clearConfiguration"
                    class="q-ml-md"
                  />
                </div>
              </div>
              
              <div v-else class="text-center q-py-xl">
                <q-icon name="settings" size="4rem" color="grey-4" />
                <div class="text-h6 q-mt-md text-grey-6">Not Configured</div>
                <div class="text-body2 text-grey-6 q-mb-lg">This plugin has not been configured yet.</div>
                <q-btn 
                  unelevated 
                  color="primary" 
                  icon="settings"
                  label="Configure Now"
                  @click="$emit('configure', plugin)"
                />
              </div>
            </q-tab-panel>

            <!-- Usage Tab -->
            <q-tab-panel name="usage" class="q-pa-md">
              <div class="row q-gutter-lg">
                <div class="col-12 col-md-6">
                  <div class="text-subtitle2 q-mb-md">Usage Statistics</div>
                  <q-list>
                    <q-item>
                      <q-item-section>
                        <q-item-label>Total Exports</q-item-label>
                        <q-item-label caption>{{ plugin.usage_stats?.total_exports || 0 }}</q-item-label>
                      </q-item-section>
                    </q-item>
                    <q-item>
                      <q-item-section>
                        <q-item-label>Successful Exports</q-item-label>
                        <q-item-label caption>{{ plugin.usage_stats?.successful_exports || 0 }}</q-item-label>
                      </q-item-section>
                    </q-item>
                    <q-item>
                      <q-item-section>
                        <q-item-label>Failed Exports</q-item-label>
                        <q-item-label caption>{{ plugin.usage_stats?.failed_exports || 0 }}</q-item-label>
                      </q-item-section>
                    </q-item>
                    <q-item>
                      <q-item-section>
                        <q-item-label>Last Used</q-item-label>
                        <q-item-label caption>{{ formatDate(plugin.usage_stats?.last_used) || 'Never' }}</q-item-label>
                      </q-item-section>
                    </q-item>
                  </q-list>
                </div>

                <div class="col-12 col-md-6">
                  <div class="text-subtitle2 q-mb-md">Performance</div>
                  <q-list>
                    <q-item>
                      <q-item-section>
                        <q-item-label>Average Duration</q-item-label>
                        <q-item-label caption>{{ plugin.performance?.avg_duration || 'N/A' }}</q-item-label>
                      </q-item-section>
                    </q-item>
                    <q-item>
                      <q-item-section>
                        <q-item-label>Success Rate</q-item-label>
                        <q-item-label caption>{{ plugin.performance?.success_rate || 'N/A' }}</q-item-label>
                      </q-item-section>
                    </q-item>
                  </q-list>
                </div>
              </div>

              <!-- Recent Activity -->
              <div class="q-mt-lg">
                <div class="text-subtitle2 q-mb-md">Recent Activity</div>
                <q-timeline v-if="plugin.recent_activity?.length">
                  <q-timeline-entry
                    v-for="activity in plugin.recent_activity"
                    :key="activity.id"
                    :title="activity.action"
                    :subtitle="formatDate(activity.timestamp)"
                    :color="activity.success ? 'green' : 'red'"
                    :icon="activity.success ? 'check' : 'error'"
                  >
                    {{ activity.description }}
                  </q-timeline-entry>
                </q-timeline>
                <div v-else class="text-grey-6 text-center q-py-md">
                  No recent activity
                </div>
              </div>
            </q-tab-panel>

            <!-- Health Tab -->
            <q-tab-panel name="health" class="q-pa-md">
              <div class="row q-gutter-lg">
                <div class="col-12 col-md-6">
                  <div class="text-subtitle2 q-mb-md">Health Status</div>
                  <q-list>
                    <q-item>
                      <q-item-section avatar>
                        <q-icon 
                          :name="getStatusIcon(plugin.health?.status || 'unknown')"
                          :color="getStatusColor(plugin.health?.status || 'unknown')"
                        />
                      </q-item-section>
                      <q-item-section>
                        <q-item-label>Overall Status</q-item-label>
                        <q-item-label caption>{{ plugin.health?.status || 'Unknown' }}</q-item-label>
                      </q-item-section>
                    </q-item>
                    <q-item>
                      <q-item-section avatar>
                        <q-icon name="schedule" />
                      </q-item-section>
                      <q-item-section>
                        <q-item-label>Last Check</q-item-label>
                        <q-item-label caption>{{ formatDate(plugin.health?.last_check) || 'Never' }}</q-item-label>
                      </q-item-section>
                    </q-item>
                  </q-list>
                </div>

                <div class="col-12 col-md-6">
                  <div class="text-subtitle2 q-mb-md">Dependencies</div>
                  <q-list>
                    <q-item 
                      v-for="dep in plugin.health?.dependencies || []"
                      :key="dep.name"
                    >
                      <q-item-section avatar>
                        <q-icon 
                          :name="dep.status === 'ok' ? 'check_circle' : 'error'"
                          :color="dep.status === 'ok' ? 'green' : 'red'"
                        />
                      </q-item-section>
                      <q-item-section>
                        <q-item-label>{{ dep.name }}</q-item-label>
                        <q-item-label caption>{{ dep.version || 'Unknown version' }}</q-item-label>
                      </q-item-section>
                    </q-item>
                  </q-list>
                </div>
              </div>

              <!-- Health Messages -->
              <div v-if="plugin.health?.messages?.length" class="q-mt-lg">
                <div class="text-subtitle2 q-mb-md">Health Messages</div>
                <div v-for="message in plugin.health.messages" :key="message.id" class="q-mb-sm">
                  <q-banner 
                    :class="`bg-${message.type === 'warning' ? 'orange' : 'red'}-1 text-${message.type === 'warning' ? 'orange' : 'red'}-8`"
                  >
                    <template v-slot:avatar>
                      <q-icon 
                        :name="message.type === 'warning' ? 'warning' : 'error'" 
                        :color="message.type === 'warning' ? 'orange' : 'red'"
                      />
                    </template>
                    {{ message.text }}
                  </q-banner>
                </div>
              </div>

              <!-- Health Actions -->
              <div class="q-mt-lg">
                <q-btn 
                  unelevated 
                  color="primary" 
                  icon="refresh"
                  label="Check Health"
                  @click="checkHealth"
                  :loading="checkingHealth"
                />
              </div>
            </q-tab-panel>
          </q-tab-panels>
        </q-card-section>

        <!-- Footer Actions -->
        <q-separator />
        <q-card-actions align="right" class="q-pa-md">
          <q-btn flat label="Close" v-close-popup />
          <q-btn 
            flat 
            color="negative" 
            icon="delete"
            label="Uninstall"
            @click="confirmUninstall"
          />
          <q-btn 
            unelevated 
            color="primary" 
            icon="settings"
            label="Configure"
            @click="$emit('configure', plugin)"
          />
        </q-card-actions>
      </div>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useQuasar } from 'quasar'
import { usePluginStore } from '../stores/plugin'
import type { Plugin } from '../api/plugin'

interface Props {
  plugin: Plugin | null
  modelValue: boolean
}

interface Emits {
  (event: 'update:modelValue', value: boolean): void
  (event: 'configure', plugin: Plugin): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const $q = useQuasar()
const pluginStore = usePluginStore()

// Component State
const isOpen = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

const loading = ref(false)
const error = ref<string | null>(null)
const testing = ref(false)
const checkingHealth = ref(false)
const activeTab = ref('overview')

// Methods
const loadPluginDetails = async () => {
  if (!props.plugin) return

  loading.value = true
  error.value = null
  
  try {
    // Load additional plugin details if needed
    await pluginStore.getPlugin(props.plugin.name)
  } catch (err) {
    error.value = 'Failed to load plugin details'
    console.error('Failed to load plugin details:', err)
  } finally {
    loading.value = false
  }
}

const getPluginColor = (category: string): string => {
  const colors: Record<string, string> = {
    backup: 'blue',
    gitops: 'green',
    sync: 'purple',
    custom: 'orange'
  }
  return colors[category] || 'blue-grey'
}

const getPluginIcon = (category: string): string => {
  const icons: Record<string, string> = {
    backup: 'backup',
    gitops: 'sync',
    sync: 'sync_alt',
    custom: 'extension'
  }
  return icons[category] || 'extension'
}

const getStatusColor = (status: string): string => {
  const colors: Record<string, string> = {
    active: 'green',
    inactive: 'grey',
    error: 'red',
    warning: 'orange',
    healthy: 'green',
    unhealthy: 'red',
    unknown: 'grey'
  }
  return colors[status] || 'grey'
}

const getStatusIcon = (status: string): string => {
  const icons: Record<string, string> = {
    active: 'check_circle',
    inactive: 'pause_circle',
    error: 'error',
    warning: 'warning',
    healthy: 'check_circle',
    unhealthy: 'error',
    unknown: 'help'
  }
  return icons[status] || 'help'
}

const formatDate = (dateString?: string): string => {
  if (!dateString) return 'N/A'
  
  try {
    return new Date(dateString).toLocaleString()
  } catch {
    return 'Invalid date'
  }
}

const testPlugin = async () => {
  if (!props.plugin) return

  testing.value = true
  
  try {
    // Here you would call the plugin test API
    await new Promise(resolve => setTimeout(resolve, 2000))
    
    $q.notify({
      type: 'positive',
      message: `${props.plugin.name} test completed successfully`
    })
  } catch (err) {
    $q.notify({
      type: 'negative',
      message: `${props.plugin.name} test failed`
    })
  } finally {
    testing.value = false
  }
}

const togglePlugin = async () => {
  if (!props.plugin) return

  try {
    const action = props.plugin.enabled ? 'disable' : 'enable'
    
    // Here you would call the API to enable/disable the plugin
    await new Promise(resolve => setTimeout(resolve, 1000))
    
    $q.notify({
      type: 'positive',
      message: `Plugin ${action}d successfully`
    })
    
    // Update local state
    props.plugin.enabled = !props.plugin.enabled
  } catch (err) {
    $q.notify({
      type: 'negative',
      message: 'Failed to update plugin status'
    })
  }
}

const clearConfiguration = () => {
  $q.dialog({
    title: 'Clear Configuration',
    message: 'Are you sure you want to clear the plugin configuration? This action cannot be undone.',
    cancel: true,
    persistent: true
  }).onOk(async () => {
    if (!props.plugin) return

    try {
      // Here you would call the API to clear configuration
      await new Promise(resolve => setTimeout(resolve, 1000))
      
      $q.notify({
        type: 'positive',
        message: 'Configuration cleared successfully'
      })
      
      // Update local state
      props.plugin.configured = false
      props.plugin.config = undefined
    } catch (err) {
      $q.notify({
        type: 'negative',
        message: 'Failed to clear configuration'
      })
    }
  })
}

const checkHealth = async () => {
  if (!props.plugin) return

  checkingHealth.value = true
  
  try {
    // Here you would call the health check API
    await new Promise(resolve => setTimeout(resolve, 2000))
    
    $q.notify({
      type: 'positive',
      message: 'Health check completed'
    })
  } catch (err) {
    $q.notify({
      type: 'negative',
      message: 'Health check failed'
    })
  } finally {
    checkingHealth.value = false
  }
}

const confirmUninstall = () => {
  $q.dialog({
    title: 'Uninstall Plugin',
    message: `Are you sure you want to uninstall "${props.plugin?.name}"? This will remove all configuration and cannot be undone.`,
    cancel: true,
    persistent: true,
    color: 'negative'
  }).onOk(async () => {
    if (!props.plugin) return

    try {
      // Here you would call the uninstall API
      await new Promise(resolve => setTimeout(resolve, 2000))
      
      $q.notify({
        type: 'positive',
        message: 'Plugin uninstalled successfully'
      })
      
      isOpen.value = false
    } catch (err) {
      $q.notify({
        type: 'negative',
        message: 'Failed to uninstall plugin'
      })
    }
  })
}
</script>

<style scoped>
.plugin-details {
  max-height: 80vh;
  overflow-y: auto;
}

.config-preview {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  font-family: 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.4;
  max-height: 300px;
  overflow-y: auto;
}

.q-timeline {
  max-height: 400px;
  overflow-y: auto;
}
</style>