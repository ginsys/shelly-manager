<template>
  <q-card style="min-width: 700px; max-width: 900px">
    <q-card-section class="row items-center">
      <div class="text-h6">Plugin Details</div>
      <q-space />
      <q-btn icon="close" flat round dense @click="emit('close')" />
    </q-card-section>

    <q-separator />

    <div v-if="plugin" class="plugin-details">
      <!-- Plugin Header -->
      <q-card-section class="row items-start q-gutter-lg">
        <div class="col">
          <div class="row items-center q-gutter-md q-mb-md">
            <q-avatar :color="getPluginColor(plugin.category)" text-color="white" size="lg">
              <q-icon :name="getPluginIcon(plugin.category)" />
            </q-avatar>
            <div>
              <div class="text-h5">{{ plugin.display_name || plugin.name }}</div>
              <div class="text-subtitle1 text-grey-7">{{ plugin.description }}</div>
            </div>
          </div>

          <div class="row q-gutter-md">
            <!-- Backend hardcodes status; present registration only, not
                 configured/enabled state (#266). -->
            <q-chip color="blue-grey" text-color="white" icon="check_circle">
              Registered
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
      </q-card-section>

      <q-separator />

      <!-- Plugin Information (read-only; configuration/toggle live in the
           plugin management flow — deferred until the backend routes exist, #264). -->
      <q-card-section>
        <q-tabs v-model="activeTab" class="text-grey-6" active-color="primary" indicator-color="primary">
          <q-tab name="overview" icon="info" label="Overview" />
          <q-tab name="capabilities" icon="featured_play_list" label="Capabilities" />
        </q-tabs>

        <q-separator />

        <q-tab-panels v-model="activeTab" animated>
          <!-- Overview Tab -->
          <q-tab-panel name="overview" class="q-pa-md">
            <q-list>
              <q-item>
                <q-item-section avatar>
                  <q-icon name="badge" />
                </q-item-section>
                <q-item-section>
                  <q-item-label>Name</q-item-label>
                  <q-item-label caption>{{ plugin.name }}</q-item-label>
                </q-item-section>
              </q-item>

              <q-item>
                <q-item-section avatar>
                  <q-icon name="label" />
                </q-item-section>
                <q-item-section>
                  <q-item-label>Display Name</q-item-label>
                  <q-item-label caption>{{ plugin.display_name || '—' }}</q-item-label>
                </q-item-section>
              </q-item>

              <q-item>
                <q-item-section avatar>
                  <q-icon name="category" />
                </q-item-section>
                <q-item-section>
                  <q-item-label>Category</q-item-label>
                  <q-item-label caption>{{ plugin.category }}</q-item-label>
                </q-item-section>
              </q-item>

              <q-item>
                <q-item-section avatar>
                  <q-icon name="info" />
                </q-item-section>
                <q-item-section>
                  <q-item-label>Version</q-item-label>
                  <q-item-label caption>v{{ plugin.version }}</q-item-label>
                </q-item-section>
              </q-item>
            </q-list>

            <div v-if="plugin.description" class="q-mt-lg">
              <div class="text-subtitle2 q-mb-md">Description</div>
              <div class="text-body2" style="line-height: 1.6">
                {{ plugin.description }}
              </div>
            </div>
          </q-tab-panel>

          <!-- Capabilities Tab -->
          <q-tab-panel name="capabilities" class="q-pa-md">
            <div class="text-subtitle2 q-mb-md">Capabilities</div>
            <q-list v-if="plugin.capabilities.length">
              <q-item v-for="capability in plugin.capabilities" :key="capability">
                <q-item-section avatar>
                  <q-icon name="check_circle" color="green" />
                </q-item-section>
                <q-item-section>{{ capability }}</q-item-section>
              </q-item>
            </q-list>
            <div v-else class="text-grey-6 text-center q-py-md">
              No capabilities reported
            </div>
          </q-tab-panel>
        </q-tab-panels>
      </q-card-section>

      <q-separator />
      <q-card-actions align="right" class="q-pa-md">
        <q-btn flat label="Close" @click="emit('close')" />
      </q-card-actions>
    </div>
  </q-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { Plugin } from '../api/plugin'

interface Props {
  plugin: Plugin | null
}

interface Emits {
  (event: 'close'): void
}

defineProps<Props>()
const emit = defineEmits<Emits>()

const activeTab = ref('overview')

// This view is read-only and renders entirely from the list-DTO `plugin` prop
// (GET /export/plugins). Status is presented as "Registered" only — the backend
// hardcodes configured/enabled, so those are not shown as meaningful (#266).
// Configuration/enable/disable are not here; their backend routes do not exist
// (#264).

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
</script>

<style scoped>
.plugin-details {
  max-height: 80vh;
  overflow-y: auto;
}
</style>
