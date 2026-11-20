<template>
  <div data-testid="bulk-apply-form">
    <h3>Apply Configuration</h3>

    <!-- Edit: the operator types the configuration -->
    <form v-if="stage === 'edit'" @submit.prevent="review">
      <p class="form-note">
        Writes one partial configuration to each selected device: it is stored as that device's
        overrides, never as a template, and then applied to the device. The JSON has the shape of
        the body of <code>PATCH /devices/{id}/overrides/new</code>, for example
        <code>{{ EXAMPLE }}</code>. A setting the device configuration does not know is refused.
      </p>
      <ul class="form-notes" data-testid="bulk-apply-notes">
        <li>A setting set to <code>null</code> removes that override.</li>
        <li>
          A secret sent as <code>{{ MASK }}</code> keeps the stored secret.
        </li>
        <li>
          At most {{ MAX_BULK_APPLY_DEVICES }} devices per call. A call can last up to 5 minutes, and
          devices not reached by then are not contacted. If no answer arrives, check each device
          before sending again.
        </li>
      </ul>

      <div class="form-field">
        <label for="bulk-apply-json">Configuration (JSON)</label>
        <textarea
          id="bulk-apply-json"
          v-model="text"
          class="form-text"
          rows="10"
          spellcheck="false"
          autocomplete="off"
          :placeholder="EXAMPLE"
          data-testid="bulk-apply-json"
        ></textarea>
      </div>

      <div class="form-field checkbox">
        <input
          id="apply-stop-on-error"
          type="checkbox"
          class="form-checkbox"
          :checked="modelValue"
          data-testid="bulk-apply-stop-on-error"
          @change="emit('update:modelValue', ($event.target as HTMLInputElement).checked)"
        />
        <label for="apply-stop-on-error">Stop on first error</label>
      </div>

      <p v-if="overCap" class="form-error" role="alert" data-testid="bulk-apply-cap">
        {{ devices.length }} devices are selected and one call takes at most
        {{ MAX_BULK_APPLY_DEVICES }}. Remove devices from the selection first.
      </p>
      <p v-if="problem" class="form-error" role="alert" data-testid="bulk-apply-problem">
        {{ problem }}
      </p>

      <div class="form-actions">
        <button type="button" class="btn" data-testid="bulk-apply-back" @click="emit('back')">
          Back
        </button>
        <button
          type="submit"
          class="btn primary"
          :disabled="overCap || devices.length === 0"
          data-testid="bulk-apply-review"
        >
          Review
        </button>
      </div>
    </form>

    <!-- Confirm: exactly what will be written, to whom -->
    <div v-else-if="snapshot" data-testid="bulk-apply-confirm">
      <p class="form-note">
        This writes to <strong>{{ snapshot.devices.length }}</strong>
        {{ snapshot.devices.length === 1 ? 'device' : 'devices' }}. Nothing is undone if a later
        device fails.
      </p>
      <p class="form-note" data-testid="bulk-apply-stop-note">
        {{
          snapshot.stopOnError
            ? 'Stop on first error is on: after the first device that does not end applied, the rest are not contacted.'
            : 'Stop on first error is off: every device is tried.'
        }}
      </p>

      <h4>Devices</h4>
      <ul class="confirm-devices" data-testid="bulk-apply-confirm-devices">
        <li v-for="d in snapshot.devices" :key="d.id" data-testid="bulk-apply-confirm-device">
          <span class="confirm-name">{{ d.name || `Device ${d.id}` }}</span>
          <span class="confirm-ip">{{ d.ip }}</span>
        </li>
      </ul>

      <h4>Written to each of them</h4>
      <pre class="confirm-json" data-testid="bulk-apply-confirm-json">{{ snapshot.json }}</pre>

      <div class="form-actions">
        <button type="button" class="btn" data-testid="bulk-apply-edit" @click="stage = 'edit'">
          Edit
        </button>
        <button type="button" class="btn primary" data-testid="bulk-apply-submit" @click="apply">
          Apply
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { MAX_BULK_APPLY_DEVICES } from '@/api/bulkApply'
import type { SelectedDevice } from '@/stores/devices'

