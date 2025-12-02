<template>
  <div class="validation-results">
    <div v-if="result.valid" class="result-section success">
      <div class="result-icon">✓</div>
      <div class="result-content">
        <h3>Configuration is valid</h3>
        <p>No errors found in the configuration.</p>
      </div>
    </div>

    <div v-else class="result-section error">
      <div class="result-icon">✗</div>
      <div class="result-content">
        <h3>Configuration has errors</h3>
        <p>Please fix the following errors before saving:</p>
      </div>
    </div>

    <div v-if="result.errors && result.errors.length > 0" class="errors-list">
      <h4>Errors</h4>
      <ul>
        <li v-for="(error, index) in result.errors" :key="index" class="error-item">
          <strong>{{ error.field }}</strong>: {{ error.message }}
        </li>
      </ul>
    </div>

    <div v-if="result.warnings && result.warnings.length > 0" class="warnings-list">
      <h4>Warnings</h4>
      <ul>
        <li v-for="(warning, index) in result.warnings" :key="index" class="warning-item">
          <strong>{{ warning.field }}</strong>: {{ warning.message }}
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ValidationResult } from '@/api/typedConfig'

interface Props {
  result: ValidationResult
}

defineProps<Props>()
</script>

<style scoped>
.validation-results {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}

.result-section {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  border-radius: 6px;
}

.result-section.success {
  background: #dcfce7;
  border: 1px solid #86efac;
}

.result-section.error {
  background: #fee2e2;
  border: 1px solid #fca5a5;
}

.result-icon {
  font-size: 24px;
  font-weight: bold;
  flex-shrink: 0;
}

.result-section.success .result-icon { color: #16a34a; }
.result-section.error .result-icon { color: #dc2626; }

.result-content { flex: 1; }
.result-content h3 {
  margin: 0 0 4px 0;
  font-size: 16px;
}

.result-content p {
  margin: 0;
  font-size: 14px;
  color: #64748b;
}

.errors-list, .warnings-list {
  padding: 12px;
  border-radius: 6px;
}

.errors-list {
  background: #fee2e2;
  border: 1px solid #fca5a5;
}

.warnings-list {
  background: #fef3c7;
  border: 1px solid #fcd34d;
}

.errors-list h4, .warnings-list h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
  font-weight: 600;
}

.errors-list h4 { color: #991b1b; }
.warnings-list h4 { color: #92400e; }

.errors-list ul, .warnings-list ul {
  margin: 0;
  padding-left: 20px;
  font-size: 14px;
}

.error-item { color: #991b1b; margin-bottom: 4px; }
.warning-item { color: #92400e; margin-bottom: 4px; }
.error-item strong, .warning-item strong { font-weight: 600; }
</style>
