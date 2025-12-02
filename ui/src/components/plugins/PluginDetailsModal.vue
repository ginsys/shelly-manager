<template>
  <div class="modal-overlay" @click="$emit('close')">
    <div class="modal-content details-modal" @click.stop>
      <Suspense>
        <template #default>
          <PluginDetailsView
            v-if="plugin"
            :plugin="plugin"
            @close="$emit('close')"
            @configure="$emit('configure', plugin)"
          />
        </template>
        <template #fallback>
          <div class="modal-loading">Loading plugin details...</div>
        </template>
      </Suspense>
    </div>
  </div>
</template>

<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import type { Plugin } from '@/api/plugin'

defineProps<{ plugin: Plugin | null }>()

defineEmits<{ close: []; configure: [Plugin] }>()

const PluginDetailsView = defineAsyncComponent(() => import('@/components/PluginDetailsView.vue'))
</script>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.5); display: flex; align-items: center; justify-content: center; z-index: 1000 }
.modal-content { background: #fff; border-radius: 8px; box-shadow: 0 25px 50px -12px rgba(0, 0, 0, .25); padding: 16px; max-width: 900px; width: 90% }
.modal-loading { padding: 16px; text-align: center; color: #64748b }
</style>

