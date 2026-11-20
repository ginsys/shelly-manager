<template>
  <fieldset v-if="isGroup" class="group" :data-path="key">
    <legend>{{ label }}</legend>
    <p v-if="schema.description" class="field-hint">{{ schema.description }}</p>
    <ConfigSchemaField
      v-for="(childSchema, childName) in fields"
      :key="childName"
      :name="String(childName)"
      :schema="childSchema"
      :path="[...path, String(childName)]"
      :overrides="overrides"
      :inherited="inherited"
      :sources="sources"
      :layer="layer"
      :errors="errors"
      @set="(p, v) => emit('set', p, v)"
      @reset="p => emit('reset', p)"
    />
  </fieldset>

  <fieldset v-else-if="isItemList" class="group item-list" :data-path="key">
    <legend>{{ label }}</legend>
    <p v-if="schema.description" class="field-hint">{{ schema.description }}</p>
    <ConfigSchemaField
      v-for="id in itemIds"
      :key="id"
      :name="String(id)"
      :legend="itemLegend(id)"
      :schema="schema.items!"
      :path="[...path, itemKey(id)]"
      :overrides="overrides"
      :inherited="inherited"
      :sources="sources"
      :layer="layer"
      :errors="errors"
      @set="(p, v) => emit('set', p, v)"
      @reset="p => emit('reset', p)"
    />
    <p v-if="itemIds.length === 0" class="field-note" data-note="no-items">
      The number of items is set by the device or a template.
    </p>
  </fieldset>

  <div v-else class="field" :class="{ overridden, readonly: readOnly, invalid: !!error }" :data-path="key">
    <div class="field-head">
      <label :for="inputId">{{ label }}</label>
      <span v-if="overridden" class="tag override">Override</span>
      <span v-if="readOnly" class="tag readonly-tag">Not applied</span>
    </div>
    <p v-if="schema.description" class="field-hint">{{ schema.description }}</p>

    <input
      v-if="schema.type === 'boolean'"
      :id="inputId"
      type="checkbox"
      :checked="effective === true"
      :disabled="readOnly"
      @change="emit('set', path, ($event.target as HTMLInputElement).checked)"
    />

    <select
      v-else-if="schema.enum"
      :id="inputId"
      class="form-select"
      :value="overridden ? enumIndex(override) : ''"
      :disabled="readOnly"
      @change="onEnumChange(($event.target as HTMLSelectElement).value)"
    >
      <option value="">{{ inheritedValue !== undefined ? `Inherited (${formatValue(inheritedValue)})` : 'Not set' }}</option>
      <option v-for="(opt, idx) in schema.enum" :key="idx" :value="String(idx)">{{ formatValue(opt) }}</option>
    </select>

    <input
      v-else-if="schema.type === 'integer' || schema.type === 'number'"
      :id="inputId"
      type="number"
      class="form-input"
      :value="boxValue"
      :min="schema.minimum"
      :max="schema.maximum"
      :step="schema.type === 'integer' ? 1 : 'any'"
      :placeholder="placeholder"
      :disabled="readOnly"
      @input="onNumberInput($event.target as HTMLInputElement)"
      @change="draft = null"
      @blur="draft = null"
    />

    <textarea
      v-else-if="isLineList"
      :id="inputId"
      class="form-textarea"
      rows="3"
      :value="linesText"
      :placeholder="placeholder"
      :disabled="readOnly"
      @input="onLinesInput(($event.target as HTMLTextAreaElement).value)"
      @change="linesDraft = null"
      @blur="linesDraft = null"
    />

    <textarea
      v-else-if="schema.type === 'array' || schema.type === 'object'"
      :id="inputId"
      class="form-textarea"
      rows="3"
      :value="jsonValue"
      :placeholder="placeholder"
      :disabled="readOnly"
      @input="onJsonInput(($event.target as HTMLTextAreaElement).value)"
    />

    <input
      v-else
      :id="inputId"
      :type="schema.format === 'password' ? 'password' : 'text'"
      class="form-input"
      :value="boxValue"
      :maxlength="schema.maxLength"
      :placeholder="placeholder"
      :disabled="readOnly"
      @input="onTextInput(($event.target as HTMLInputElement).value)"
      @change="draft = null"
      @blur="draft = null"
    />

    <p v-if="!overridden && inheritedValue !== undefined" class="field-inherited">
      Inherited: {{ schema.format === 'password' ? '••••' : inheritedText }}<span v-if="source"> from {{ source }}</span>
    </p>
    <p v-if="clearsSecret" class="field-note" data-note="clears-secret">The stored password is removed when you save.</p>
    <p v-if="readOnly" class="field-note">Apply cannot write this field to the device yet.</p>
    <p v-if="error" class="field-error" role="alert">{{ error }}</p>

    <div v-if="overridden || canUseDefault || keepsSecret || canEmptyList" class="field-actions">
      <button v-if="canEmptyList" type="button" class="link-btn" data-action="empty-list" @click="onEmptyList">Use an empty list</button>
      <button v-if="keepsSecret" type="button" class="link-btn" data-action="clear-secret" @click="emit('set', path, '')">Clear stored password</button>
      <button v-if="overridden" type="button" class="link-btn" data-action="reset" @click="onReset">Reset</button>
      <button v-if="canUseDefault" type="button" class="link-btn" data-action="use-default" @click="onUseDefault">
        Use default ({{ formatValue(schema.default) }})
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { SchemaProperty } from '@/api/typedConfig'
import {
  clone,
  formatValue,
  getAt,
  hasAt,
  isItem,
  itemKey,
  listIds,
  pathKey,
  SAVED_SECRET_MASK,
  sourceKey,
  sameValue,
  subtreeApplicable,
  type ConfigBody,
  type ConfigPath,
} from '@/utils/config-form'

