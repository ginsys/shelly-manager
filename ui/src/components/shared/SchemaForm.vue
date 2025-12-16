<template>
  <div class="schema-form">
    <div v-for="(field, key) in schema" :key="key" class="form-field">
      <label class="field-label">
        {{ field.label || key }}
        <span v-if="field.required" class="required-indicator">*</span>
      </label>

      <p v-if="field.description" class="field-description">
        {{ field.description }}
      </p>

      <!-- Text Input -->
      <input
        v-if="field.type === 'string' || !field.type"
        :value="modelValue[key]"
        @input="updateField(key, ($event.target as HTMLInputElement).value)"
        :placeholder="field.placeholder"
        :required="field.required"
        type="text"
        class="field-input"
      />

      <!-- Number Input -->
      <input
        v-else-if="field.type === 'number'"
        :value="modelValue[key]"
        @input="updateField(key, Number(($event.target as HTMLInputElement).value))"
        :placeholder="field.placeholder"
        :required="field.required"
        :min="field.min"
        :max="field.max"
        type="number"
        class="field-input"
      />

      <!-- Textarea -->
      <textarea
        v-else-if="field.type === 'textarea'"
        :value="modelValue[key]"
        @input="updateField(key, ($event.target as HTMLTextAreaElement).value)"
        :placeholder="field.placeholder"
        :required="field.required"
        rows="4"
        class="field-textarea"
      />

      <!-- Checkbox -->
      <label v-else-if="field.type === 'boolean'" class="field-checkbox">
        <input
          type="checkbox"
          :checked="modelValue[key]"
          @change="updateField(key, ($event.target as HTMLInputElement).checked)"
        />
        <span>{{ field.placeholder || 'Enable' }}</span>
      </label>

      <!-- Select Dropdown -->
      <select
        v-else-if="field.type === 'select'"
        :value="modelValue[key]"
        @change="updateField(key, ($event.target as HTMLSelectElement).value)"
        :required="field.required"
        class="field-select"
      >
        <option value="">{{ field.placeholder || 'Select...' }}</option>
        <option
          v-for="option in field.options"
          :key="String(option.value)"
          :value="option.value"
        >
          {{ option.label }}
        </option>
      </select>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { FieldSchema } from '@/types/schema'

interface Props {
  schema: FieldSchema
  modelValue: Record<string, unknown>
}

interface Emits {
  (e: 'update:modelValue', value: Record<string, unknown>): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

function updateField(key: string, value: unknown) {
  const updated = { ...props.modelValue, [key]: value }
  emit('update:modelValue', updated)
}
</script>

<style scoped>
.schema-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-weight: 500;
  font-size: 14px;
  color: #374151;
  display: flex;
  align-items: center;
  gap: 4px;
}

.required-indicator {
  color: #dc2626;
  font-weight: 600;
}

.field-description {
  margin: 0;
  font-size: 13px;
  color: #6b7280;
  line-height: 1.4;
}

.field-input,
.field-select,
.field-textarea {
  padding: 8px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 14px;
  font-family: inherit;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.field-input:focus,
.field-select:focus,
.field-textarea:focus {
  outline: none;
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.field-input::placeholder,
.field-textarea::placeholder {
  color: #9ca3af;
}

.field-textarea {
  resize: vertical;
  min-height: 80px;
}

.field-checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #374151;
  cursor: pointer;
  user-select: none;
}

.field-checkbox input[type="checkbox"] {
  width: 16px;
  height: 16px;
  cursor: pointer;
}

.field-checkbox span {
  line-height: 1;
}

/* Responsive design */
@media (max-width: 640px) {
  .schema-form {
    gap: 14px;
  }

  .field-label {
    font-size: 13px;
  }

  .field-description {
    font-size: 12px;
  }

  .field-input,
  .field-select,
  .field-textarea {
    font-size: 13px;
  }
}
</style>
