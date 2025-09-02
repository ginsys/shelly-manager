<template>
  <form class="preview-form" @submit.prevent="onPreview">
    <label>Plugin <input v-model="plugin" placeholder="mockfile" /></label>
    <label>Format <input v-model="format" placeholder="txt" /></label>
    <button :disabled="!plugin || !format">Preview</button>
    <span v-if="msg" class="msg">{{ msg }}</span>
  </form>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { previewExport } from '@/api/export'
const plugin = ref('')
const format = ref('')
const msg = ref('')
async function onPreview(){
  msg.value = 'Running…'
  try {
    const res = await previewExport({ plugin_name: plugin.value, format: format.value })
    msg.value = `Records: ${res.summary?.record_count ?? res.preview?.record_count ?? 'n/a'}`
  } catch(e:any){ msg.value = e?.message || 'Failed' }
}
</script>
<style scoped>
.preview-form { display:flex; gap:8px; align-items:center; }
.msg { margin-left: 8px; color:#334155 }
</style>
