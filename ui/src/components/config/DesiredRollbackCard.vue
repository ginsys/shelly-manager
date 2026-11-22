<template>
  <div class="card" data-testid="versions-card">
    <div class="header-row"><h2>Desired configuration versions</h2></div>
    <p class="hint">What Apply sends to the device. The History card below lists configurations imported from it.</p>
    <div v-if="notice" class="notice" :class="notice.kind" role="status" data-testid="rollback-notice">
      <span>{{ notice.text }}</span>
      <button class="btn" data-testid="rollback-notice-dismiss" @click="notice = null">Dismiss</button>
    </div>
    <div v-if="listError" class="live-error" data-testid="versions-error">{{ listError }}</div>
    <div v-if="versions.length > 0" class="list">
      <div v-for="v in versions" :key="v.id" class="item" data-testid="version-entry">
        <span class="strong">Version {{ v.id }}</span>
        <span>{{ formatDate(v.created_at) }}</span>
        <span class="pill">{{ v.action }}</span>
        <span class="hint">{{ v.changed_by || '-' }}</span>
        <span v-if="v.restored_from" class="hint">restored from version {{ v.restored_from }}</span>
        <div class="spacer" />
        <span v-if="v.new_desired === null" class="hint" data-testid="version-no-text">No stored text: cannot be rolled back to</span>
        <button
          v-else
          class="btn"
          :data-testid="`rollback-${v.id}`"
          :disabled="editing || !ready"
          :title="editing ? 'Save or cancel the overrides edit first' : !ready ? 'Wait until the configuration has been read' : ''"
          @click="openDialog(v)"
        >
          Roll back
        </button>
      </div>
    </div>
    <div v-else-if="loaded && !listError" class="empty-state" data-testid="versions-empty">No versions yet.</div>

    <div v-if="target" class="modal-overlay" data-testid="rollback-dialog">
      <div class="modal" role="dialog" aria-modal="true" aria-labelledby="rollback-title">
        <h2 id="rollback-title">Roll back to version {{ target.id }}</h2>
        <p>
          This replaces the desired configuration with version {{ target.id }}. The device is unchanged until you apply
          it.
        </p>
        <p v-if="!diff" class="hint" data-testid="rollback-no-diff">
          The text of this version could not be read as settings, so no difference is shown.
        </p>
        <p v-else-if="diff.length === 0" class="hint" data-testid="rollback-no-diff">
          No difference from the current desired configuration.
        </p>
        <ul v-else class="diff" data-testid="rollback-diff">
          <li v-for="line in diff" :key="line.path" data-testid="rollback-diff-line">
            <code>{{ line.path }}</code>: {{ line.kind }}
            <template v-if="line.kind !== 'added'"> from <code>{{ JSON.stringify(line.from) }}</code></template>
            <template v-if="line.kind !== 'removed'"> to <code>{{ JSON.stringify(line.to) }}</code></template>
          </li>
        </ul>
        <p class="hint" data-testid="rollback-secrets-note">
          Secrets are not compared: the current configuration holds none, so a secret this rollback adds, removes or
          changes shows no line.
        </p>
        <p data-testid="rollback-detach">
          <template v-if="templates.length > 0">These templates are detached: {{ templates.map((t) => t.name).join(', ') }}.</template>
          <template v-else>No template is attached.</template>
        </p>
        <p data-testid="rollback-way-back">
          Way back, in two steps: re-attach the templates listed above, then clear the device overrides. Each step
          recomputes the desired configuration; neither contacts the device.
        </p>

        <p v-if="refusal" class="error" role="alert" data-testid="rollback-refusal">
          {{ refusal.message }}
          <template v-if="refusal.paths.length > 0 && prompt.length === 0">
            <code v-for="p in refusal.paths" :key="p" class="path">{{ p }}</code>
          </template>
        </p>
        <div v-if="prompt.length > 0" data-testid="rollback-secrets">
          <div v-for="p in prompt" :key="p" class="field">
            <label :for="inputId(p)">{{ p }}</label>
            <textarea
              v-if="isUrlList(p)"
              :id="inputId(p)"
              v-model="entered[p]"
              rows="3"
              autocomplete="off"
              placeholder="One URL per line"
              :data-testid="`secret-${p}`"
            />
            <input
              v-else
              :id="inputId(p)"
              v-model="entered[p]"
              type="password"
              autocomplete="new-password"
              :data-testid="`secret-${p}`"
            />
          </div>
        </div>

        <div class="modal-actions">
          <button class="btn" data-testid="rollback-cancel" :disabled="busy" @click="closeDialog">Cancel</button>
          <button class="btn primary" data-testid="rollback-confirm" :disabled="busy || editing || !ready" @click="confirm">
            {{ prompt.length > 0 ? 'Retry with these secrets' : 'Roll back' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useLateResult } from '@/composables/useNotice'
import { getDesiredHistory, rollbackDesired, RollbackRefused, type DesiredVersion, type RollbackSecrets } from '@/api/desiredRollback'
import { diffDesired, parseVersion } from '@/utils/desiredDiff'
import type { ConfigTemplate } from '@/api/templates'

const props = defineProps<{
  deviceId: number | string
  current: Record<string, any> | null
  templates: ConfigTemplate[]
  editing: boolean
  // The page has read the current desired configuration and templates. Until then the difference and the
  // detach list would compare with nothing, and a rollback must not be confirmed on that.
  ready: boolean
}>()
const emit = defineEmits<{ refresh: [] }>()

const versions = ref<DesiredVersion[]>([])
const loaded = ref(false)
const listError = ref('')
const notice = ref<{ kind: 'success' | 'error'; text: string } | null>(null)
const target = ref<DesiredVersion | null>(null)
const busy = ref(false)
const refusal = ref<{ message: string; paths: string[] } | null>(null)
// The paths a 409 asked for, and what the operator typed for them. The typed text lives only here: it is
// cleared after every answer, on cancel and when the card goes (a device change re-creates the card).
const prompt = ref<string[]>([])
const entered = ref<Record<string, string>>({})

const reportLateResult = useLateResult()
// Counts the reads of the list: only the newest fills it. `gone` is true once the card was removed.
let readSeq = 0
let gone = false
onUnmounted(() => {
  gone = true
  entered.value = {}
})

const diff = computed(() => {
  const parsed = target.value ? parseVersion(target.value.new_desired) : null
  return parsed ? diffDesired(parsed, props.current ?? {}) : null
})

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

// An action URL list is entered as a list, one URL per line. The grammar of the key is all the UI knows of
// the paths: the server decides which secrets a version needs.
const isUrlList = (path: string) => /^actions\..*\.secret_urls(#\d+)?$/.test(path)
const inputId = (path: string) => `secret-${path.replace(/[^A-Za-z0-9_-]/g, '_')}`

async function loadVersions() {
  const seq = ++readSeq
  try {
    const rows = await getDesiredHistory(props.deviceId)
    if (gone || seq !== readSeq) return
    versions.value = rows
    listError.value = ''
    loaded.value = true
  } catch (e: any) {
    if (gone || seq !== readSeq) return
    // A list already shown stays: a read that fails is not a reason to take versions away.
    listError.value = e?.response?.data?.error?.message || e?.message || 'Failed to read the desired configuration versions'
  }
}
onMounted(loadVersions)
// The one cue to read the versions again: the page finishing a read. A rollback asks the page to read (the refresh
// event) and so reads the versions through this; a save or a template change on the page records versions too, and
// a failed first read can only be retried by the page's Refresh.
watch(() => props.ready, (now) => {
  if (now) void loadVersions()
})

function openDialog(version: DesiredVersion) {
  target.value = version
  refusal.value = null
  prompt.value = []
  entered.value = {}
}

function closeDialog() {
  target.value = null
  refusal.value = null
  prompt.value = []
  entered.value = {}
}

// The secrets the retry sends, or the paths still empty. A text with no other line is an empty list.
function collect(): { secrets?: RollbackSecrets; missing: string[] } {
  if (prompt.value.length === 0) return { missing: [] }
  const secrets: RollbackSecrets = {}
  const missing: string[] = []
  for (const path of prompt.value) {
    const raw = entered.value[path] ?? ''
    const value = isUrlList(path) ? raw.split('\n').map((l) => l.trim()).filter((l) => l !== '') : raw
    if (value.length === 0) missing.push(path)
    else secrets[path] = value
  }
  return { secrets, missing }
}

async function confirm() {
  const version = target.value
  if (busy.value || !version) return
  // The dialog does not trap the keyboard, so the editor can be opened behind it after the dialog opened.
  if (props.editing) {
    refusal.value = { message: 'Save or cancel the overrides edit first: a rollback replaces the overrides.', paths: [] }
    return
  }
  if (!props.ready) {
    refusal.value = { message: 'The configuration is being read again: wait for it, then confirm.', paths: [] }
    return
  }
  const { secrets, missing } = collect()
  if (missing.length > 0) {
    refusal.value = { message: `Enter a value for every listed path: ${missing.join(', ')}.`, paths: [] }
    return
  }
  busy.value = true
  try {
    const result = await rollbackDesired(props.deviceId, version.id, secrets)
    reportLateResult('info', result.message, { action: 'Roll back desired configuration', resource: 'Device', resourceId: props.deviceId })
    if (gone) return
    closeDialog()
    notice.value = { kind: 'success', text: result.message }
    emit('refresh')
  } catch (e) {
    if (gone) return
    const err = e instanceof RollbackRefused ? e : new RollbackRefused(0, '', 'The rollback failed.', [])
    entered.value = {}
    if (err.status === 409 && err.paths.length > 0) {
      prompt.value = err.paths
      refusal.value = { message: err.message, paths: [] }
    } else if (err.status === 409) {
      // Changed while the rollback was prepared: no secret is asked for, the page reads again.
      closeDialog()
      notice.value = { kind: 'error', text: err.message }
      emit('refresh')
    } else {
      refusal.value = { message: err.message, paths: err.status === 422 ? err.paths : [] }
    }
  } finally {
    if (!gone) busy.value = false
  }
}
</script>

<style scoped>
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 16px; }
.header-row h2 { margin: 0 0 8px; font-size: 16px; }
.hint { color: #475569; font-size: 13px; margin: 4px 0; }
.list { display: flex; flex-direction: column; gap: 8px; }
.item { display: flex; align-items: center; gap: 12px; padding: 12px; background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; }
.strong { font-weight: 600; }
.spacer { flex: 1; }
.pill { padding: 2px 8px; border-radius: 999px; font-size: 12px; font-weight: 600; background: #e0e7ff; color: #3730a3; }
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; font-size: 14px; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn.primary { background: #2563eb; color: white; border-color: #2563eb; }
.notice { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px; border-radius: 6px; margin-bottom: 8px; }
.notice.success { background: #f0fdf4; color: #166534; }
.notice.error, .live-error { background: #fee2e2; color: #991b1b; padding: 12px; border-radius: 6px; }
.empty-state { padding: 24px; text-align: center; color: #64748b; background: #f9fafb; border-radius: 6px; }
.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.35); display: flex; align-items: center; justify-content: center; padding: 16px; z-index: 50; }
.modal { width: min(640px, 100%); max-height: calc(100vh - 32px); overflow-y: auto; background: #fff; border-radius: 12px; border: 1px solid #e5e7eb; padding: 14px; }
.diff { max-height: 220px; overflow-y: auto; margin: 8px 0; padding-left: 20px; font-size: 13px; }
.error { color: #991b1b; }
.path { margin-right: 6px; }
.field { display: flex; flex-direction: column; gap: 4px; margin: 8px 0; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 12px; border-top: 1px solid #e5e7eb; margin-top: 12px; }
</style>
