<template>
  <div v-if="text" :class="['message', type]">
    <slot>{{ text }}</slot>
    <button class="message-close" @click="$emit('close')">✖</button>
  </div>
</template>

<script setup lang="ts">
defineProps<{ text: string; type: 'success' | 'error' }>()
defineEmits<{ close: [] }>()
</script>

<style scoped>
.message { display:flex; align-items:center; justify-content: space-between; gap: 10px; padding: 10px 12px; border:1px solid #e5e7eb; border-radius: 8px; margin-top: 12px }
.message.success { background:#ecfdf5; border-color:#a7f3d0; color:#065f46 }
.message.error { background:#fef2f2; border-color:#fecaca; color:#991b1b }
.message-close { background:transparent; border:0; cursor:pointer; color:inherit }
</style>

