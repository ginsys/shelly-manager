<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="back-link" to="/templates">← Back to Templates</router-link>
      <h1 class="title">Template Examples</h1>
      <div class="spacer" />
    </div>

    <!-- Loading/Error states -->
    <div v-if="store.loading" class="card state">Loading examples...</div>
    <div v-else-if="store.error" class="card state error">{{ store.error }}</div>

    <!-- Examples Grid -->
    <div v-else class="examples-grid">
      <div v-for="example in store.examples" :key="example.name" class="example-card">
        <div class="example-header">
          <h3>{{ example.name }}</h3>
          <span class="device-type">{{ example.deviceType }}</span>
        </div>
        <p class="example-desc">{{ example.description }}</p>

        <div class="example-content">
          <label>Template Content</label>
          <pre class="code-block">{{ example.content }}</pre>
        </div>

        <div v-if="example.variables" class="example-variables">
          <label>Variables</label>
          <ul>
            <li v-for="(type, key) in example.variables" :key="key">
              <code>{{ key }}</code>: {{ type }}
            </li>
          </ul>
        </div>

        <div class="example-actions">
          <button class="btn" @click="copyToClipboard(example.content)">Copy Template</button>
          <button class="btn primary" @click="useTemplate(example)">Use This Template</button>
        </div>
      </div>

      <div v-if="store.examples.length === 0" class="card state">
        No example templates available
      </div>
    </div>

    <!-- Use Template Dialog -->
    <div v-if="showUseDialog && selectedExample" class="modal-overlay" @click.self="showUseDialog = false">
      <div class="modal">
        <h2>Create Template from Example</h2>
        <div class="form-group">
          <label>Name *</label>
          <input v-model="formData.name" type="text" class="form-input" placeholder="Enter template name" />
        </div>
        <div class="form-group">
          <label>Device Type *</label>
          <select v-model="formData.deviceType" class="form-input">
            <option :value="selectedExample.deviceType">{{ selectedExample.deviceType }} (recommended)</option>
            <option value="shelly1">Shelly 1</option>
            <option value="shelly1pm">Shelly 1PM</option>
            <option value="shelly25">Shelly 2.5</option>
            <option value="shellyplug">Shelly Plug</option>
            <option value="shellyem">Shelly EM</option>
            <option value="shelly3em">Shelly 3EM</option>
          </select>
        </div>
        <div class="form-group">
          <label>Description</label>
          <textarea v-model="formData.description" class="form-input" rows="2" />
        </div>
        <div class="form-group">
          <label>Template Content *</label>
          <textarea v-model="formData.templateContent" class="form-input monospace" rows="12" spellcheck="false" />
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showUseDialog = false">Cancel</button>
          <button class="btn primary" @click="handleCreateTemplate">Create Template</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useTemplatesStore } from '@/stores/templates'
import type { TemplateExample } from '@/api/templates'

const router = useRouter()
const store = useTemplatesStore()

const showUseDialog = ref(false)
const selectedExample = ref<TemplateExample | null>(null)
const formData = ref({
  name: '',
  deviceType: '',
  description: '',
  templateContent: ''
})

async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    alert('Template copied to clipboard')
  } catch (e) {
    alert('Failed to copy to clipboard')
  }
}

function useTemplate(example: TemplateExample) {
  selectedExample.value = example
  formData.value = {
    name: `${example.name} (Copy)`,
    deviceType: example.deviceType,
    description: example.description,
    templateContent: example.content
  }
  showUseDialog.value = true
}

async function handleCreateTemplate() {
  if (!formData.value.name || !formData.value.deviceType || !formData.value.templateContent) {
    alert('Please fill in all required fields')
    return
  }

  try {
    const template = await store.create(formData.value)
    showUseDialog.value = false
    router.push(`/templates/${template.id}`)
  } catch (e: any) {
    alert('Failed to create template: ' + (e?.message || 'Unknown error'))
  }
}

onMounted(() => {
  store.fetchExamples()
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.back-link { color: #2563eb; text-decoration: none; }
.back-link:hover { text-decoration: underline; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }

.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; }
.state { padding: 32px; text-align: center; color: #64748b; }
.state.error { color: #b91c1c; }

.examples-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(400px, 1fr)); gap: 16px; }
.example-card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 16px; display: flex; flex-direction: column; gap: 12px; }
.example-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.example-header h3 { font-size: 16px; margin: 0; }
.device-type { padding: 2px 8px; background: #e0e7ff; color: #3730a3; border-radius: 999px; font-size: 12px; font-weight: 600; white-space: nowrap; }
.example-desc { margin: 0; color: #64748b; font-size: 14px; }

.example-content, .example-variables { display: flex; flex-direction: column; gap: 6px; }
.example-content label, .example-variables label { font-size: 12px; font-weight: 600; color: #64748b; text-transform: uppercase; }
.code-block { background: #1e293b; color: #e2e8f0; padding: 12px; border-radius: 4px; overflow-x: auto; font-size: 12px; margin: 0; font-family: ui-monospace, monospace; white-space: pre-wrap; word-break: break-all; }
.example-variables ul { margin: 0; padding-left: 20px; font-size: 14px; }
.example-variables code { background: #f1f5f9; padding: 2px 6px; border-radius: 3px; font-size: 13px; }

.example-actions { display: flex; gap: 8px; margin-top: auto; }
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; font-size: 14px; flex: 1; }
.btn:hover { background: #f8fafc; }
.btn.primary { background: #2563eb; color: white; border-color: #2563eb; }
.btn.primary:hover { background: #1d4ed8; }

.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; padding: 24px; border-radius: 8px; max-width: 800px; width: 90%; max-height: 90vh; overflow-y: auto; }
.modal h2 { margin-top: 0; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-weight: 500; color: #374151; margin-bottom: 6px; }
.form-input { width: 100%; padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-family: inherit; font-size: 14px; }
.monospace { font-family: ui-monospace, monospace; font-size: 13px; }
.modal-actions { display: flex; gap: 12px; justify-content: flex-end; margin-top: 20px; }
</style>
