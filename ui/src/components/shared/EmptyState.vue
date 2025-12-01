<template>
  <div class="empty-state">
    <span class="empty-icon" aria-hidden="true">…</span>
    <div class="content">
      <h3 class="title">{{ title }}</h3>
      <p class="message">{{ message }}</p>
      <div class="action"><slot name="action" /></div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  message: string
}>()
</script>

<style scoped>
.empty-state { display:flex; align-items:flex-start; gap:12px; padding:12px; border:1px solid #e5e7eb; background:#f8fafc; border-radius:8px; color:#334155 }
.empty-icon { display:inline-flex; align-items:center; justify-content:center; width:24px; height:24px; border-radius:999px; background:#e2e8f0; color:#475569; font-weight:700 }
.content { flex:1 }
.title { margin:0; font-size:16px }
.message { margin:4px 0 0 0; font-size:14px }
.action { margin-top:8px }
</style>

