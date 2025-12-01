<template>
  <div class="column-toggle">
    <button class="btn" @click="open = !open" :aria-expanded="open">Columns</button>
    <div v-if="open" class="menu" @click.outside="open=false">
      <label v-for="col in columns" :key="col.key" class="item">
        <input type="checkbox" :checked="model[col.key]" @change="onToggle(col.key, $event)" />
        <span>{{ col.label }}</span>
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'

type Column = { key: string; label: string }

const props = defineProps<{
  columns: Column[]
  modelValue: Record<string, boolean>
}>()

const emit = defineEmits<{ 'update:modelValue': [Record<string, boolean>] }>()

const open = ref(false)
const model = reactive<Record<string, boolean>>({ ...props.modelValue })

watch(() => props.modelValue, (v) => {
  Object.assign(model, v)
})

function onToggle(key: string, e: Event) {
  const target = e.target as HTMLInputElement
  const next = { ...model, [key]: !!target.checked }
  emit('update:modelValue', next)
}
</script>

<style scoped>
.column-toggle { position: relative }
.btn { padding: 6px 10px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer }
.menu { position: absolute; right: 0; margin-top: 6px; background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; box-shadow: 0 10px 15px -3px rgba(0,0,0,.1); padding: 8px; min-width: 180px; z-index: 10 }
.item { display: flex; gap: 8px; align-items: center; padding: 6px 4px; font-size: 14px }
.item + .item { border-top: 1px solid #f1f5f9 }
</style>

