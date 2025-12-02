<template>
  <div class="modal-overlay" @click="$emit('cancel')">
    <div class="modal-content confirm-modal" @click.stop>
      <h3>{{ title }}</h3>
      <p class="message" v-html="message"></p>
      <div class="modal-actions">
        <button class="secondary-button" @click="$emit('cancel')">{{ cancelText || 'Cancel' }}</button>
        <button class="danger-button" @click="$emit('confirm')">{{ confirmText || 'Confirm' }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; message: string; confirmText?: string; cancelText?: string }>()
defineEmits<{ confirm: []; cancel: [] }>()
</script>

<style scoped>
.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, .45); display: flex; align-items: center; justify-content: center; z-index: 1000 }
.modal-content { background: #fff; border-radius: 8px; box-shadow: 0 25px 50px -12px rgba(0,0,0,.25); padding: 16px; max-width: 560px; width: 90% }
.message { color: #334155 }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px }
.secondary-button { background: #f8fafc; color: #334155; border: 1px solid #e2e8f0; border-radius: 6px; padding: 6px 12px }
.danger-button { background: #ef4444; color: #fff; border: 1px solid #ef4444; border-radius: 6px; padding: 6px 12px }
</style>

