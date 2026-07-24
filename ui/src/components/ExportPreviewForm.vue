<template>
  <section class="preview-panel" data-testid="export-preview-form">
    <h2>Preview Export</h2>
    <form @submit.prevent="submit">
      <div class="form-field">
        <label for="export-plugin">Export plugin</label>
        <select id="export-plugin" v-model="pluginName" data-testid="plugin-select" :disabled="Boolean(listError)" @change="changePlugin">
          <option value="">Select a plugin…</option>
          <option v-for="plugin in plugins" :key="plugin.name" :value="plugin.name">
            {{ plugin.display_name }}
          </option>
        </select>
      </div>

      <div v-if="pluginName" class="form-field">
        <label for="export-format">Output format</label>
        <select id="export-format" v-model="format" :disabled="schemaLoading" @change="invalidateResult">
          <option value="">Select a format…</option>
          <option v-for="item in formats" :key="item" :value="item">{{ item.toUpperCase() }}</option>
        </select>
      </div>

      <div v-if="schemaLoading" role="status">Loading plugin schema…</div>
      <SchemaForm
        v-else-if="schema"
        :schema="schema"
        :values="values"
        :touched="touched"
        :show-all-errors="showAllErrors"
        @update:values="updateValues"
        @update:touched="updateTouched"
      />

      <p v-if="listError" class="error" role="alert">{{ listError }}</p>
      <p v-if="schemaError" class="error" role="alert">{{ schemaError }}</p>
      <p v-if="requestError" class="error" role="alert">{{ requestError }}</p>
      <button data-testid="preview-button" :disabled="!canPreview || loading">
        {{ loading ? 'Previewing…' : 'Preview Export' }}
      </button>
    </form>

    <div v-if="result" class="result" data-testid="preview-section">
      <h3>Preview result</h3>
      <dl>
        <div><dt>Status</dt><dd>{{ result.preview.success ? 'Ready' : 'Issues found' }}</dd></div>
        <div><dt>Records</dt><dd>{{ result.preview.record_count.toLocaleString() }}</dd></div>
        <div><dt>Estimated size</dt><dd>{{ formatBytes(result.preview.estimated_size) }}</dd></div>
      </dl>
      <ul v-if="result.preview.warnings?.length">
        <li v-for="warning in result.preview.warnings" :key="warning">{{ warning }}</li>
      </ul>
      <details>
        <summary>Raw response</summary>
        <pre>{{ JSON.stringify(result, null, 2) }}</pre>
      </details>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { previewExport, type ExportPreviewResponse, type ExportRequest } from '@/api/export'
import { getPluginSchema, listPlugins, type Plugin, type PluginSchema } from '@/api/plugin'
import SchemaForm from '@/components/shared/SchemaForm.vue'
import {
  initialPluginConfig,
  pluginConfigFromForm,
  pluginFormErrors,
  type PluginFormTouched,
  type PluginFormValues,
} from '@/utils/plugin-schema'

const plugins = ref<Plugin[]>([])
const pluginName = ref('')
const format = ref('')
const schema = ref<PluginSchema | null>(null)
const values = ref<PluginFormValues>({})
const touched = ref<PluginFormTouched>({})
const result = ref<ExportPreviewResponse | null>(null)
const listError = ref('')
const schemaError = ref('')
const requestError = ref('')
const schemaLoading = ref(false)
const loading = ref(false)
const showAllErrors = ref(false)
let token = 0

const selectedPlugin = computed(() => plugins.value.find(plugin => plugin.name === pluginName.value))
const formats = computed(() => selectedPlugin.value?.capabilities ?? [])
const canPreview = computed(() => {
  if (!pluginName.value || !format.value || !schema.value || listError.value || schemaError.value) return false
  return Object.keys(pluginFormErrors(schema.value, values.value, touched.value, true)).length === 0
})

onMounted(async () => {
  const current = ++token
  try {
    const registered = (await listPlugins()).plugins
    if (current === token) plugins.value = registered
  } catch (error) {
    if (current === token) listError.value = message(error, 'Failed to load plugins')
  }
})

onBeforeUnmount(() => { token++ })

async function changePlugin() {
  const current = ++token
  format.value = ''
  schema.value = null
  values.value = {}
  touched.value = {}
  result.value = null
  requestError.value = ''
  schemaError.value = ''
  showAllErrors.value = false
  loading.value = false
  schemaLoading.value = false
  if (!pluginName.value) return
  schemaLoading.value = true
  try {
    const loaded = await getPluginSchema(pluginName.value)
    if (current !== token) return
    schema.value = { ...loaded, required: loaded.required ?? [] }
    values.value = initialPluginConfig(schema.value)
  } catch (error) {
    if (current === token) schemaError.value = message(error, 'Failed to load plugin schema')
  } finally {
    if (current === token) schemaLoading.value = false
  }
}

function invalidateResult() {
  token++
  result.value = null
  requestError.value = ''
  loading.value = false
}

function updateValues(next: PluginFormValues) {
  values.value = next
  invalidateResult()
}

function updateTouched(next: PluginFormTouched) {
  touched.value = next
  invalidateResult()
}

async function submit() {
  if (!schema.value) return
  showAllErrors.value = true
  let config: Record<string, unknown>
  try {
    config = pluginConfigFromForm(schema.value, values.value, touched.value)
  } catch {
    return
  }
  const request: ExportRequest = {
    plugin_name: pluginName.value,
    format: format.value,
    config,
    filters: {},
    output: { type: 'response' },
    options: {
      dry_run: true,
      include_history: false,
      validate_only: true,
      compact_output: false,
      include_metadata: true,
    },
  }
  const current = ++token
  loading.value = true
  requestError.value = ''
  result.value = null
  try {
    const response = await previewExport(request)
    if (current === token) result.value = response
  } catch (error) {
    if (current === token) requestError.value = message(error, 'Export preview failed')
  } finally {
    if (current === token) loading.value = false
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB']
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** unit).toFixed(unit ? 1 : 0)} ${units[unit]}`
}

function message(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}
</script>

<style scoped>
.preview-panel { margin: 20px 0; padding: 18px; border: 1px solid #d1d5db; border-radius: 8px; }
form { display: flex; flex-direction: column; gap: 16px; }
.form-field { display: flex; flex-direction: column; gap: 6px; }
select, button { padding: 9px 11px; font: inherit; }
button { align-self: start; }
.error { color: #b91c1c; }
dl { display: flex; gap: 24px; }
dt { color: #6b7280; font-size: .8rem; }
dd { margin: 2px 0 0; font-weight: 600; }
pre { overflow: auto; }
</style>
