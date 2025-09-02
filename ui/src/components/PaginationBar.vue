<template>
  <div class="pager" v-if="totalPages > 1">
    <button :disabled="!hasPrev" @click="$emit('update:page', page-1)">Prev</button>
    <span>Page {{ page }} / {{ totalPages }}</span>
    <button :disabled="!hasNext" @click="$emit('update:page', page+1)">Next</button>
  </div>
</template>

<script setup lang="ts">
defineProps<{ page: number; totalPages: number; hasNext: boolean; hasPrev: boolean }>()
defineEmits<{ 'update:page':[number] }>()
</script>

<style scoped>
.pager { display: flex; gap: 8px; align-items: center; padding: 8px 0; }
button[disabled] { opacity: .5; }
</style>

