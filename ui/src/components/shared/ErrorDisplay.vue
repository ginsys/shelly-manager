<template>
  <div class="error-display" :class="severityClass" role="alert" data-testid="error-display">
    <div class="error-header">
      <span class="error-icon" aria-hidden="true">{{ icon }}</span>
      <span class="error-title">{{ title }}</span>
      <span v-if="error?.code" class="error-code">[{{ error.code }}]</span>
    </div>
    <p class="error-message">{{ error?.message }}</p>
    <p v-if="error?.details" class="error-details">{{ error.details }}</p>
    <ul v-if="error?.suggestions?.length" class="error-suggestions">
      <li v-for="s in error.suggestions" :key="s">{{ s }}</li>
    </ul>
    <div class="error-actions">
      <button v-if="error?.retryable" class="btn primary" @click="$emit('retry')">Retry</button>
      <button class="btn" @click="$emit('dismiss')">Dismiss</button>
    </div>
  </div>
  
</template>

<script setup lang="ts">
import type { AppError } from '@/types/errors'

const props = defineProps<{
  error: AppError
  title?: string
}>()

defineEmits<{ retry: []; dismiss: [] }>()

const severityClass = computed(() => props.error?.severity || 'error')
const icon = computed(() => props.error?.severity === 'warning' ? '⚠️' : props.error?.severity === 'info' ? 'ℹ️' : '❌')
const title = computed(() => props.title || 'Operation Failed')
</script>

<style scoped>
.error-display { border: 1px solid #fecaca; background: #fef2f2; color: #991b1b; border-radius: 8px; padding: 12px; }
.error-display.warning { border-color: #fcd34d; background: #fffbeb; color: #92400e; }
.error-display.info { border-color: #93c5fd; background: #eff6ff; color: #1d4ed8; }
.error-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; font-weight: 600; }
.error-icon { font-size: 1.1rem }
.error-code { color: #6b7280; font-size: .8rem }
.error-message { margin: 0 0 4px 0 }
.error-details { margin: 0 0 8px 0; color: #6b7280 }
.error-suggestions { margin: 0 0 8px 16px; }
.error-actions { display: flex; gap: 8px }
.btn { background: #f3f4f6; border: 1px solid #d1d5db; padding: 6px 10px; border-radius: 6px; cursor: pointer }
.btn.primary { background: #3b82f6; border-color: #3b82f6; color: #fff }
.btn:disabled { opacity: .6; cursor: not-allowed }
</style>

