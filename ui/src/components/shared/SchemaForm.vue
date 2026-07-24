<template>
  <div class="schema-form">
    <div v-for="field in fields" :key="field.name" class="form-field">
      <label class="field-label" :for="fieldID(field.name)">
        {{ field.name }}
        <span v-if="field.required" aria-hidden="true" class="required-indicator">*</span>
      </label>
      <p class="field-description">{{ field.description }}</p>

      <select
        v-if="field.enum"
        :id="fieldID(field.name)"
        :value="values[field.name]"
        :aria-invalid="Boolean(errors[field.name])"
        :aria-describedby="errors[field.name] ? errorID(field.name) : undefined"
        class="field-input"
        @change="update(field.name, selectedValue(field, $event))"
      >
        <option :value="EMPTY_ENUM">Select…</option>
        <option
          v-for="(option, index) in field.enum"
          :key="index"
          :value="option"
        >
          {{ formatOption(option) }}
        </option>
      </select>

      <label v-else-if="field.type === 'boolean'" class="field-checkbox">
        <input
          :id="fieldID(field.name)"
          type="checkbox"
          :checked="values[field.name] === true"
          :aria-invalid="Boolean(errors[field.name])"
          :aria-describedby="errors[field.name] ? errorID(field.name) : undefined"
          @change="update(field.name, ($event.target as HTMLInputElement).checked)"
        />
        <span>Enabled</span>
      </label>

      <textarea
        v-else-if="field.type === 'array' || field.type === 'object'"
        :id="fieldID(field.name)"
        :value="textValue(field.name)"
        :aria-invalid="Boolean(errors[field.name])"
        :aria-describedby="errors[field.name] ? errorID(field.name) : undefined"
        class="field-textarea"
        rows="6"
        @input="update(field.name, ($event.target as HTMLTextAreaElement).value)"
      />

      <input
        v-else-if="field.type === 'number'"
        :id="fieldID(field.name)"
        type="number"
        :value="numberValue(field.name)"
        :min="field.minimum"
        :max="field.maximum"
        :aria-invalid="Boolean(errors[field.name])"
        :aria-describedby="errors[field.name] ? errorID(field.name) : undefined"
        class="field-input"
        @input="updateNumber(field.name, $event)"
      />

      <input
        v-else
        :id="fieldID(field.name)"
        :type="field.sensitive ? 'password' : 'text'"
        :value="textValue(field.name)"
        :pattern="field.pattern"
        :aria-invalid="Boolean(errors[field.name])"
        :aria-describedby="errors[field.name] ? errorID(field.name) : undefined"
        class="field-input"
        @input="update(field.name, ($event.target as HTMLInputElement).value)"
      />

      <p v-if="errors[field.name]" :id="errorID(field.name)" class="field-error">
        {{ errors[field.name] }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PluginSchema } from '@/api/plugin'
import {
  ABSENT_NUMBER,
  EMPTY_ENUM,
  EMPTY_FIELD,
  pluginFormErrors,
  toFieldSchema,
  type FieldSchema,
  type PluginFormTouched,
  type PluginFormValues,
} from '@/utils/plugin-schema'

const props = defineProps<{
  schema: PluginSchema
  values: PluginFormValues
  touched: PluginFormTouched
  showAllErrors?: boolean
}>()

const emit = defineEmits<{
  'update:values': [value: PluginFormValues]
  'update:touched': [value: PluginFormTouched]
}>()

const fields = computed(() => toFieldSchema(props.schema))
const errors = computed(() =>
  pluginFormErrors(props.schema, props.values, props.touched, props.showAllErrors),
)

function fieldID(name: string): string {
  return `plugin-config-${name}`
}

function errorID(name: string): string {
  return `${fieldID(name)}-error`
}

function update(name: string, value: unknown) {
  emit('update:values', { ...props.values, [name]: value })
  emit('update:touched', { ...props.touched, [name]: true })
}

function updateNumber(name: string, event: Event) {
  const input = event.target as HTMLInputElement
  update(name, input.value === '' ? ABSENT_NUMBER : input.valueAsNumber)
}

function textValue(name: string): string {
  const value = props.values[name]
  if (value === EMPTY_FIELD || value === EMPTY_ENUM || value === ABSENT_NUMBER || value == null) {
    return ''
  }
  return String(value)
}

function numberValue(name: string): number | string {
  const value = props.values[name]
  return typeof value === 'number' ? value : ''
}

function selectedValue(field: FieldSchema, event: Event): unknown {
  const select = event.target as HTMLSelectElement & { selectedOptions: HTMLCollectionOf<HTMLOptionElement & { _value?: unknown }> }
  const option = select.selectedOptions[0]
  return option?._value ?? field.enum?.[select.selectedIndex - 1] ?? EMPTY_ENUM
}

function formatOption(value: unknown): string {
  return typeof value === 'string' ? value : JSON.stringify(value)
}
</script>

<style scoped>
.schema-form { display: flex; flex-direction: column; gap: 16px; }
.form-field { display: flex; flex-direction: column; gap: 6px; }
.field-label { font-weight: 600; color: #374151; }
.required-indicator, .field-error { color: #b91c1c; }
.field-description, .field-error { margin: 0; font-size: 0.85rem; }
.field-description { color: #6b7280; }
.field-input, .field-textarea {
  box-sizing: border-box;
  width: 100%;
  padding: 8px 10px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font: inherit;
}
.field-input[aria-invalid="true"], .field-textarea[aria-invalid="true"] { border-color: #b91c1c; }
.field-checkbox { display: flex; align-items: center; gap: 8px; }
</style>
