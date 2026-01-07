<template>
  <div class="page">
    <div class="toolbar">
      <h1 class="title">Configuration Templates</h1>
      <div class="spacer" />
      <button class="btn primary" @click="openCreate">+ Create Template</button>
    </div>

    <div class="filters">
      <select v-model="store.scopeFilter" class="select" @change="store.fetchTemplates()">
        <option value="">All Scopes</option>
        <option value="global">Global</option>
        <option value="group">Group</option>
        <option value="device_type">Device Type</option>
      </select>

      <input
        v-model="store.deviceTypeFilter"
        type="text"
        class="search"
        placeholder="Filter device type (e.g. SHPLG-S)"
      />

      <input v-model="store.searchFilter" type="text" class="search" placeholder="Search by name..." />
    </div>

    <div v-if="store.loading" class="card state">Loading...</div>
    <div v-else-if="store.error" class="card state error">{{ store.error }}</div>

    <div v-else class="card">
      <table v-if="store.filteredTemplates.length > 0" class="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Scope</th>
            <th>Device Type</th>
            <th>Updated</th>
            <th>Secrets</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="template in store.filteredTemplates" :key="template.id">
            <td>
              <router-link :to="`/templates/${template.id}`" class="link">{{ template.name }}</router-link>
            </td>
            <td><span class="pill">{{ template.scope }}</span></td>
            <td><span class="pill">{{ template.device_type || '-' }}</span></td>
            <td>{{ formatDate(template.updated_at) }}</td>
            <td>
              <span v-if="hasSecrets(template)" class="pill warning">has secrets</span>
              <span v-else class="pill">none</span>
            </td>
            <td>
              <div class="actions">
                <button class="action-btn" @click="openEdit(template)">Edit</button>
                <button class="action-btn danger" @click="handleDelete(template.id)">Delete</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="state">No templates found</div>
    </div>

    <div v-if="showDialog" class="modal-overlay" @click.self="closeDialog">
      <div class="modal">
        <h2>{{ editing ? 'Edit Template' : 'Create Template' }}</h2>

        <div class="form-grid">
          <div class="form-group">
            <label>Name *</label>
            <input v-model="form.name" type="text" class="form-input" required />
          </div>

          <div class="form-group">
            <label>Scope *</label>
            <select v-model="form.scope" class="form-input">
              <option value="global">global</option>
              <option value="group">group</option>
              <option value="device_type">device_type</option>
            </select>
          </div>

          <div class="form-group" :class="{ disabled: form.scope !== 'device_type' }">
            <label>Device Type {{ form.scope === 'device_type' ? '*' : '' }}</label>
            <input
              v-model="form.device_type"
              type="text"
              class="form-input"
              :disabled="form.scope !== 'device_type'"
              placeholder="SHPLG-S"
            />
          </div>

          <div class="form-group full">
            <label>Description</label>
            <textarea v-model="form.description" class="form-input" rows="2" />
          </div>

          <div class="form-group full">
            <label>Config (JSON) *</label>
            <textarea v-model="configJson" class="template-area" rows="14" spellcheck="false" />
            <div class="hint">
              Store only the desired fields. Password fields are accepted, but returned redacted.
            </div>
          </div>
        </div>

        <div class="modal-actions">
          <button class="btn" @click="closeDialog">Cancel</button>
          <button class="btn primary" @click="handleSave">Save</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useTemplatesStore } from '@/stores/templates'
import type { ConfigTemplate, TemplateScope } from '@/api/templates'

const store = useTemplatesStore()

const showDialog = ref(false)
const editing = ref<ConfigTemplate | null>(null)

const form = ref<{ name: string; description?: string; scope: TemplateScope; device_type?: string }>({
  name: '',
  description: '',
  scope: 'global',
  device_type: '',
})

const configJson = ref<string>('{}')

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function hasSecrets(t: ConfigTemplate) {
  return Boolean(t.has_wifi_password || t.has_mqtt_password || t.has_auth_password)
}

function openCreate() {
  editing.value = null
  form.value = { name: '', description: '', scope: 'global', device_type: '' }
  configJson.value = '{\n  \n}'
  showDialog.value = true
}

function openEdit(template: ConfigTemplate) {
  editing.value = template
  form.value = {
    name: template.name,
    description: template.description || '',
    scope: (template.scope as TemplateScope) || 'global',
    device_type: template.device_type || '',
  }
  configJson.value = JSON.stringify(template.config || {}, null, 2)
  showDialog.value = true
}

function closeDialog() {
  showDialog.value = false
  editing.value = null
}

async function handleSave() {
  let parsedConfig: Record<string, any>
  try {
    parsedConfig = JSON.parse(configJson.value || '{}')
  } catch (e: any) {
    alert('Invalid config JSON: ' + (e?.message || 'Unknown error'))
    return
  }

  try {
    if (editing.value) {
      await store.update(editing.value.id, {
        name: form.value.name,
        description: form.value.description,
        config: parsedConfig,
      })
    } else {
      await store.create({
        name: form.value.name,
        description: form.value.description,
        scope: form.value.scope,
        device_type: form.value.scope === 'device_type' ? form.value.device_type : undefined,
        config: parsedConfig,
      })
    }

    closeDialog()
    await store.fetchTemplates()
  } catch (e: any) {
    alert('Failed to save template: ' + (e?.message || 'Unknown error'))
  }
}

async function handleDelete(id: number) {
  if (!confirm('Delete this template? This will fail if assigned to devices.')) return

  try {
    await store.remove(id)
  } catch (e: any) {
    alert('Failed to delete: ' + (e?.message || 'Unknown error'))
  }
}

onMounted(() => {
  store.fetchTemplates()
})
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }

.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; }
.btn.primary { background: #2563eb; border-color: #2563eb; color: #fff; }

.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 10px; padding: 12px; }
.state { color: #64748b; }
.state.error { color: #b91c1c; background: #fee2e2; border-color: #fecaca; }

.filters { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.select, .search { padding: 8px 10px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px; }
.search { min-width: 240px; }

.table { width: 100%; border-collapse: collapse; }
.table th, .table td { padding: 10px; border-bottom: 1px solid #e5e7eb; text-align: left; }
.link { color: #2563eb; text-decoration: none; }
.link:hover { text-decoration: underline; }

.pill { display: inline-flex; padding: 2px 8px; border-radius: 999px; background: #f1f5f9; color: #334155; font-size: 12px; }
.pill.warning { background: #fef3c7; color: #92400e; }

.actions { display: flex; gap: 8px; }
.action-btn { padding: 6px 10px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; }
.action-btn.danger { border-color: #fecaca; color: #b91c1c; background: #fff; }

.modal-overlay { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.35); display: flex; align-items: center; justify-content: center; padding: 16px; }
.modal { width: min(840px, 100%); background: #fff; border-radius: 12px; border: 1px solid #e5e7eb; padding: 14px; }

.form-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; }
.form-group { display: flex; flex-direction: column; gap: 6px; }
.form-group.full { grid-column: 1 / -1; }
.form-group.disabled { opacity: 0.6; }
.form-input { padding: 8px 10px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px; }
.template-area { padding: 10px; border: 1px solid #d1d5db; border-radius: 8px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; }
.hint { font-size: 12px; color: #64748b; }

.modal-actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 12px; border-top: 1px solid #e5e7eb; margin-top: 12px; }
</style>
