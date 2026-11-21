<template>
  <div class="modal-overlay">
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="schedule-form-title" data-testid="schedule-form">
      <h2 id="schedule-form-title">{{ schedule ? 'Edit drift schedule' : 'New drift schedule' }}</h2>
      <p v-if="failure" class="form-error" role="alert" data-testid="schedule-form-error">{{ failure }}</p>

      <form @submit.prevent="save">
        <div class="form-group">
          <label for="schedule-name">Name</label>
          <input id="schedule-name" v-model="name" required class="form-input" data-testid="schedule-name" />
        </div>
        <div class="form-group">
          <label for="schedule-description">Description</label>
          <input id="schedule-description" v-model="description" class="form-input" data-testid="schedule-description" />
        </div>

        <div class="form-group">
          <label for="schedule-preset">Runs</label>
          <select id="schedule-preset" v-model="preset" class="form-input" data-testid="schedule-preset">
            <option v-for="p in PRESETS" :key="p.key" :value="p.key">{{ p.label }} ({{ p.spec }})</option>
            <option value="custom">Custom</option>
          </select>
          <input
            v-if="preset === 'custom'"
            v-model="customSpec"
            required
            class="form-input custom-spec"
            aria-label="Cron spec"
            placeholder="0 3 * * *"
            data-testid="schedule-custom-spec"
          />
        </div>

        <div v-if="!stored" class="form-group">
          <label class="inline"><input v-model="enabled" type="checkbox" data-testid="schedule-enabled" /> Enabled</label>
        </div>

        <fieldset class="form-group modes">
          <legend>Devices to check</legend>
          <label class="inline"><input v-model="mode" type="radio" value="all" data-testid="mode-all" /> All devices</label>
          <label class="inline"><input v-model="mode" type="radio" value="devices" data-testid="mode-devices" /> Chosen devices</label>
          <label class="inline"><input v-model="mode" type="radio" value="filter" data-testid="mode-filter" /> Filter by type and status</label>

          <div v-if="mode === 'devices'" class="picker" data-testid="device-picker">
            <p v-if="devicesError" class="form-error" role="alert">{{ devicesError }}</p>
            <p v-else-if="devices.length === 0 && selectedIds.length === 0" class="hint">No devices are stored.</p>
            <label v-for="d in devices" :key="d.id" class="inline device-row">
              <input v-model="selectedIds" type="checkbox" :value="d.id" :data-testid="`device-${d.id}`" />
              {{ d.name || d.mac }} <span class="hint">{{ d.type }}</span>
            </label>
            <div v-if="unlistedIds.length > 0" data-testid="unlisted-devices">
              <p class="hint">These chosen devices are no longer in the list. The server refuses a save that names one, so uncheck it.</p>
              <label v-for="id in unlistedIds" :key="id" class="inline device-row">
                <input v-model="selectedIds" type="checkbox" :value="id" :data-testid="`device-${id}`" />
                Device {{ id }}
              </label>
            </div>
          </div>

          <div v-if="mode === 'filter'" class="filter-fields" data-testid="filter-fields">
            <label for="filter-type">Type</label>
            <input id="filter-type" v-model="filterType" list="schedule-device-types" class="form-input" data-testid="filter-type" />
            <datalist id="schedule-device-types">
              <option v-for="t in deviceTypes" :key="t" :value="t" />
            </datalist>
            <label for="filter-status">Status</label>
            <input id="filter-status" v-model="filterStatus" class="form-input" data-testid="filter-status" />
            <p v-if="extraFilterKeys.length > 0" class="hint" data-testid="filter-extra-keys">
              This filter has fields this form cannot edit ({{ extraFilterKeys.join(', ') }}); saving replaces it.
            </p>
          </div>
        </fieldset>

        <p v-if="blocked" class="hint" role="status" data-testid="save-blocked">{{ blocked }}</p>
        <div class="actions">
          <button type="submit" class="primary-button" :disabled="saving || stale || !!blocked" data-testid="schedule-save">
            {{ saving ? 'Saving...' : 'Save' }}
          </button>
          <button type="button" class="secondary-button" :disabled="saving" data-testid="schedule-cancel" @click="emit('cancel')">
            Cancel
          </button>
        </div>
        <p class="hint save-note" data-testid="save-note">
          Saving can skip a run that is already due. A changed schedule restarts its timer.
        </p>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useDriftStore } from '@/stores/drift'
import { StaleScheduleError, type DriftDeviceFilter, type DriftSchedule, type DriftScheduleInput } from '@/api/drift'
import type { Device } from '@/api/types'

const props = defineProps<{
  schedule?: DriftSchedule | null
  devices: Device[]
  // Why the device list is empty or short, when its load failed.
  devicesError?: string
}>()
const emit = defineEmits<{
  (e: 'saved', schedule: DriftSchedule): void
  (e: 'cancel'): void
}>()

const store = useDriftStore()

const PRESETS = [
  { key: 'hourly', label: 'Hourly', spec: '0 * * * *' },
  { key: 'daily', label: 'Daily at 03:00', spec: '0 3 * * *' },
  { key: 'weekly', label: 'Weekly, Monday 03:00', spec: '0 3 * * 1' }
]

