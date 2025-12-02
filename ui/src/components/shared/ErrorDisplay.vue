<template>
  <div v-if="error" class="error-display" :class="severity">
    <div class="error-header">
      <span class="error-icon">{{ icon }}</span>
      <div class="error-title-section">
        <span class="error-title">{{ title }}</span>
        <span class="error-code" v-if="error.code">[{{ error.code }}]</span>
      </div>
    </div>

    <div class="error-content">
      <p class="error-message">{{ error.message }}</p>

      <p class="error-details" v-if="error.details">{{ error.details }}</p>

      <div class="error-context" v-if="error.context">
        <strong>While:</strong> {{ error.context.action }}
        <span v-if="error.context.resource">
          ({{ error.context.resource }}
          <span v-if="error.context.resourceId">#{{ error.context.resourceId }}</span>)
        </span>
      </div>

      <ul class="error-suggestions" v-if="error.suggestions?.length">
        <li class="suggestion-title">Suggested actions:</li>
        <li v-for="(suggestion, index) in error.suggestions" :key="index" class="suggestion-item">
          {{ suggestion }}
        </li>
      </ul>
    </div>

    <div class="error-actions">
      <button
        v-if="error.retryable"
        @click="emit('retry')"
        class="retry-button"
      >
        🔄 Retry
      </button>
      <button
        @click="emit('dismiss')"
        class="dismiss-button"
      >
        ✕ Dismiss
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AppError } from '@/types/errors'

const props = withDefaults(defineProps<{
  error: AppError | null
  severity?: 'error' | 'warning' | 'info'
}>(), {
  severity: 'error'
})

const emit = defineEmits<{
  retry: []
  dismiss: []
}>()

const icon = computed(() => {
  switch (props.severity) {
    case 'error':
      return '❌'
    case 'warning':
      return '⚠️'
    case 'info':
      return 'ℹ️'
    default:
      return '❌'
  }
})

const title = computed(() => {
  switch (props.severity) {
    case 'error':
      return 'Error'
    case 'warning':
      return 'Warning'
    case 'info':
      return 'Notice'
    default:
      return 'Error'
  }
})
</script>

<style scoped>
.error-display {
  border-radius: 6px;
  padding: 16px;
  margin: 16px 0;
  border-width: 1px;
  border-style: solid;
}

.error-display.error {
  background-color: #fef2f2;
  border-color: #fca5a5;
  color: #991b1b;
}

.error-display.warning {
  background-color: #fffbeb;
  border-color: #fcd34d;
  color: #92400e;
}

.error-display.info {
  background-color: #eff6ff;
  border-color: #93c5fd;
  color: #1e40af;
}

.error-header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}

.error-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.error-title-section {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.error-title {
  font-weight: 600;
  font-size: 16px;
}

.error-code {
  font-family: monospace;
  font-size: 12px;
  opacity: 0.8;
  background-color: rgba(0, 0, 0, 0.05);
  padding: 2px 6px;
  border-radius: 3px;
}

.error-content {
  margin-bottom: 16px;
}

.error-message {
  font-size: 14px;
  margin: 0 0 8px 0;
  font-weight: 500;
}

.error-details {
  font-size: 13px;
  margin: 8px 0;
  opacity: 0.9;
  font-style: italic;
}

.error-context {
  font-size: 13px;
  margin: 8px 0;
  padding: 8px;
  background-color: rgba(0, 0, 0, 0.03);
  border-radius: 4px;
}

.error-suggestions {
  margin: 12px 0 0 0;
  padding: 0;
  list-style: none;
}

.suggestion-title {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 6px;
}

.suggestion-item {
  font-size: 13px;
  margin: 4px 0;
  padding-left: 20px;
  position: relative;
}

.suggestion-item::before {
  content: '→';
  position: absolute;
  left: 0;
  opacity: 0.6;
}

.error-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.retry-button,
.dismiss-button {
  padding: 6px 12px;
  border: none;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.2s;
}

.retry-button {
  background-color: #3b82f6;
  color: white;
}

.retry-button:hover {
  background-color: #2563eb;
}

.dismiss-button {
  background-color: transparent;
  color: inherit;
  border: 1px solid currentColor;
  opacity: 0.7;
}

.dismiss-button:hover {
  opacity: 1;
}

@media (max-width: 768px) {
  .error-actions {
    flex-direction: column-reverse;
  }

  .retry-button,
  .dismiss-button {
    width: 100%;
  }
}
</style>
