<template>
  <div class="error-state" data-testid="error-state">
    <span class="error-icon">⚠️</span>
    <h3 class="error-title">{{ title }}</h3>
    <p class="error-message">{{ message }}</p>
    <button
      v-if="retryable"
      @click="$emit('retry')"
      class="retry-btn"
      data-testid="retry-btn"
    >
      Retry
    </button>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  title?: string
  message: string
  retryable?: boolean
}>()

defineEmits<{
  retry: []
}>()
</script>

<style scoped>
.error-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 3rem;
  text-align: center;
  min-height: 300px;
}

.error-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.error-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #ef4444;
  margin: 0 0 0.5rem 0;
}

.error-message {
  color: #6b7280;
  margin: 0 0 1.5rem 0;
  max-width: 500px;
}

.retry-btn {
  padding: 0.5rem 1.5rem;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: background-color 0.2s;
}

.retry-btn:hover {
  background: #2563eb;
}

.retry-btn:active {
  background: #1d4ed8;
}
</style>