// The filter a stored schedule carries is a raw JSON value: an object, a JSON string, or null.
function readFilter(raw: unknown): Record<string, unknown> {
  let value = raw
  if (typeof value === 'string') {
    try {
      value = JSON.parse(value)
    } catch {
      return {}
    }
  }
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

const stored = props.schedule ?? null
const storedFilter = readFilter(stored?.device_filter)
const storedType = typeof storedFilter.type === 'string' ? storedFilter.type : ''
const storedStatus = typeof storedFilter.status === 'string' ? storedFilter.status : ''
const extraFilterKeys = Object.keys(storedFilter).filter(k => k !== 'type' && k !== 'status')

const name = ref(stored?.name ?? '')
const description = ref(stored?.description ?? '')
// Only a new schedule has the box. An edit sends no `enabled`, so the server keeps it and a save can
// never undo the card's Enable/Disable.
const enabled = ref(true)
const presetOfSpec = PRESETS.find(p => p.spec === stored?.cron_spec)
const preset = ref<string>(stored ? (presetOfSpec?.key ?? 'custom') : 'daily')
const customSpec = ref(stored && !presetOfSpec ? stored.cron_spec : '')
const selectedIds = ref<number[]>([...(stored?.device_ids ?? [])])
const filterType = ref(storedType)
const filterStatus = ref(storedStatus)
const mode = ref<'all' | 'devices' | 'filter'>(
  selectedIds.value.length > 0 ? 'devices' : storedType || storedStatus || extraFilterKeys.length > 0 ? 'filter' : 'all'
)

const saving = ref(false)
const failure = ref('')
// Set once the server refused the save as stale: this form holds the old revision, so Save stays off.
const stale = ref(false)

const deviceTypes = computed(() => [...new Set(props.devices.map(d => d.type).filter(Boolean))].sort())
const unlistedIds = computed(() => selectedIds.value.filter(id => !props.devices.some(d => d.id === id)))

// An empty selection must never mean "all devices" by accident: Save waits until the mode has content.
const blocked = computed(() => {
  if (mode.value === 'devices' && selectedIds.value.length === 0) return 'Choose at least one device'
  if (mode.value === 'filter' && !filterType.value.trim() && !filterStatus.value.trim()) return 'Give a type or a status'
  return ''
})

function cronSpec(): string {
  const p = PRESETS.find(x => x.key === preset.value)
  return p ? p.spec : customSpec.value.trim()
}

// At most one of device_ids and device_filter, and neither for all devices: the server refuses both.
function buildInput(): DriftScheduleInput {
  const input: DriftScheduleInput = {
    name: name.value.trim(),
    description: description.value.trim(),
    cron_spec: cronSpec()
  }
  if (!stored) input.enabled = enabled.value
  if (mode.value === 'devices') {
    input.device_ids = [...selectedIds.value]
  } else if (mode.value === 'filter') {
    const filter: DriftDeviceFilter = {}
    if (filterType.value.trim()) filter.type = filterType.value.trim()
    if (filterStatus.value.trim()) filter.status = filterStatus.value.trim()
    input.device_filter = filter
  }
  return input
}

async function save() {
  if (blocked.value || saving.value || stale.value) return
  saving.value = true
  failure.value = ''
  try {
    const input = buildInput()
    const result = stored ? await store.update(stored.id, input, stored.revision) : await store.create(input)
    emit('saved', result)
  } catch (e: any) {
    if (e instanceof StaleScheduleError) {
      stale.value = true
      failure.value = 'This schedule changed since you opened it. Close this form and open the schedule again.'
    } else {
      failure.value = e?.message || 'The schedule could not be saved'
    }
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: white;
  padding: 24px;
  border-radius: 8px;
  max-width: 560px;
  width: 90%;
  max-height: 90vh;
  overflow-y: auto;
}

.modal h2 {
  margin: 0 0 12px 0;
}

.form-group {
  margin-bottom: 12px;
}

.form-group > label:not(.inline),
.filter-fields label {
  display: block;
  margin-bottom: 4px;
  font-weight: 500;
}

.inline {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 4px 0;
}

.form-input {
  width: 100%;
  padding: 8px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  box-sizing: border-box;
}

.custom-spec {
  margin-top: 8px;
}

.modes {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 8px 12px;
}

.modes legend {
  font-weight: 500;
  padding: 0 4px;
}

.picker {
  margin-top: 8px;
  max-height: 200px;
  overflow-y: auto;
  border-top: 1px solid #e5e7eb;
  padding-top: 8px;
}

.filter-fields {
  margin-top: 8px;
  border-top: 1px solid #e5e7eb;
  padding-top: 8px;
}

.filter-fields .form-input {
  margin-bottom: 8px;
}

.hint {
  margin: 4px 0;
  font-size: 12px;
  color: #64748b;
}

.form-error {
  margin: 0 0 12px 0;
  padding: 8px 12px;
  background: #fee2e2;
  color: #b91c1c;
  border-radius: 6px;
  font-size: 14px;
}

.actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.save-note {
  margin-top: 8px;
}

.primary-button {
  padding: 8px 16px;
  background: #2563eb;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}

.primary-button:hover:not(:disabled) {
  background: #1d4ed8;
}

.secondary-button {
  padding: 8px 16px;
  background: #e5e7eb;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}

.primary-button:disabled,
.secondary-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
