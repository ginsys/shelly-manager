<template>
  <div class="filters">
    <label>
      Plugin:
      <input v-model="localPlugin" placeholder="case-sensitive" @input="onPlugin" />
    </label>
    <label>
      Success:
      <select v-model="localSuccess" @change="onSuccess">
        <option :value="''">Any</option>
        <option :value="'true'">Success</option>
        <option :value="'false'">Failure</option>
      </select>
    </label>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
const props = defineProps<{ plugin?: string; success?: boolean | undefined }>()
const emit = defineEmits<{ 'update:plugin':[string], 'update:success':[boolean|undefined] }>()
const localPlugin = ref(props.plugin || '')
const localSuccess = ref(props.success === undefined ? '' : String(props.success))
function onPlugin() { emit('update:plugin', localPlugin.value) }
function onSuccess() {
  if (localSuccess.value === '') emit('update:success', undefined)
  else emit('update:success', localSuccess.value === 'true')
}
watch(() => props.plugin, v => localPlugin.value = v || '')
watch(() => props.success, v => localSuccess.value = (v===undefined)?'':String(v))
</script>

<style scoped>
.filters { display:flex; gap: 16px; padding: 8px 0; }
label { display:flex; gap: 8px; align-items:center; }
input, select { padding: 4px 6px; }
</style>