interface Props {
  name: string
  // The legend of an item of a list, which has no name of its own.
  legend?: string
  schema: SchemaProperty
  path: ConfigPath
  overrides: ConfigBody
  inherited?: ConfigBody | null
  sources?: Record<string, string>
  // True in a template form, where a saved password belongs to the body being edited.
  layer?: boolean
  errors: Record<string, string>
}

const props = defineProps<Props>()
const emit = defineEmits<{
  set: [path: ConfigPath, value: any]
  reset: [path: ConfigPath]
}>()

const key = computed(() => pathKey(props.path))
const inputId = computed(() => `cfg-${key.value.replace(/\./g, '-')}`)
const label = computed(() => props.legend || props.schema.title || formatLabel(props.name))
const isGroup = computed(() => props.schema.type === 'object' && !!props.schema.properties)

// The id of a list item says which item it is. It is no field to set or reset: an item without it
// would be read as item 0.
const fields = computed(() => {
  const all = props.schema.properties ?? {}
  if (!isItem(props.path[props.path.length - 1] ?? '')) return all
  return Object.fromEntries(Object.entries(all).filter(([name]) => name !== 'id'))
})

// A list of objects with an id (relays) is one fieldset per item, each item addressed by its id
// (see config-form.ts). It shows the items of the inherited list and of the override list, and
// offers no add or remove: the schema fixes no length, and how many channels there are is the
// device's own. A list that cannot be addressed by id (two items of one id) stays a JSON box.
const inheritedList = computed(() => getAt(props.inherited, props.path))
const ownList = computed(() => getAt(props.overrides, props.path))
const addressable = computed(() =>
  [ownList.value, inheritedList.value].every(list => list === undefined || (Array.isArray(list) && listIds(list) !== null)))
const isItemList = computed(() => props.schema.type === 'array' && !!props.schema.items?.properties && addressable.value)
const itemIds = computed(() => {
  const ids = new Set<number>()
  for (const list of [inheritedList.value, ownList.value]) {
    if (Array.isArray(list)) (listIds(list) ?? []).forEach(id => ids.add(id))
  }
  return [...ids].sort((a, b) => a - b)
})

