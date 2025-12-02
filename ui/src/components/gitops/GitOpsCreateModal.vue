<template>
  <div class="modal-overlay" @click="$emit('close')">
    <div class="modal-content create-modal" @click.stop>
      <GitOpsConfigForm
        :available-devices="availableDevices"
        :loading="loading"
        :error="error"
        @submit="$emit('submit', $event)"
        @preview="$emit('preview', $event)"
        @cancel="$emit('close')"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import GitOpsConfigForm from '@/components/GitOpsConfigForm.vue'
import type { Device } from '@/api/types'

defineProps<{ availableDevices: Device[]; loading: boolean; error: string }>()
defineEmits<{ close: []; submit: [any]; preview: [any] }>()
</script>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, .45); display: flex; align-items: center; justify-content: center; z-index: 1000 }
.modal-content { background: #fff; border-radius: 8px; box-shadow: 0 25px 50px -12px rgba(0,0,0,.25); padding: 16px; max-width: 900px; width: 90% }
</style>

