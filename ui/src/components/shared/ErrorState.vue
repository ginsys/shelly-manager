<template>
  <div class="error-state" role="alert">
    <span class="error-icon" aria-hidden="true">!</span>
    <div class="content">
      <h3 class="title">{{ title }}</h3>
      <p class="message">{{ message }}</p>
    </div>
    <button v-if="retryable" class="retry" @click="$emit('retry')">Retry</button>
  </div>
  
</template>

<script setup lang="ts">
defineProps<{
  title: string
  message: string
  retryable?: boolean
}>()

defineEmits<{ retry: [] }>()
</script>

<style scoped>
.error-state { display:flex; align-items:center; gap:12px; padding:12px; border:1px solid #fecaca; background:#fee2e2; border-radius:8px; color:#7f1d1d }
.error-icon { display:inline-flex; align-items:center; justify-content:center; width:24px; height:24px; border-radius:999px; background:#ef4444; color:#fff; font-weight:700 }
.content { flex:1 }
.title { margin:0; font-size:16px }
.message { margin:4px 0 0 0; font-size:14px }
.retry { padding:6px 10px; border:1px solid #b91c1c; background:#fff; color:#7f1d1d; border-radius:6px; cursor:pointer }
.retry:hover { background:#fff7f7 }
</style>

