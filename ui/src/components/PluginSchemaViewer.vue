<template>
  <div class="schema-viewer">
    <div class="schema-header">
      <div class="schema-title">
        <h3>Configuration Schema</h3>
        <span v-if="schema.title" class="schema-subtitle">{{ schema.title }}</span>
      </div>
      
      <div class="schema-actions">
        <button 
          class="action-btn example-btn"
          @click="loadExample"
          title="Load example configuration"
        >
          📝 Example
        </button>
        <button 
          class="action-btn export-btn"
          @click="exportSchema"
          title="Export schema as JSON"
        >
          📤 Export
        </button>
      </div>
    </div>

    <div v-if="schema.description" class="schema-description">
      {{ schema.description }}
    </div>

    <!-- Schema Properties -->
    <div class="schema-content">
      <div v-if="schema.properties" class="properties-section">
        <h4>Configuration Properties</h4>
        
        <div class="properties-list">
          <div 
            v-for="[fieldName, property] in Object.entries(schema.properties)" 
            :key="fieldName"
            class="property-item"
            :class="{ required: isRequired(fieldName) }"
          >
            <div class="property-header">
              <div class="property-name">
                <span class="field-name">{{ fieldName }}</span>
                <span v-if="isRequired(fieldName)" class="required-indicator">*</span>
                <span class="field-type">{{ formatPropertyType(property) }}</span>
              </div>
              
              <div class="property-metadata">
                <span v-if="property.default !== undefined" class="default-value" :title="`Default: ${formatValue(property.default)}`">
                  🎯 {{ formatValue(property.default) }}
                </span>
                <span v-if="property.enum" class="enum-indicator" :title="`Options: ${property.enum.join(', ')}`">
                  📋 {{ property.enum.length }} options
                </span>
              </div>
            </div>

            <div v-if="property.title || property.description" class="property-description">
              <div v-if="property.title" class="property-title">{{ property.title }}</div>
              <div v-if="property.description" class="property-desc">{{ property.description }}</div>
            </div>

            <!-- Property Constraints -->
            <div v-if="hasConstraints(property)" class="property-constraints">
              <div class="constraints-list">
                <span v-if="property.minLength" class="constraint">
                  Min length: {{ property.minLength }}
                </span>
                <span v-if="property.maxLength" class="constraint">
                  Max length: {{ property.maxLength }}
                </span>
                <span v-if="property.minimum !== undefined" class="constraint">
                  Min: {{ property.minimum }}
                </span>
                <span v-if="property.maximum !== undefined" class="constraint">
                  Max: {{ property.maximum }}
                </span>
                <span v-if="property.pattern" class="constraint">
                  Pattern: {{ property.pattern }}
                </span>
                <span v-if="property.format" class="constraint">
                  Format: {{ property.format }}
                </span>
              </div>
            </div>

            <!-- Enum Values -->
            <div v-if="property.enum" class="property-enum">
              <div class="enum-label">Valid options:</div>
              <div class="enum-values">
                <span 
                  v-for="option in property.enum" 
                  :key="option"
                  class="enum-value"
                >
                  {{ formatValue(option) }}
                </span>
              </div>
            </div>

            <!-- Examples -->
            <div v-if="property.examples?.length" class="property-examples">
              <div class="examples-label">Examples:</div>
              <div class="examples-list">
                <code 
                  v-for="(example, index) in property.examples.slice(0, 3)" 
                  :key="index"
                  class="example-value"
                >
                  {{ formatValue(example) }}
                </code>
                <span v-if="property.examples.length > 3" class="more-examples">
                  +{{ property.examples.length - 3 }} more
                </span>
              </div>
            </div>

            <!-- Nested Object Properties -->
            <div v-if="property.type === 'object' && property.properties" class="nested-properties">
              <div class="nested-label">Object properties:</div>
              <div class="nested-list">
                <div 
                  v-for="[nestedName, nestedProp] in Object.entries(property.properties).slice(0, 5)" 
                  :key="nestedName"
                  class="nested-property"
                >
                  <span class="nested-name">{{ nestedName }}</span>
                  <span class="nested-type">{{ formatPropertyType(nestedProp) }}</span>
                  <span v-if="property.required?.includes(nestedName)" class="nested-required">required</span>
                </div>
                <div v-if="Object.keys(property.properties).length > 5" class="nested-more">
                  +{{ Object.keys(property.properties).length - 5 }} more properties
                </div>
              </div>
            </div>

            <!-- Array Item Type -->
            <div v-if="property.type === 'array' && property.items" class="array-items">
              <div class="array-label">Array items type:</div>
              <div class="array-type">{{ formatPropertyType(property.items) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Schema Examples -->
      <div v-if="schema.examples?.length" class="examples-section">
        <h4>Complete Configuration Examples</h4>
        <div class="schema-examples">
          <div 
            v-for="(example, index) in schema.examples" 
            :key="index"
            class="schema-example"
          >
            <div class="example-header">
              <span class="example-title">Example {{ index + 1 }}</span>
              <button 
                class="example-load-btn"
                @click="loadSchemaExample(example)"
                title="Load this example"
              >
                📥 Load
              </button>
            </div>
            <pre class="example-code">{{ JSON.stringify(example, null, 2) }}</pre>
          </div>
        </div>
      </div>

      <!-- No Schema Available -->
      <div v-if="!schema.properties && !schema.examples?.length" class="no-schema">
        <div class="no-schema-icon">📄</div>
        <h4>No Schema Available</h4>
        <p>This plugin doesn't provide a configuration schema. You can still configure it using a custom JSON object.</p>
        
        <div class="manual-config-tips">
          <h5>Manual Configuration Tips:</h5>
          <ul>
            <li>Check the plugin documentation for supported configuration options</li>
            <li>Start with an empty object <code>{}</code> and add properties as needed</li>
            <li>Use the test feature to validate your configuration</li>
            <li>Look at similar plugins for configuration patterns</li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { type PluginSchema, type PluginSchemaProperty } from '@/api/plugin'

// Props
interface Props {
  schema: PluginSchema
  plugin?: any
}

const props = withDefaults(defineProps<Props>(), {
  schema: () => ({ type: 'object', properties: {} }),
  plugin: null
})

// Emits
const emit = defineEmits<{
  loadExample: [config: Record<string, any>]
  exportSchema: [schema: PluginSchema]
}>()

/**
 * Check if a field is required
 */
function isRequired(fieldName: string): boolean {
  return props.schema.required?.includes(fieldName) || false
}

/**
 * Check if property has validation constraints
 */
function hasConstraints(property: PluginSchemaProperty): boolean {
  return !!(
    property.minLength ||
    property.maxLength ||
    property.minimum !== undefined ||
    property.maximum !== undefined ||
    property.pattern ||
    property.format
  )
}

/**
 * Format property type for display
 */
function formatPropertyType(property: PluginSchemaProperty): string {
  let type = property.type
  
  if (property.type === 'array' && property.items) {
    type = `${formatPropertyType(property.items)}[]`
  }
  
  if (property.enum) {
    return `enum (${property.enum.length} options)`
  }
  
  return type
}

/**
 * Format value for display
 */
function formatValue(value: any): string {
  if (typeof value === 'string') {
    return value.length > 30 ? `"${value.substring(0, 30)}..."` : `"${value}"`
  }
  
  if (typeof value === 'object') {
    return JSON.stringify(value)
  }
  
  return String(value)
}

/**
 * Load example configuration
 */
function loadExample() {
  if (props.plugin?.example_config) {
    emit('loadExample', props.plugin.example_config)
  } else if (props.schema.examples?.length) {
    emit('loadExample', props.schema.examples[0])
  } else {
    // Generate a default example from schema
    const example = generateDefaultExample()
    emit('loadExample', example)
  }
}

/**
 * Load a specific schema example
 */
function loadSchemaExample(example: Record<string, any>) {
  emit('loadExample', example)
}

/**
 * Export schema
 */
function exportSchema() {
  emit('exportSchema', props.schema)
}

/**
 * Generate default example from schema
 */
function generateDefaultExample(): Record<string, any> {
  const example: Record<string, any> = {}
  
  if (!props.schema.properties) {
    return example
  }
  
  for (const [fieldName, property] of Object.entries(props.schema.properties)) {
    if (property.examples?.length) {
      example[fieldName] = property.examples[0]
    } else if (property.default !== undefined) {
      example[fieldName] = property.default
    } else if (property.enum?.length) {
      example[fieldName] = property.enum[0]
    } else {
      // Generate based on type
      switch (property.type) {
        case 'string':
          example[fieldName] = property.format === 'email' ? 'user@example.com' : 'example_value'
          break
        case 'number':
        case 'integer':
          example[fieldName] = property.minimum || 0
          break
        case 'boolean':
          example[fieldName] = false
          break
        case 'array':
          example[fieldName] = []
          break
        case 'object':
          example[fieldName] = {}
          break
      }
    }
  }
  
  return example
}
</script>

<style scoped>
.schema-viewer {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  overflow: hidden;
}

.schema-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 20px;
  background: #f8fafc;
  border-bottom: 1px solid #e5e7eb;
}

.schema-title h3 {
  margin: 0 0 4px 0;
  color: #1f2937;
  font-size: 1.25rem;
}

.schema-subtitle {
  color: #6b7280;
  font-size: 0.875rem;
  font-weight: 500;
}

.schema-actions {
  display: flex;
  gap: 8px;
}

.action-btn {
  background: #f3f4f6;
  border: 1px solid #d1d5db;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
}

.action-btn:hover {
  background: #e5e7eb;
}

.example-btn:hover {
  background: #ede9fe;
  border-color: #8b5cf6;
  color: #6b21a8;
}

.export-btn:hover {
  background: #dbeafe;
  border-color: #3b82f6;
  color: #1e40af;
}

.schema-description {
  padding: 16px 20px;
  background: #fefce8;
  border-bottom: 1px solid #e5e7eb;
  color: #92400e;
  font-size: 0.875rem;
  line-height: 1.5;
}

.schema-content {
  padding: 20px;
}

.properties-section h4,
.examples-section h4 {
  margin: 0 0 16px 0;
  color: #1f2937;
  font-size: 1.125rem;
  font-weight: 600;
}

.properties-list {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.property-item {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 16px;
  background: #ffffff;
}

.property-item.required {
  border-left: 4px solid #f59e0b;
}

.property-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 8px;
}

.property-name {
  display: flex;
  align-items: center;
  gap: 6px;
}

.field-name {
  font-weight: 600;
  color: #1f2937;
  font-family: monospace;
  background: #f3f4f6;
  padding: 2px 6px;
  border-radius: 3px;
}

.required-indicator {
  color: #f59e0b;
  font-weight: 600;
}

.field-type {
  background: #e0e7ff;
  color: #3730a3;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.property-metadata {
  display: flex;
  gap: 8px;
  align-items: center;
}

.default-value,
.enum-indicator {
  background: #f0fdf4;
  color: #166534;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: help;
}

.enum-indicator {
  background: #fef3c7;
  color: #92400e;
}

.property-description {
  margin-bottom: 12px;
}

.property-title {
  font-weight: 600;
  color: #374151;
  margin-bottom: 4px;
  font-size: 0.875rem;
}

.property-desc {
  color: #6b7280;
  font-size: 0.875rem;
  line-height: 1.5;
}

.property-constraints {
  margin-bottom: 12px;
}

.constraints-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.constraint {
  background: #fef2f2;
  color: #991b1b;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.property-enum {
  margin-bottom: 12px;
}

.enum-label,
.examples-label,
.nested-label,
.array-label {
  font-size: 0.75rem;
  font-weight: 600;
  color: #4b5563;
  margin-bottom: 6px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.enum-values,
.examples-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.enum-value,
.example-value {
  background: #f3f4f6;
  color: #374151;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-family: monospace;
  border: 1px solid #d1d5db;
}

.more-examples {
  color: #6b7280;
  font-size: 0.75rem;
  font-style: italic;
  align-self: center;
}

.nested-properties {
  margin-bottom: 12px;
}

.nested-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.nested-property {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px;
  background: #f9fafb;
  border-radius: 4px;
  font-size: 0.75rem;
}

.nested-name {
  font-family: monospace;
  font-weight: 500;
  color: #1f2937;
}

.nested-type {
  background: #e0e7ff;
  color: #3730a3;
  padding: 1px 4px;
  border-radius: 2px;
  font-size: 0.6875rem;
}

.nested-required {
  background: #fef3c7;
  color: #92400e;
  padding: 1px 4px;
  border-radius: 2px;
  font-size: 0.6875rem;
  font-weight: 500;
}

.nested-more {
  color: #6b7280;
  font-style: italic;
  padding: 6px;
  text-align: center;
}

.array-items {
  display: flex;
  align-items: center;
  gap: 8px;
}

.array-type {
  background: #e0e7ff;
  color: #3730a3;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.75rem;
  font-weight: 500;
}

.examples-section {
  margin-top: 32px;
  padding-top: 24px;
  border-top: 1px solid #e5e7eb;
}

.schema-examples {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.schema-example {
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  overflow: hidden;
}

.example-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #f8fafc;
  border-bottom: 1px solid #e5e7eb;
}

.example-title {
  font-weight: 600;
  color: #374151;
  font-size: 0.875rem;
}

.example-load-btn {
  background: #10b981;
  color: white;
  border: none;
  padding: 4px 8px;
  border-radius: 3px;
  font-size: 0.75rem;
  cursor: pointer;
  transition: background-color 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
}

.example-load-btn:hover {
  background: #059669;
}

.example-code {
  padding: 16px;
  background: #1f2937;
  color: #f9fafb;
  font-family: monospace;
  font-size: 0.75rem;
  line-height: 1.5;
  margin: 0;
  overflow-x: auto;
}

.no-schema {
  text-align: center;
  padding: 40px 20px;
  color: #6b7280;
}

.no-schema-icon {
  font-size: 3rem;
  margin-bottom: 16px;
}

.no-schema h4 {
  color: #374151;
  margin: 0 0 8px 0;
}

.no-schema p {
  margin: 0 0 24px 0;
}

.manual-config-tips {
  text-align: left;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 16px;
  margin-top: 20px;
}

.manual-config-tips h5 {
  margin: 0 0 12px 0;
  color: #374151;
  font-size: 0.875rem;
}

.manual-config-tips ul {
  margin: 0;
  padding-left: 20px;
  color: #4b5563;
  font-size: 0.875rem;
}

.manual-config-tips li {
  margin-bottom: 6px;
  line-height: 1.5;
}

.manual-config-tips code {
  background: #f3f4f6;
  padding: 1px 4px;
  border-radius: 2px;
  font-size: 0.8125rem;
}

/* Responsive design */
@media (max-width: 768px) {
  .schema-header {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .schema-actions {
    justify-content: flex-start;
  }

  .property-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .property-name {
    flex-wrap: wrap;
  }

  .example-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .enum-values,
  .examples-list {
    flex-direction: column;
  }

  .nested-property {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
}
</style>