// The text a secret is masked as (internal/redact in the backend): sent back as it is, it keeps the
// stored secret.
const MASK = '********'

// A configuration the route accepts, as the placeholder and the note show it.
const EXAMPLE = '{"system":{"device":{"name":"Kitchen"}}}'

const props = defineProps<{
  devices: SelectedDevice[]
  // The shared "stop on first error" flag of the dialog.
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  back: []
  submit: [payload: { deviceIds: number[]; config: Record<string, unknown> }]
}>()

const stage = ref<'edit' | 'confirm'>('edit')
const text = ref('')
const problem = ref('')

// What the operator confirmed: the devices and the JSON as they were when "Review" succeeded. Apply
// sends this, never the live props, so nothing is written that was not on this screen.
const snapshot = ref<{
  devices: SelectedDevice[]
  config: Record<string, unknown>
  json: string
  stopOnError: boolean
} | null>(null)

const overCap = computed(() => props.devices.length > MAX_BULK_APPLY_DEVICES)

function review() {
  problem.value = ''
  if (props.devices.length === 0 || overCap.value) return
  if (text.value.trim() === '') {
    problem.value = 'Type the configuration to write, as a JSON object.'
    return
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(text.value)
  } catch {
    problem.value = 'This is not valid JSON.'
    return
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    problem.value = `The configuration must be a JSON object, such as ${EXAMPLE}.`
    return
  }
  const config = parsed as Record<string, unknown>
  if (Object.keys(config).length === 0) {
    problem.value = 'The configuration is empty. Name at least one setting.'
    return
  }
  snapshot.value = {
    devices: props.devices.map((d) => ({ ...d })),
    config,
    // The parsed value, which is what the request carries: of a key typed twice, the one that wins.
    json: JSON.stringify(config, null, 2),
    stopOnError: props.modelValue
  }
  stage.value = 'confirm'
}

function apply() {
  if (!snapshot.value) return
  emit('submit', {
    deviceIds: snapshot.value.devices.map((d) => d.id),
    config: snapshot.value.config
  })
}

// Forget what was typed and confirmed. The dialog calls it when an answer arrives and when it
// closes, so a secret typed here does not stay in component state.
function reset() {
  stage.value = 'edit'
  text.value = ''
  problem.value = ''
  snapshot.value = null
}

defineExpose({ reset })
</script>

<style scoped>
.form-note {
  margin: 0 0 16px 0;
  font-size: 14px;
  color: #4b5563;
}

.form-notes {
  margin: 0 0 16px 0;
  padding-left: 20px;
  font-size: 14px;
  color: #4b5563;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 16px;
}

.form-field.checkbox {
  flex-direction: row;
  align-items: center;
}

.form-field.checkbox label {
  margin-left: 8px;
}

.form-field label {
  font-size: 14px;
  font-weight: 500;
  color: #374151;
}

.form-checkbox {
  width: 20px;
  height: 20px;
}

.form-text {
  width: 100%;
  box-sizing: border-box;
  padding: 8px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  font-family: monospace;
  font-size: 13px;
}

.form-error {
  margin: 0 0 12px 0;
  font-size: 14px;
  color: #991b1b;
}

.confirm-devices {
  list-style: none;
  margin: 0 0 16px 0;
  padding: 0;
  max-height: 180px;
  overflow-y: auto;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.confirm-devices li {
  display: flex;
  gap: 12px;
  padding: 6px 12px;
  border-bottom: 1px solid #f3f4f6;
  font-size: 13px;
}

.confirm-devices li:last-child {
  border-bottom: none;
}

.confirm-name {
  flex: 1;
  font-weight: 500;
}

.confirm-ip {
  color: #6b7280;
  font-family: monospace;
}

.confirm-json {
  margin: 0 0 16px 0;
  padding: 12px;
  max-height: 260px;
  overflow: auto;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-size: 13px;
}

.form-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 20px;
}

.btn {
  padding: 8px 16px;
  border: 1px solid #cbd5e1;
  background: #fff;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
}

.btn:hover {
  background: #f8fafc;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn.primary {
  background: #2563eb;
  color: white;
  border-color: #2563eb;
}

.btn.primary:hover {
  background: #1d4ed8;
}
</style>
