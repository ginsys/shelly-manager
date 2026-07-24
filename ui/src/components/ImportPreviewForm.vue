<template>
  <section class="preview-panel" data-testid="import-preview-form">
    <h2>Preview SMA Import</h2>
    <form @submit.prevent="submit">
      <div class="form-field">
        <label for="import-plugin">Import plugin</label>
        <select id="import-plugin" v-model="pluginName" data-testid="plugin-select" :disabled="Boolean(listError)" @change="invalidate">
          <option value="">Select a plugin…</option>
          <option v-for="plugin in plugins" :key="plugin.name" :value="plugin.name">
            {{ plugin.display_name }}
          </option>
        </select>
      </div>

      <template v-if="pluginName">
        <div class="form-field">
          <label for="import-file">SMA or JSON file</label>
          <input
            id="import-file"
            ref="fileInput"
            type="file"
            accept=".sma,.json,application/json,application/gzip"
            @change="selectFile"
          />
          <span v-if="file">{{ file.name }} ({{ formatBytes(file.size) }})</span>
        </div>
        <div class="separator">or</div>
        <div class="form-field">
          <label for="import-text">Raw SMA JSON</label>
          <textarea
            id="import-text"
            v-model="text"
            rows="7"
            placeholder="Paste a 2026.1 SMA JSON representation"
            @input="changeText"
          />
        </div>
      </template>

      <p
        v-if="registryLoaded && !listError && plugins.length === 0"
        class="empty-state"
        data-testid="no-compatible-importer"
      >
        No compatible browser import plugin is available.
      </p>
      <p v-if="listError" class="error" role="alert">{{ listError }}</p>
      <p v-if="requestError" class="error" role="alert">{{ requestError }}</p>
      <button data-testid="preview-button" :disabled="!pluginName || (!file && !text) || loading">
        {{ loading ? 'Analyzing…' : 'Preview Import' }}
      </button>
    </form>

    <div v-if="result" class="result" data-testid="preview-section">
      <h3>Import preview</h3>
      <dl>
        <div><dt>Will create</dt><dd>{{ result.summary.will_create }}</dd></div>
        <div><dt>Will update</dt><dd>{{ result.summary.will_update }}</dd></div>
        <div><dt>Will delete</dt><dd>{{ result.summary.will_delete }}</dd></div>
        <div><dt>Changes</dt><dd>{{ result.changes_count }}</dd></div>
        <div><dt>Imported</dt><dd>{{ result.preview.records_imported ?? 0 }}</dd></div>
        <div><dt>Skipped</dt><dd>{{ result.preview.records_skipped ?? 0 }}</dd></div>
      </dl>
      <ul v-if="result.preview.warnings?.length">
        <li v-for="warning in result.preview.warnings" :key="warning">{{ warning }}</li>
      </ul>
      <div v-if="result.preview.errors?.length">
        <h4>Plugin errors</h4>
        <ul class="error" role="alert">
          <li v-for="error in result.preview.errors" :key="error">{{ error }}</li>
        </ul>
      </div>
      <div v-if="result.preview.changes?.length">
        <h4>Proposed changes</h4>
        <ul>
          <li v-for="(change, index) in result.preview.changes" :key="`${change.resource_id}-${index}`">
            {{ change.type }} {{ change.resource }} {{ change.resource_id }}
          </li>
        </ul>
      </div>
      <details>
        <summary>Raw response</summary>
        <pre>{{ JSON.stringify(result, null, 2) }}</pre>
      </details>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import {
  BROWSER_DATA_IMPORT_PLUGINS,
  previewImport,
  type ImportPreviewResponse,
  type ImportRequest,
} from '@/api/import'
import { listPlugins, type Plugin } from '@/api/plugin'
import { bytesToBase64 } from '@/utils/base64'

const maxBrowserBytes = 7 * 1024 * 1024
const plugins = ref<Plugin[]>([])
const pluginName = ref('')
const file = ref<File | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const text = ref('')
const listError = ref('')
const requestError = ref('')
const registryLoaded = ref(false)
const loading = ref(false)
const result = ref<ImportPreviewResponse | null>(null)
let token = 0

onMounted(async () => {
  const current = ++token
  try {
    const registered = (await listPlugins()).plugins
    if (current !== token) return
    plugins.value = registered.filter(plugin =>
      BROWSER_DATA_IMPORT_PLUGINS.includes(plugin.name as 'sma')
      && plugin.capabilities.includes('sma'),
    )
  } catch (error) {
    if (current === token) listError.value = message(error, 'Failed to load plugins')
  } finally {
    if (current === token) registryLoaded.value = true
  }
})

onBeforeUnmount(() => { token++ })

function invalidate() {
  token++
  result.value = null
  requestError.value = ''
  loading.value = false
}

function selectFile(event: Event) {
  file.value = (event.target as HTMLInputElement).files?.[0] ?? null
  if (file.value) text.value = ''
  invalidate()
}

function changeText() {
  if (text.value && file.value) {
    file.value = null
    if (fileInput.value) fileInput.value.value = ''
  }
  invalidate()
}

async function submit() {
  const current = ++token
  loading.value = true
  requestError.value = ''
  result.value = null
  try {
    if (file.value && file.value.size > maxBrowserBytes) {
      throw new Error('Import source exceeds the 7 MiB browser limit')
    }
    const bytes = file.value
      ? new Uint8Array(await file.value.arrayBuffer())
      : new TextEncoder().encode(text.value)
    if (bytes.byteLength > maxBrowserBytes) {
      throw new Error('Import source exceeds the 7 MiB browser limit')
    }
    if (current !== token) return
    const request: ImportRequest = {
      plugin_name: 'sma',
      format: 'sma',
      source: { type: 'data', data: bytesToBase64(bytes) },
      config: {},
      options: {
        dry_run: true,
        validate_only: true,
      },
    }
    const response = await previewImport(request)
    if (current === token) result.value = response
  } catch (error) {
    if (current === token) requestError.value = message(error, 'Import preview failed')
  } finally {
    if (current === token) loading.value = false
  }
}

function formatBytes(bytes: number): string {
  return bytes < 1024 ? `${bytes} B` : `${(bytes / 1024 / 1024).toFixed(2)} MiB`
}

function message(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}
</script>

<style scoped>
.preview-panel { margin: 20px 0; padding: 18px; border: 1px solid #d1d5db; border-radius: 8px; }
form { display: flex; flex-direction: column; gap: 14px; }
.form-field { display: flex; flex-direction: column; gap: 6px; }
select, textarea, button { padding: 9px 11px; font: inherit; }
button { align-self: start; }
.separator { color: #6b7280; }
.error { color: #b91c1c; }
.empty-state { color: #4b5563; }
dl { display: flex; flex-wrap: wrap; gap: 22px; }
dt { color: #6b7280; font-size: .8rem; }
dd { margin: 2px 0 0; font-weight: 600; }
pre { overflow: auto; }
</style>
