<template>
  <div class="page">
    <div class="toolbar">
      <h1 class="title">Configuration Templates</h1>
      <div class="spacer" />
      <router-link class="btn" to="/templates/examples">Browse Examples</router-link>
      <button class="btn primary" @click="showCreateDialog = true">+ Create Template</button>
    </div>

    <!-- Filters -->
    <div class="filters">
      <select v-model="deviceType" class="select" @change="handleFilterChange">
        <option value="">All Device Types</option>
        <option value="shelly1">Shelly 1</option>
        <option value="shelly1pm">Shelly 1PM</option>
        <option value="shelly25">Shelly 2.5</option>
        <option value="shellyplug">Shelly Plug</option>
        <option value="shellyem">Shelly EM</option>
        <option value="shelly3em">Shelly 3EM</option>
      </select>
      <input
        v-model="search"
        type="text"
        class="search"
        placeholder="Search templates..."
        @input="handleSearchChange"
      />
    </div>

    <!-- Loading/Error states -->
    <div v-if="store.loading" class="card state">Loading...</div>
    <div v-else-if="store.error" class="card state error">{{ store.error }}</div>

    <!-- Templates Table -->
    <div v-else class="card">
      <table v-if="store.templates.length > 0" class="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Device Type</th>
            <th>Description</th>
            <th>Updated</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="template in store.templates" :key="template.id">
            <td>
              <router-link :to="`/templates/${template.id}`" class="link">
                {{ template.name }}
              </router-link>
            </td>
            <td><span class="device-type">{{ template.deviceType }}</span></td>
            <td class="desc">{{ template.description || '-' }}</td>
            <td>{{ formatDate(template.updatedAt) }}</td>
            <td>
              <div class="actions">
                <button class="action-btn" @click="handleEdit(template)">Edit</button>
                <button class="action-btn danger" @click="handleDelete(template.id)">Delete</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else class="state">No templates found</div>
    </div>

    <!-- Pagination -->
    <div v-if="store.meta?.pagination" class="pagination">
      <button class="btn" :disabled="store.page <= 1" @click="prevPage">Prev</button>
      <span>Page {{ store.page }} / {{ store.meta.pagination.total_pages || 1 }}</span>
      <button class="btn" :disabled="!store.meta.pagination.has_next" @click="nextPage">Next</button>
    </div>

    <!-- Create Dialog (simple inline form) -->
    <div v-if="showCreateDialog" class="modal-overlay" @click.self="showCreateDialog = false">
      <div class="modal">
        <h2>{{ editingTemplate ? 'Edit Template' : 'Create Template' }}</h2>
        <div class="form-group">
          <label>Name *</label>
          <input v-model="formData.name" type="text" class="form-input" required />
        </div>
        <div class="form-group">
          <label>Device Type *</label>
          <select v-model="formData.deviceType" class="form-input" required>
            <option value="">Select...</option>
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
          <textarea v-model="formData.templateContent" class="template-area" rows="10" spellcheck="false" />
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
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useTemplatesStore } from '@/stores/templates'
import type { ConfigTemplate } from '@/api/templates'

const router = useRouter()
const store = useTemplatesStore()

const deviceType = ref('')
const search = ref('')
const showCreateDialog = ref(false)
const editingTemplate = ref<ConfigTemplate | null>(null)
const formData = ref({
  name: '',
  deviceType: '',
  description: '',
  templateContent: ''
})

function formatDate(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function handleFilterChange() {
  store.setDeviceTypeFilter(deviceType.value || undefined)
  store.fetchTemplates()
}

let searchTimeout: ReturnType<typeof setTimeout> | null = null
function handleSearchChange() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    store.setSearchFilter(search.value || undefined)
    store.fetchTemplates()
  }, 300)
}

function handleEdit(template: ConfigTemplate) {
  editingTemplate.value = template
  formData.value = {
    name: template.name,
    deviceType: template.deviceType,
    description: template.description || '',
    templateContent: template.templateContent
  }
  showCreateDialog.value = true
}

async function handleDelete(id: number) {
  if (!confirm('Are you sure you want to delete this template?')) return
  try {
    await store.remove(id)
  } catch (e: any) {
    alert('Failed to delete: ' + (e?.message || 'Unknown error'))
  }
}

async function handleSave() {
  try {
    if (editingTemplate.value) {
      await store.update(editingTemplate.value.id, formData.value)
    } else {
      await store.create(formData.value)
    }
    closeDialog()
  } catch (e: any) {
    alert('Failed to save: ' + (e?.message || 'Unknown error'))
  }
}

function closeDialog() {
  showCreateDialog.value = false
  editingTemplate.value = null
  formData.value = {
    name: '',
    deviceType: '',
    description: '',
    templateContent: ''
  }
}

function prevPage() {
  if (store.page > 1) {
    store.setPage(store.page - 1)
    store.fetchTemplates()
  }
}

function nextPage() {
  if (store.meta?.pagination?.has_next) {
    store.setPage(store.page + 1)
    store.fetchTemplates()
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
.btn { padding: 6px 12px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.btn:hover { background: #f8fafc; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn.primary { background: #2563eb; color: white; border-color: #2563eb; }
.btn.primary:hover { background: #1d4ed8; }

.filters { display: flex; gap: 8px; }
.select { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px; min-width: 180px; }
.search { padding: 6px 8px; border: 1px solid #cbd5e1; border-radius: 6px; flex: 1; max-width: 300px; }

.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; }
.state { padding: 32px; text-align: center; color: #64748b; }
.state.error { color: #b91c1c; }

.table { width: 100%; border-collapse: collapse; }
.table th, .table td { text-align: left; padding: 10px 12px; border-bottom: 1px solid #f1f5f9; }
.table th { background: #f8fafc; font-weight: 600; }
.link { color: #2563eb; text-decoration: none; }
.link:hover { text-decoration: underline; }
.device-type { padding: 2px 8px; background: #e0e7ff; color: #3730a3; border-radius: 999px; font-size: 12px; font-weight: 600; }
.desc { color: #64748b; max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.actions { display: flex; gap: 4px; }
.action-btn { padding: 4px 8px; font-size: 12px; background: #e5e7eb; border: none; border-radius: 4px; cursor: pointer; }
.action-btn:hover { background: #cbd5e1; }
.action-btn.danger { background: #fee2e2; color: #991b1b; }

.pagination { display: flex; align-items: center; gap: 8px; justify-content: center; padding: 8px; }

.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; padding: 24px; border-radius: 8px; max-width: 800px; width: 90%; max-height: 90vh; overflow-y: auto; }
.modal h2 { margin-top: 0; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-weight: 500; color: #374151; margin-bottom: 6px; }
.form-input { width: 100%; padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 6px; font-family: inherit; font-size: 14px; }
.template-area { font-family: ui-monospace, monospace; font-size: 13px; resize: vertical; }
.modal-actions { display: flex; gap: 12px; justify-content: flex-end; margin-top: 20px; }
</style>
