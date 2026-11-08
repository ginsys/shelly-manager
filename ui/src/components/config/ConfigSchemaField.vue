<template>
  <fieldset v-if="isGroup" class="group" :data-path="key">
    <legend>{{ label }}</legend>
    <p v-if="schema.description" class="field-hint">{{ schema.description }}</p>
    <ConfigSchemaField
      v-for="(childSchema, childName) in schema.properties"
      :key="childName"
      :name="String(childName)"
      :schema="childSchema"
      :path="[...path, String(childName)]"
      :overrides="overrides"
      :inherited="inherited"
      :sources="sources"
      :errors="errors"
      @set="(p, v) => emit('set', p, v)"
      @reset="p => emit('reset', p)"
    />
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
      :value="overridden ? override : ''"
      :min="schema.minimum"
      :max="schema.maximum"
      :step="schema.type === 'integer' ? 1 : 'any'"
      :placeholder="placeholder"
      :disabled="readOnly"
      @input="onNumberInput($event.target as HTMLInputElement)"
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
      :value="overridden ? override : ''"
      :maxlength="schema.maxLength"
      :placeholder="placeholder"
      :disabled="readOnly"
      @input="onTextInput(($event.target as HTMLInputElement).value)"
    />

    <p v-if="!overridden && inheritedValue !== undefined" class="field-inherited">
      Inherited: {{ schema.format === 'password' ? '••••' : formatValue(inheritedValue) }}<span v-if="source"> from {{ source }}</span>
    </p>
    <p v-if="readOnly" class="field-note">Apply cannot write this field to the device yet.</p>
    <p v-if="error" class="field-error" role="alert">{{ error }}</p>

    <div v-if="overridden || canUseDefault" class="field-actions">
      <button v-if="overridden" type="button" class="link-btn" data-action="reset" @click="emit('reset', path)">Reset</button>
      <button v-if="canUseDefault" type="button" class="link-btn" data-action="use-default" @click="emit('set', path, clone(schema.default))">
        Use default ({{ formatValue(schema.default) }})
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SchemaProperty } from '@/api/typedConfig'
import {
  clone,
  formatValue,
  getAt,
  hasAt,
  pathKey,
  subtreeApplicable,
  type ConfigBody,
  type ConfigPath,
} from '@/utils/config-form'

interface Props {
  name: string
  schema: SchemaProperty
  path: ConfigPath
  overrides: ConfigBody
  inherited?: ConfigBody | null
  sources?: Record<string, string>
  errors: Record<string, string>
}

const props = defineProps<Props>()
const emit = defineEmits<{
  set: [path: ConfigPath, value: any]
  reset: [path: ConfigPath]
}>()

const key = computed(() => pathKey(props.path))
const inputId = computed(() => `cfg-${key.value.replace(/\./g, '-')}`)
const label = computed(() => props.schema.title || formatLabel(props.name))
const isGroup = computed(() => props.schema.type === 'object' && !!props.schema.properties)

const overridden = computed(() => hasAt(props.overrides, props.path))
const override = computed(() => getAt(props.overrides, props.path))
const inheritedValue = computed(() => getAt(props.inherited, props.path))
const effective = computed(() => (overridden.value ? override.value : inheritedValue.value))
const source = computed(() => props.sources?.[key.value])
const readOnly = computed(() => !subtreeApplicable(props.schema))
const error = computed(() => props.errors[key.value])

const placeholder = computed(() => {
  if (inheritedValue.value !== undefined) {
    return props.schema.format === 'password' ? '••••' : formatValue(inheritedValue.value)
  }
  return props.schema.default !== undefined ? formatValue(props.schema.default) : ''
})

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

function onNumberInput(input: HTMLInputElement) {
  if (input.value === '') {
    emit('reset', props.path)
    return
  }
  emit('set', props.path, input.valueAsNumber)
}

function onTextInput(value: string) {
  if (value === '') {
    emit('reset', props.path)
    return
  }
  emit('set', props.path, value)
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
