<template>
  <section class="schema-viewer">
    <header>
      <div>
        <h3>Configuration Schema</h3>
        <span class="version">Version {{ schema.version }}</span>
      </div>
      <span class="read-only-badge">Read-only</span>
    </header>

    <div v-if="entries.length" class="properties">
      <SchemaProperty
        v-for="[name, property] in entries"
        :key="name"
        :name="name"
        :property="property"
        :required="required.has(name)"
      />
    </div>
    <p v-else>No configuration properties are published.</p>

    <div v-if="schema.examples?.length" class="examples">
      <h4>Examples</h4>
      <pre v-for="(example, index) in schema.examples" :key="index">{{ JSON.stringify(example, null, 2) }}</pre>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, type PropType } from 'vue'
import type { PluginSchema, PluginSchemaProperty } from '@/api/plugin'

const props = defineProps<{ schema: PluginSchema }>()
const required = computed(() => new Set(props.schema.required ?? []))
const entries = computed(() => Object.entries(props.schema.properties))

const SchemaProperty = defineComponent({
  name: 'SchemaProperty',
  props: {
    name: { type: String, required: true },
    property: { type: Object as PropType<PluginSchemaProperty>, required: true },
    required: { type: Boolean, default: false },
  },
  setup(componentProps) {
    const renderProperty = (name: string, property: PluginSchemaProperty, isRequired = false) => {
      const constraints: string[] = []
      if (Object.prototype.hasOwnProperty.call(property, 'default')) {
        constraints.push(`Default: ${JSON.stringify(property.default)}`)
      }
      if (property.enum) constraints.push(`Values: ${property.enum.map(value => JSON.stringify(value)).join(', ')}`)
      if (property.pattern) constraints.push(`Pattern: ${property.pattern}`)
      if (property.minimum !== undefined) constraints.push(`Minimum: ${property.minimum}`)
      if (property.maximum !== undefined) constraints.push(`Maximum: ${property.maximum}`)
      if (property.sensitive) constraints.push('Sensitive')

      const children = [
        h('div', { class: 'property-heading' }, [
          h('strong', name),
          isRequired ? h('span', { class: 'required' }, ' *') : null,
          h('code', property.type),
        ]),
        h('p', property.description),
        constraints.length ? h('ul', constraints.map(value => h('li', value))) : null,
      ]
      if (property.type === 'object' && property.properties) {
        children.push(h('div', { class: 'nested' },
          Object.entries(property.properties).map(([childName, child]) => renderProperty(childName, child)),
        ))
      }
      if (property.type === 'array' && property.items) {
        children.push(h('div', { class: 'nested' }, [renderProperty('items', property.items)]))
      }
      return h('article', { class: 'property' }, children)
    }
    return () => renderProperty(componentProps.name, componentProps.property, componentProps.required)
  },
})
</script>

<style scoped>
.schema-viewer { border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; }
header { display: flex; justify-content: space-between; padding: 16px; background: #f8fafc; }
h3 { margin: 0; }
.version { color: #6b7280; font-size: .85rem; }
.read-only-badge { align-self: start; padding: 3px 8px; background: #e5e7eb; border-radius: 999px; }
.properties, .examples { padding: 16px; }
.property { border-top: 1px solid #e5e7eb; padding: 12px 0; }
.property:first-child { border-top: 0; }
.property-heading { display: flex; gap: 8px; align-items: center; }
.required { color: #b91c1c; }
.property p { margin: 5px 0; color: #4b5563; }
.nested { margin-left: 18px; padding-left: 12px; border-left: 2px solid #e5e7eb; }
pre { overflow: auto; background: #111827; color: #f9fafb; padding: 12px; }
</style>