function itemLegend(id: number): string {
  const itemPath = [...props.path, itemKey(id)]
  const name = getAt(props.overrides, [...itemPath, 'name']) ?? getAt(props.inherited, [...itemPath, 'name'])
  return typeof name === 'string' && name !== '' ? `Item ${id}: ${name}` : `Item ${id}`
}

// A list of text values is one value per line, never JSON.
const isLineList = computed(() => props.schema.type === 'array' && props.schema.items?.type === 'string')

const overridden = computed(() => hasAt(props.overrides, props.path))
const override = computed(() => getAt(props.overrides, props.path))
const inheritedValue = computed(() => getAt(props.inherited, props.path))
const effective = computed(() => (overridden.value ? override.value : inheritedValue.value))
const source = computed(() => {
  const at = sourceKey(props.inherited, props.path)
  return at === undefined ? undefined : props.sources?.[at]
})
const readOnly = computed(() => !subtreeApplicable(props.schema))
// A list the form cannot split into items is one JSON box, which shows the errors of what is in it.
const error = computed(() => {
  if (props.errors[key.value] !== undefined) return props.errors[key.value]
  if (isGroup.value || isItemList.value) return undefined
  const inner = Object.keys(props.errors).find(k => k.startsWith(`${key.value}.`))
  return inner === undefined ? undefined : props.errors[inner]
})

// A list of text values reads as its entries, not as JSON.
function shown(value: any, separator: string): string {
  return Array.isArray(value) && value.every(entry => typeof entry === 'string')
    ? value.join(separator)
    : formatValue(value)
}

const inheritedText = computed(() => shown(inheritedValue.value, ', '))

const placeholder = computed(() => {
  if (inheritedValue.value !== undefined) {
    return props.schema.format === 'password' ? '••••' : shown(inheritedValue.value, '\n')
  }
  return props.schema.default !== undefined ? shown(props.schema.default, '\n') : ''
})

// A template form shows a saved password as the mask. A save that leaves it out keeps it, so the
// form offers an explicit way to remove it: an empty string clears it on the server.
const isSavedSecret = computed(() =>
  !!props.layer && props.schema.format === 'password' && inheritedValue.value === SAVED_SECRET_MASK)
const keepsSecret = computed(() => isSavedSecret.value && !overridden.value)
const clearsSecret = computed(() => isSavedSecret.value && overridden.value && override.value === '')

const canUseDefault = computed(() =>
  props.schema.default !== undefined &&
  !readOnly.value &&
  JSON.stringify(override.value) !== JSON.stringify(props.schema.default),
)

// A JSON field that does not parse is kept as its raw text, so validation reports it and save
// stays blocked, while the text the user typed stays in the box.
const jsonValue = computed(() => {
  if (!overridden.value) return ''
  return typeof override.value === 'string' ? override.value : JSON.stringify(override.value, null, 2)
})

function formatLabel(name: string): string {
  return name.replace(/_/g, ' ').replace(/^./, s => s.toUpperCase())
}

function enumIndex(value: any): string {
  const idx = (props.schema.enum || []).findIndex(opt => opt === value)
  return idx < 0 ? '' : String(idx)
}

function onEnumChange(value: string) {
  if (value === '') {
    emit('reset', props.path)
    return
  }
  emit('set', props.path, props.schema.enum![Number(value)])
}

// The text being typed. A value equal to the inherited one is no override, so the editor removes
// it, and the box would then render empty under the user's hands: typing "plug2" over an inherited
// "plug" would lose "plug". The draft keeps the box on what was typed until the box is left, or
// Reset or Use default is chosen.
const draft = ref<string | null>(null)

// The box shows the draft only while the model is what typing that draft leaves: the override
// equals it, or there is none because it equals the inherited value, or the box is empty. A model
// that changed from outside (a refresh, another device) shows its own value, whatever was typed.
const boxValue = computed(() => {
  const model = overridden.value ? override.value : ''
  if (draft.value === null) return model
  const typed = props.schema.type === 'integer' || props.schema.type === 'number'
    ? Number(draft.value)
    : draft.value
  const consistent = draft.value === ''
    ? !overridden.value
    : overridden.value
      ? sameValue(override.value, typed)
      : sameValue(inheritedValue.value, typed)
  return consistent ? draft.value : model
})

