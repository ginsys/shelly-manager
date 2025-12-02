<template>
  <div class="filters-section">
    <div class="filter-row">
      <div class="filter-group">
        <label class="filter-label">Format:</label>
        <select :value="format" @change="onFormat" class="filter-select">
          <option value="">All formats</option>
          <option value="terraform">Terraform</option>
          <option value="ansible">Ansible</option>
          <option value="kubernetes">Kubernetes</option>
          <option value="docker-compose">Docker Compose</option>
          <option value="yaml">YAML</option>
        </select>
      </div>
      <div class="filter-group">
        <label class="filter-label">Status:</label>
        <select :value="successValue" @change="onSuccess" class="filter-select">
          <option value="all">All statuses</option>
          <option value="true">Success only</option>
          <option value="false">Failed only</option>
        </select>
      </div>
      <div class="filter-actions">
        <button @click="$emit('refresh')" class="refresh-button" :disabled="loading">🔄 Refresh</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{ format: string; success?: boolean; loading: boolean }>()
const emit = defineEmits<{ 'update:format': [string]; 'update:success': [boolean | undefined]; refresh: [] }>()

function onFormat(e: Event) { emit('update:format', (e.target as HTMLSelectElement).value) }
function onSuccess(e: Event) {
  const v = (e.target as HTMLSelectElement).value
  emit('update:success', v === 'all' ? undefined : v === 'true')
}

const successValue = computed(() => {
  if (props.success === undefined) return 'all'
  return props.success ? 'true' : 'false'
})
</script>

<style scoped>
.filters-section { margin: 16px 0 }
.filter-row { display: flex; gap: 12px; flex-wrap: wrap; align-items: flex-end }
.filter-group { display: grid; gap: 6px }
.filter-label { font-size: .875rem; color: #4b5563 }
.filter-select { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px }
.filter-actions .refresh-button { background: #10b981; color: white; border: none; padding: 8px 16px; border-radius: 6px; font-size: .875rem; cursor: pointer }
.filter-actions .refresh-button:disabled { opacity: .6; cursor: not-allowed }
</style>

