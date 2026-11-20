<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" :to="`/devices/${deviceId}`">Back to Device</router-link>
      <h1 class="title">Device Configuration</h1>
      <div class="spacer" />
      <button class="btn" @click="handleRefresh" :disabled="store.loading">Refresh</button>
      <button 
        v-if="store.status?.pending_changes" 
        class="btn primary" 
        @click="handleApply"
        :disabled="store.loading"
      >
        Apply Config
      </button>
    </div>

    <ErrorDisplay
      :error="actionError"
      :show-retry="false"
      data-testid="config-action-error"
      @dismiss="clearActionError"
    />

    <div
      v-if="applyNotice"
      class="card apply-result"
      :class="applyNotice.kind"
      :data-kind="applyNotice.kind"
      data-testid="apply-result"
    >
      <span>{{ applyNotice.deviceId !== String(deviceId) ? `Device ${applyNotice.deviceId}: ` : '' }}{{ applyNotice.text }}</span>
      <button class="btn" data-testid="apply-result-dismiss" @click="applyNotice = null">Dismiss</button>
    </div>

    <div v-if="store.loading" class="card state">Loading...</div>
    <!-- The store sets its error only when a load fails; an action failure shows in the error panel above. -->
    <div v-else-if="store.error" class="card state error">{{ store.error }}</div>

    <template v-else>
      <div v-if="store.status" class="card status-card">
        <div class="status-row">
          <span class="status-label">Status:</span>
          <span v-if="store.status.config_applied && !store.status.pending_changes" class="badge success">Applied</span>
          <span v-else-if="store.status.pending_changes" class="badge warning">Pending</span>
          <span v-else class="badge">Not configured</span>
        </div>
        <div v-if="store.status.last_applied" class="status-row">
          <span class="status-label">Last Applied:</span>
          <span>{{ formatDate(store.status.last_applied) }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">Templates:</span>
          <span>{{ store.status.template_count }} template(s)</span>
        </div>
        <div class="status-row">
          <span class="status-label">Overrides:</span>
          <span>{{ store.status.has_overrides ? 'Yes' : 'No' }}</span>
        </div>
        <div class="status-actions">
          <button class="btn" @click="handleVerify" :disabled="store.loading">Verify Device</button>
        </div>
      </div>

      <div v-if="store.lastVerify" class="card verify-card">
        <h3>Verification Result</h3>
        <div v-if="store.lastVerify.match" class="verify-success">
          Device configuration matches desired state
        </div>
        <div v-else class="verify-failed">
          Configuration drift detected
          <div v-if="store.lastVerify.differences" class="differences">
            <div v-for="(diff, idx) in store.lastVerify.differences" :key="idx" class="diff-item">
              <strong>{{ diff.path }}:</strong> expected <code>{{ JSON.stringify(diff.expected) }}</code>, 
              actual <code>{{ JSON.stringify(diff.actual) }}</code>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="header-row">
          <h2>Templates ({{ store.templates.length }})</h2>
          <div class="spacer" />
          <button class="btn" @click="showAddTemplate = true">+ Add Template</button>
        </div>
        <div v-if="store.templates.length === 0" class="empty-state">
          No templates assigned. Add a template to configure this device.
        </div>
        <div v-else class="templates-list">
          <div v-for="(template, idx) in store.templates" :key="template.id" class="template-item">
            <span class="template-order">{{ idx + 1 }}.</span>
            <div class="template-info">
              <div class="template-name">{{ template.name }}</div>
              <div class="template-meta">
                <span class="pill">{{ template.scope }}</span>
                <span v-if="template.device_type" class="pill">{{ template.device_type }}</span>
              </div>
            </div>
            <button class="btn-icon" @click="handleRemoveTemplate(template.id)" title="Remove">x</button>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="header-row">
          <h2>Device Overrides</h2>
          <div class="spacer" />
          <button v-if="store.overrides && Object.keys(store.overrides).length > 0" class="btn" @click="handleClearOverrides">Clear All</button>
          <button class="btn" @click="toggleEditOverrides">
            {{ editOverrides ? 'Cancel' : 'Edit' }}
          </button>
          <button
            v-if="editOverrides"
            class="btn primary"
            data-testid="save-overrides"
            :disabled="!overridesValid"
            :title="overridesValid ? '' : 'Fix the invalid fields first'"
            @click="handleSaveOverrides"
          >
            Save
          </button>
        </div>
        <div v-if="(!store.overrides || Object.keys(store.overrides).length === 0) && !editOverrides" class="empty-state">
          No overrides set. Click Edit to add device-specific overrides.
        </div>
        <div v-else-if="editOverrides" class="editor">
          <ConfigEditor
            ref="overridesEditor"
            v-model="overridesData"
            v-model:valid="overridesValid"
            :held="overridesHeld"
            :server-errors="refusedKeys"
            :device-id="deviceId"
            :inherited="store.inherited"
            :sources="store.inheritedSources"
          />
          <ul v-if="saveErrors.length > 0" class="save-errors" data-testid="override-save-errors">
            <li v-for="err in saveErrors" :key="err.path"><code>{{ err.path }}</code>: {{ err.message }}</li>
          </ul>
        </div>
        <ConfigView v-else :config="store.overrides" />
      </div>

      <div class="card">
        <div class="header-row">
          <h2>Desired Configuration</h2>
          <div class="spacer" />
          <button class="btn" @click="showSources = !showSources">
            {{ showSources ? 'Hide Sources' : 'Show Sources' }}
          </button>
          <button class="btn" @click="showRawJson = !showRawJson">
            {{ showRawJson ? 'Structured View' : 'Raw JSON' }}
          </button>
        </div>
        <div v-if="!store.desiredConfig" class="empty-state">
          No configuration computed. Add templates or overrides above.
        </div>
        <pre v-else-if="showRawJson" class="code-view">{{ JSON.stringify(store.desiredConfig, null, 2) }}</pre>
        <ConfigView 
          v-else 
          :config="store.desiredConfig" 
          :sources="showSources ? store.sources : undefined"
          :show-source-badges="showSources"
          :expand-all="true"
        />
      </div>

      <div class="card live-card">
        <div class="header-row">
          <h2>Live on device</h2>
          <div class="spacer" />
          <button class="btn" data-testid="live-load" @click="handleLoadLive" :disabled="liveLoading">
            {{ liveLoading ? 'Reading...' : liveConfig ? 'Reload from device' : 'Read from device' }}
          </button>
        </div>
        <div v-if="liveError" class="live-error" data-testid="live-error">{{ liveError }}</div>
        <div v-else-if="liveConfig" data-testid="live-config">
          <ConfigView :config="liveConfig" :expand-all="true" />
        </div>
        <div v-else class="empty-state">
          Not read yet. The device is only contacted when you click the button.
        </div>
      </div>
    </template>

    <!-- What an import stored, and what changed it since. Own state, so a refresh of the page above
         neither hides these nor closes an open entry. -->
    <div class="card" data-testid="stored-card">
      <div class="header-row">
        <h2>Last imported configuration</h2>
        <button
          v-if="stored && isSettings(stored.config)"
          class="btn"
          data-testid="create-template-from-import"
          @click="showFromImport = true"
        >
          Create template from this import
        </button>
      </div>
      <CreateTemplateFromImportDialog
        v-if="showFromImport"
        :device-id="deviceId"
        @close="showFromImport = false"
      />
      <div v-if="storedError" class="live-error" data-testid="stored-config-error">{{ storedError }}</div>
      <div v-else-if="stored" data-testid="stored-config">
        <div class="status-row">
          <span class="status-label">Imported:</span>
          <span data-testid="stored-config-time">{{ stored.last_synced ? formatDate(stored.last_synced) : 'unknown' }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">Last changed:</span>
          <span data-testid="stored-config-changed">{{ formatDate(stored.updated_at) }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">Status:</span>
          <span data-testid="stored-config-status">{{ stored.sync_status }}</span>
        </div>
        <ConfigView v-if="isSettings(stored.config)" :config="stored.config" :expand-all="true" />
        <pre v-else class="code-view">{{ stored.config }}</pre>
      </div>
      <div v-else-if="storedLoaded" class="empty-state" data-testid="stored-config-empty">
        Nothing was imported from this device yet.
      </div>
    </div>

    <div class="card" data-testid="history-card">
      <div class="header-row">
        <h2>History</h2>
      </div>
      <div v-if="historyError" class="live-error" data-testid="history-error">{{ historyError }}</div>
      <div v-else-if="history.length > 0" class="history-list">
        <div v-for="entry in history" :key="entry.id" class="history-item" data-testid="history-entry">
          <div class="history-head">
            <span>{{ formatDate(entry.timestamp) }}</span>
            <span class="pill">{{ entry.source }}</span>
            <span class="history-user">{{ entry.user || '-' }}</span>
            <div class="spacer" />
            <button class="btn" :data-testid="`history-toggle-${entry.id}`" @click="toggleHistoryEntry(entry.id)">
              {{ openEntries.has(entry.id) ? 'Hide' : 'Show' }}
            </button>
          </div>
          <template v-if="openEntries.has(entry.id)">
            <ConfigView v-if="isSettings(entry.config)" :config="entry.config" :expand-all="true" />
            <pre v-else-if="entry.config" class="code-view">{{ entry.config }}</pre>
            <div v-else class="empty-state">No configuration was recorded for this change.</div>
          </template>
        </div>
      </div>
      <div v-else-if="historyLoaded" class="empty-state" data-testid="history-empty">
        No history yet.
      </div>
    </div>

    <div v-if="showAddTemplate" class="modal-overlay" @click.self="closeAddTemplate">
      <div class="modal">
        <h2>Add Template</h2>
        <ErrorDisplay
          :error="addError"
          :show-retry="false"
          data-testid="add-template-error"
          @dismiss="clearAddError"
        />
        <div class="form-group">
          <label>Template ID *</label>
          <input
            v-model.number="newTemplateId"
            type="number"
            class="form-input"
            placeholder="Enter template ID"
            @input="onTemplateIdInput"
          />
          <div v-if="addIdError" class="field-error" data-testid="add-template-id-error">{{ addIdError }}</div>
        </div>
        <div class="form-group">
          <label>Position (optional)</label>
          <input v-model.number="newTemplatePosition" type="number" class="form-input" placeholder="Leave empty to append" />
        </div>
        <div class="modal-actions">
          <button class="btn" @click="closeAddTemplate">Cancel</button>
          <button class="btn primary" :disabled="addingTemplate" @click="handleAddTemplate">Add</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useDeviceConfigNewStore } from '@/stores/deviceConfigNew'
import ConfigView from '@/components/config/ConfigView.vue'
import ConfigEditor from '@/components/config/ConfigEditor.vue'
import CreateTemplateFromImportDialog from '@/components/config/CreateTemplateFromImportDialog.vue'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import { useError } from '@/composables/useError'
import { usePageError, useLateResult } from '@/composables/useNotice'
import {
  getCurrentDeviceConfig,
  getStoredDeviceConfig,
  getConfigHistory,
  type StoredDeviceConfig,
  type ConfigHistoryEntry,
} from '@/api/deviceConfig'
import { formErrorKey, isSettings, nullRemovedSecrets } from '@/utils/config-form'
import { TemplateSaveError, type FieldError } from '@/api/templates'

const route = useRoute()
const store = useDeviceConfigNewStore()

// Follows the route: the router reuses this component when the address changes from one device
// to another, and every save, apply and template change below must target the device on screen.
const deviceId = computed<number | string>(() => route.params.id as string)
const editOverrides = ref(false)
const overridesEditor = ref<InstanceType<typeof ConfigEditor> | null>(null)
const overridesValid = ref(true)
const overridesData = ref<Record<string, any>>({})
// Fields the server refused on the last Save, as the template page lists them.
const saveErrors = ref<FieldError[]>([])
// The same refusals keyed for the form, by relay id, against the body that was sent: the server names
// a list item by its position in that body. They live here because the editor is removed and
// re-created while the page loads, and an edit made since must not move them to another item.
const refusedKeys = ref<Record<string, string>>({})
// The overrides as they were saved, which the editor's required-field guard reads. The editor is
// removed and re-created while the page loads (Verify Device does it), so it lives here, beside the
// body it describes. Only a load of the overrides sets it; no edit does.
const overridesHeld = ref<Record<string, any>>({})
const showSources = ref(false)
const showRawJson = ref(false)
const showAddTemplate = ref(false)
const newTemplateId = ref<number | null>(null)
const newTemplatePosition = ref<number | null>(null)
const liveConfig = ref<Record<string, any> | null>(null)
const liveError = ref<string | null>(null)
const liveLoading = ref(false)
// "Create template from this import": the dialog reads the typed import and the device schema itself.
const showFromImport = ref(false)

// The stored import and its history. A card shows its own failed read, never an alert.
const stored = ref<StoredDeviceConfig | null>(null)
const storedLoaded = ref(false)
const storedError = ref<string | null>(null)
const history = ref<ConfigHistoryEntry[]>([])
const historyLoaded = ref(false)
const historyError = ref<string | null>(null)
const openEntries = ref<Set<number>>(new Set())

// A failed action shows as an error panel under the toolbar. A failed template add shows inside the
// add-template dialog instead, which is open and covers the page when it happens.
// A failure goes to the app notice after the page was left, or while the Add Template dialog covers
// the panel: a device change in this page instance drops the loading state of an action that is
// still open, so its failure can arrive with the dialog open. `addError` stays page-local: once the
// page is left, `handleAddTemplate` sends its failure here instead.
const { error: actionError, setError: setActionError, clearError: clearActionError } = usePageError(
  () => showAddTemplate.value
)
const { error: addError, setError: setAddError, clearError: clearAddError } = useError()
const addIdError = ref('')
// What the last apply came to. It is a result, not an error: an unconfirmed apply is neither a
// failure nor a success, so it has its own card and does not go through ErrorDisplay.
type ApplyNotice = { kind: 'success' | 'unconfirmed' | 'failed'; deviceId: string; text: string }
const applyNotice = ref<ApplyNotice | null>(null)
const reportLateResult = useLateResult()

function loadOverrides(body: Record<string, any> | null | undefined) {
  overridesData.value = JSON.parse(JSON.stringify(body || {}))
  overridesHeld.value = JSON.parse(JSON.stringify(body || {}))
}

watch(() => store.overrides, (newOverrides) => {
  loadOverrides(newOverrides)
}, { immediate: true, deep: true })

// Cancel throws the edit away: the body and the memory go back to the saved overrides, so the next
// Edit starts clean and a key reset in the abandoned edit blocks nothing.
function toggleEditOverrides() {
  saveErrors.value = []
  refusedKeys.value = {}
  if (editOverrides.value) loadOverrides(store.overrides)
  editOverrides.value = !editOverrides.value
}

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

// Counts the device changes of this page. Each handler remembers the count when it starts. After an
// await, a handler that started before a device change stops, even when the route has come back to
// the same id: it must not reload, close or fill anything for what is on screen now.
let routeGeneration = 0

function toggleHistoryEntry(entryId: number) {
  const next = new Set(openEntries.value)
  if (!next.delete(entryId)) next.add(entryId)
  openEntries.value = next
}

function resetStored() {
  showFromImport.value = false
  stored.value = null
  storedLoaded.value = false
  storedError.value = null
  history.value = []
  historyLoaded.value = false
  historyError.value = null
  openEntries.value = new Set()
}

// Counts the reads of the stored import and its history. An answer fills the cards only while it is
// the newest read: an older refresh that answers last must not replace a newer one.
let storedReadSeq = 0

// Reads what the import stored and its history. The two reads fail apart, and an answer that arrives
// after the device on screen changed belongs to the old device and fills nothing.
async function loadStored() {
  const id = deviceId.value
  const generation = routeGeneration
  const seq = ++storedReadSeq
  const current = () => routeGeneration === generation && storedReadSeq === seq
  const readStored = getStoredDeviceConfig(id).then(
    (row) => {
      if (!current()) return
      stored.value = row
      storedError.value = null
      storedLoaded.value = true
    },
    (e: any) => {
      if (!current()) return
      stored.value = null
      storedError.value = e?.response?.data?.error?.message || e?.message || 'Failed to read the imported configuration'
    },
  )
  const readHistory = getConfigHistory(id).then(
    (rows) => {
      if (!current()) return
      history.value = rows
      historyError.value = null
      historyLoaded.value = true
    },
    (e: any) => {
      if (!current()) return
      history.value = []
      historyError.value = e?.response?.data?.error?.message || e?.message || 'Failed to read the configuration history'
    },
  )
  await Promise.all([readStored, readHistory])
}

async function handleRefresh() {
  const id = deviceId.value
  const generation = routeGeneration
  void loadStored()
  try {
    await store.refresh(id)
    if (routeGeneration !== generation) return
    loadOverrides(store.overrides)
  } catch (e: any) {
    console.error('Refresh failed:', e)
  }
}

// Read-only view of what the device reports now. Loaded on demand: an
// unreachable device shows the error here and never fails the page. A read that
// finishes after the device on screen changed belongs to the old device: it
// fills nothing and clears nothing, because the device change already reset the panel.
async function handleLoadLive() {
  const id = deviceId.value
  const generation = routeGeneration
  liveLoading.value = true
  liveError.value = null
  try {
    const record = await getCurrentDeviceConfig(id)
    if (routeGeneration !== generation) return
    // The endpoint returns the stored record; its "config" member holds the
    // device settings. Fall back to the whole object for a bare payload.
    liveConfig.value = (record?.config as Record<string, any> | undefined) ?? record
  } catch (e: any) {
    if (routeGeneration !== generation) return
    liveConfig.value = null
    liveError.value = e?.response?.data?.error?.message || e?.message || 'Failed to read configuration from device'
  } finally {
    if (routeGeneration === generation) liveLoading.value = false
  }
}

function closeAddTemplate() {
  showAddTemplate.value = false
  addIdError.value = ''
  clearAddError()
}

// Typing another id drops both the empty-id message and a failure of the last add.
function onTemplateIdInput() {
  addIdError.value = ''
  clearAddError()
}

// A failure or an apply result that arrives after the page moved to another device is still shown:
// the apply changed a physical device. The error panel names its device in its context line, and the
// result card names it when it is not the device on screen.
// True from the click until the add and its refresh are done. Add is off meanwhile, so two adds never
// overlap: a second add failing could otherwise hide the first one's failed refresh.
const addingTemplate = ref(false)

async function handleAddTemplate() {
  if (addingTemplate.value) return
  clearAddError()
  if (!newTemplateId.value) {
    addIdError.value = 'Template ID is required'
    return
  }
  addIdError.value = ''

  const id = deviceId.value
  const generation = routeGeneration
  addingTemplate.value = true
  try {
    await store.addTemplate(
      id, 
      newTemplateId.value, 
      newTemplatePosition.value === null ? undefined : newTemplatePosition.value
    )
    if (routeGeneration !== generation) return
    await handleRefresh()
    if (routeGeneration !== generation) return
    showAddTemplate.value = false
    newTemplateId.value = null
    newTemplatePosition.value = null
  } catch (e: any) {
    const context = { action: 'Add template', resource: 'Device', resourceId: id }
    if (routeGeneration !== generation || !showAddTemplate.value) {
      // The dialog was closed (cancelled, or left with the old device): nothing shows its panel, so
      // the failure goes to the page panel, which names the device.
      setActionError(e, context)
      return
    }
    setAddError(e, context)
  } finally {
    if (routeGeneration === generation) addingTemplate.value = false
  }
}

async function handleRemoveTemplate(templateId: number) {
  if (!confirm('Remove this template from the device?')) return

  clearActionError()
  const id = deviceId.value
  const generation = routeGeneration
  try {
    await store.removeTemplate(id, templateId)
    if (routeGeneration !== generation) return
    await handleRefresh()
  } catch (e: any) {
    setActionError(e, { action: 'Remove template', resource: 'Device', resourceId: id })
  }
}

async function handleSaveOverrides() {
  if (!overridesValid.value) return
  const id = deviceId.value
  const generation = routeGeneration
  saveErrors.value = []
  refusedKeys.value = {}
  clearActionError()
  let sent: Record<string, any> = {}
  try {
    sent = nullRemovedSecrets(overridesData.value, overridesHeld.value, overridesEditor.value?.schema)
    await store.saveOverrides(id, sent)
    if (routeGeneration !== generation) return
    editOverrides.value = false
    await handleRefresh()
  } catch (e: any) {
    if (e instanceof TemplateSaveError) {
      // The server refused values the form let through. Keep the edit open and list them.
      if (routeGeneration === generation) {
        saveErrors.value = e.fields
        refusedKeys.value = Object.fromEntries(e.fields.map(f => [formErrorKey(sent, f.path), f.message]))
      }
      return
    }
    setActionError(e, { action: 'Save overrides', resource: 'Device', resourceId: id })
  }
}

async function handleClearOverrides() {
  if (!confirm('Clear all device overrides?')) return

  clearActionError()
  const id = deviceId.value
  const generation = routeGeneration
  try {
    await store.clearOverrides(id)
    if (routeGeneration !== generation) return
    await handleRefresh()
  } catch (e: any) {
    setActionError(e, { action: 'Clear overrides', resource: 'Device', resourceId: id })
  }
}

async function handleApply() {
  if (!confirm('Apply this configuration to the device? This will modify the physical device.')) return

  clearActionError()
  applyNotice.value = null
  const id = deviceId.value
  const generation = routeGeneration
  try {
    const result = await store.apply(id)
    let outcome: ApplyNotice
    if (result.unconfirmed) {
      // A Wi-Fi network change that the device stopped answering after. Not a failure and not a
      // success: the device stays pending until a later read confirms it.
      outcome = {
        kind: 'unconfirmed',
        deviceId: String(id),
        text:
          'Applied, unconfirmed: the device may have changed network and has not answered since. ' +
          'Find the device on its new network, then verify it.',
      }
    } else if (result.success) {
      outcome = {
        kind: 'success',
        deviceId: String(id),
        text: `Applied successfully. ${result.applied_count} field(s) updated.`,
      }
    } else {
      outcome = {
        kind: 'failed',
        deviceId: String(id),
        text: `Apply completed with errors. ${result.failed_count} field(s) failed.`,
      }
    }
    applyNotice.value = outcome
    // Apply changes a physical device: after the user left the page the card is gone, so the app
    // notice says what it came to (ginsys/shelly-manager#660).
    reportLateResult(outcome.kind === 'success' ? 'info' : 'warning', outcome.text, {
      action: 'Apply configuration',
      resource: 'Device',
      resourceId: id,
    })
    if (routeGeneration !== generation) return
    await handleRefresh()
  } catch (e: any) {
    setActionError(e, { action: 'Apply configuration', resource: 'Device', resourceId: id })
  }
}

async function handleVerify() {
  clearActionError()
  const id = deviceId.value
  try {
    await store.verify(id)
  } catch (e: any) {
    setActionError(e, { action: 'Verify device', resource: 'Device', resourceId: id })
  }
}

// The store is shared by every instance of this page. A new instance starts from an empty store, which
// also drops any answer that an earlier instance still waits for. The count makes this instance's own
// handlers stop when the page is left, so none of them touches what the next page shows.
onMounted(() => {
  store.reset()
  handleRefresh()
})

onUnmounted(() => {
  routeGeneration++
})

// A new device id means another device: drop everything that belonged to the old one, then load
// the new one. An empty id means the route is leaving this page, so there is nothing to load.
watch(deviceId, (next, previous) => {
  if (!next || next === previous) return
  routeGeneration++
  store.reset()
  loadOverrides(null)
  editOverrides.value = false
  showAddTemplate.value = false
  addingTemplate.value = false
  newTemplateId.value = null
  newTemplatePosition.value = null
  addIdError.value = ''
  clearAddError()
  clearActionError()
  applyNotice.value = null
  liveConfig.value = null
  liveError.value = null
  liveLoading.value = false
  resetStored()
  handleRefresh()
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }

.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; font-size: 14px; }
.btn:hover { background: #f8fafc; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn.primary { background: #2563eb; color: white; border-color: #2563eb; }
.btn.primary:hover { background: #1d4ed8; }

.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 16px; }
.state { text-align: center; color: #64748b; padding: 32px; }
.state.error { color: #b91c1c; }

.apply-result { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.apply-result.success { background: #f0fdf4; border-color: #86efac; color: #166534; }
.apply-result.unconfirmed { background: #fffbeb; border-color: #fcd34d; color: #92400e; }
.apply-result.failed { background: #fef2f2; border-color: #fca5a5; color: #991b1b; }
.field-error { color: #b91c1c; font-size: 13px; margin-top: 4px; }

.status-card { background: #f9fafb; }
.status-row { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
.status-label { font-weight: 600; min-width: 120px; }
.status-actions { margin-top: 12px; display: flex; gap: 8px; }

.badge { padding: 4px 12px; border-radius: 999px; font-size: 13px; font-weight: 600; background: #e2e8f0; color: #334155; }
.badge.success { background: #dcfce7; color: #065f46; }
.badge.warning { background: #fef3c7; color: #92400e; }

.verify-card h3 { margin: 0 0 12px 0; }
.verify-success { padding: 12px; background: #dcfce7; color: #065f46; border-radius: 6px; }
.verify-failed { padding: 12px; background: #fee2e2; color: #991b1b; border-radius: 6px; }
.differences { margin-top: 12px; }
.diff-item { padding: 8px; background: #fff; border: 1px solid #fecaca; border-radius: 4px; margin-bottom: 6px; font-size: 13px; }
.diff-item code { background: #fef2f2; padding: 2px 4px; border-radius: 3px; }

.header-row { display: flex; align-items: center; margin-bottom: 16px; }
.header-row h2 { margin: 0; font-size: 16px; }

.live-error { padding: 12px; background: #fee2e2; color: #991b1b; border-radius: 6px; }

.empty-state { padding: 24px; text-align: center; color: #64748b; background: #f9fafb; border-radius: 6px; }

.history-list { display: flex; flex-direction: column; gap: 8px; }
.history-item { padding: 12px; background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; }
.history-head { display: flex; align-items: center; gap: 12px; }
.history-user { color: #64748b; }

.templates-list { display: flex; flex-direction: column; gap: 8px; }
.template-item { display: flex; align-items: center; gap: 12px; padding: 12px; background: #f9fafb; border: 1px solid #e5e7eb; border-radius: 6px; }
.template-order { font-weight: 600; color: #64748b; min-width: 24px; }
.template-info { flex: 1; }
.template-name { font-weight: 600; margin-bottom: 4px; }
.template-meta { display: flex; gap: 6px; }

.pill { padding: 2px 8px; border-radius: 999px; font-size: 12px; font-weight: 600; background: #e0e7ff; color: #3730a3; }

.btn-icon { padding: 4px 8px; border: none; background: #fee2e2; color: #991b1b; border-radius: 4px; cursor: pointer; font-size: 18px; line-height: 1; }
.btn-icon:hover { background: #fecaca; }

.editor { margin-top: 12px; }
.save-errors { margin: 8px 0 0; padding: 8px 12px 8px 28px; background: #fee2e2; color: #991b1b; border-radius: 6px; font-size: 13px; }
.code-textarea { width: 100%; padding: 12px; border: 1px solid #cbd5e1; border-radius: 6px; font-family: ui-monospace, monospace; font-size: 13px; resize: vertical; }
.code-view { background: #f8fafc; padding: 12px; border-radius: 6px; overflow-x: auto; font-family: ui-monospace, monospace; font-size: 13px; margin: 0; }

.sources-view { display: flex; flex-direction: column; gap: 8px; }
.source-item { display: flex; align-items: center; gap: 12px; padding: 8px; background: #f9fafb; border-radius: 4px; }
.source-path { font-weight: 600; font-size: 13px; font-family: ui-monospace, monospace; }
.source-badge { padding: 2px 8px; border-radius: 999px; font-size: 12px; background: #dbeafe; color: #1e40af; }

.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; padding: 24px; border-radius: 8px; max-width: 500px; width: 90%; }
.modal h2 { margin-top: 0; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; margin-bottom: 6px; font-weight: 600; font-size: 14px; }
.form-input { width: 100%; padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-size: 14px; }
.modal-actions { display: flex; gap: 12px; justify-content: flex-end; margin-top: 16px; }
</style>