function onReset() {
  draft.value = null
  linesDraft.value = null
  emit('reset', props.path)
}

function onUseDefault() {
  draft.value = null
  linesDraft.value = null
  emit('set', props.path, clone(props.schema.default))
}

function onNumberInput(input: HTMLInputElement) {
  draft.value = input.value
  if (input.value === '') {
    emit('reset', props.path)
    return
  }
  emit('set', props.path, input.valueAsNumber)
}

function onTextInput(value: string) {
  draft.value = value
  if (value === '') {
    emit('reset', props.path)
    return
  }
  emit('set', props.path, value)
}

function parseLines(text: string): string[] {
  return text.split('\n').map(line => line.trim()).filter(line => line !== '')
}

// The text being typed, kept like the draft of a text box: a trailing newline is no entry yet, and
// the box must not drop it under the user's hands.
const linesDraft = ref<string | null>(null)
const linesText = computed(() => {
  const model = overridden.value && Array.isArray(override.value) ? override.value.join('\n') : ''
  if (linesDraft.value === null) return model
  // An entry list equal to the inherited one is no override, so the box compares with that.
  const own = overridden.value ? override.value : inheritedValue.value
  const entries = Array.isArray(own) ? own : []
  return sameValue(parseLines(linesDraft.value), entries) ? linesDraft.value : model
})

// Emptying the box removes the override and brings the inherited list back, so an empty list, which
// the server takes as "no entries", is its own action.
const canEmptyList = computed(() =>
  isLineList.value && !readOnly.value && Array.isArray(inheritedValue.value) && inheritedValue.value.length > 0 &&
  !(overridden.value && Array.isArray(override.value) && override.value.length === 0))

function onEmptyList() {
  linesDraft.value = null
  emit('set', props.path, [])
}

function onLinesInput(text: string) {
  linesDraft.value = text
  const entries = parseLines(text)
  if (entries.length === 0) {
    emit('reset', props.path)
    return
  }
  emit('set', props.path, entries)
}

function onJsonInput(text: string) {
  if (text.trim() === '') {
    emit('reset', props.path)
    return
  }
  try {
    emit('set', props.path, JSON.parse(text))
  } catch {
    emit('set', props.path, text)
  }
}
</script>

<style scoped>
.group { border: 1px solid #e5e7eb; border-radius: 6px; padding: 8px 12px 12px; margin: 0; display: flex; flex-direction: column; gap: 12px; }
.group legend { font-size: 13px; font-weight: 600; color: #334155; padding: 0 4px; }
.field { display: flex; flex-direction: column; gap: 4px; }
.field-head { display: flex; align-items: center; gap: 6px; }
.field-head label { font-size: 13px; font-weight: 500; color: #374151; }
.tag { font-size: 11px; font-weight: 600; padding: 1px 6px; border-radius: 999px; }
.tag.override { background: #dbeafe; color: #1e40af; }
.tag.readonly-tag { background: #f1f5f9; color: #64748b; }
.field-hint, .field-inherited, .field-note, .field-error { font-size: 12px; margin: 0; }
.field-hint, .field-inherited, .field-note { color: #64748b; }
.field-error { color: #b91c1c; }
.field-actions { display: flex; gap: 12px; }
.link-btn { border: none; background: none; padding: 0; color: #2563eb; cursor: pointer; font-size: 12px; }
.link-btn:hover { text-decoration: underline; }
.form-input, .form-select, .form-textarea { padding: 8px 10px; border: 1px solid #d1d5db; border-radius: 4px; font-size: 14px; }
.form-textarea { font-family: ui-monospace, monospace; font-size: 13px; resize: vertical; }
.invalid .form-input, .invalid .form-select, .invalid .form-textarea { border-color: #b91c1c; }
.readonly { opacity: 0.75; }
input[type="checkbox"] { width: 18px; height: 18px; }
</style>
