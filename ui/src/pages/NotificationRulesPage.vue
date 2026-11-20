<template>
  <main style="padding:16px">
    <h1>Notification Rules</h1>
    <p style="color:#6b7280;margin-bottom:16px">
      Define rules to trigger notifications based on events
    </p>

    <div style="margin-bottom:16px">
      <button class="primary-button" @click="openCreateForm">
        ➕ Create Rule
      </button>
    </div>

    <ErrorDisplay
      :error="actionError"
      :show-retry="false"
      data-testid="rule-action-error"
      @dismiss="clearActionError"
    />

    <DataTable
      :rows="store.rules"
      :loading="store.rulesLoading"
      :error="store.rulesError"
      :cols="7"
      :rowKey="row => row.id"
    >
      <template #header>
        <th>Name</th>
        <th>Channel</th>
        <th>Alert Level</th>
        <th>Min Severity</th>
        <th>Categories</th>
        <th>Enabled</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>{{ row.name }}</td>
        <td>{{ getChannelName(row.channelId) }}</td>
        <td><span class="badge">{{ row.alertLevel || '-' }}</span></td>
        <td>{{ row.minSeverity || '-' }}</td>
        <td>{{ row.categories.length ? row.categories.join(', ') : 'all' }}</td>
        <td>{{ row.enabled ? '✓ Yes' : '✗ No' }}</td>
        <td>
          <button class="link-button text-red-600" @click="handleDelete(row.id)">
            Delete
          </button>
        </td>
      </template>
    </DataTable>

    <!-- Create form modal -->
    <div v-if="showCreateForm" class="modal-overlay" @click="showCreateForm = false">
      <div class="modal" @click.stop>
        <h2>Create Notification Rule</h2>
        <p v-if="createError" class="form-error" role="alert" data-testid="rule-create-error">{{ createError }}</p>
        <form @submit.prevent="handleCreate">
          <div class="form-group">
            <label>Name:</label>
            <input v-model="newRule.name" required class="form-input" />
          </div>
          <div class="form-group">
            <label>Channel:</label>
            <select v-model="newRule.channelId" required class="form-input">
              <option value="">-- Select Channel --</option>
              <option v-for="channel in store.channels" :key="channel.id" :value="channel.id">
                {{ channel.name }}
              </option>
            </select>
          </div>
          <div class="form-group">
            <label>Alert Level:</label>
            <select v-model="newRule.alertLevel" class="form-input">
              <option value="all">All</option>
              <option value="critical">Critical</option>
              <option value="warning">Warning</option>
              <option value="info">Info</option>
            </select>
          </div>
          <div class="form-group">
            <label>Min Severity:</label>
            <select v-model="newRule.minSeverity" class="form-input">
              <option value="">None</option>
              <option value="info">Info</option>
              <option value="warning">Warning</option>
              <option value="critical">Critical</option>
            </select>
          </div>
          <div class="form-group">
            <label>Categories (comma-separated, empty for all):</label>
            <input v-model="categoriesInput" placeholder="security,device" class="form-input" />
          </div>
          <div class="form-group">
            <label>Enabled:</label>
            <input type="checkbox" v-model="newRule.enabled" />
          </div>
          <div style="display:flex;gap:8px;margin-top:16px">
            <button type="submit" class="primary-button">Create</button>
            <button type="button" class="secondary-button" @click="showCreateForm = false">
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useNotificationsStore } from '@/stores/notifications'
import DataTable from '@/components/DataTable.vue'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import { useLateFailure, usePageError } from '@/composables/useNotice'

const store = useNotificationsStore()
const reportLate = useLateFailure()
const showCreateForm = ref(false)
// The failure of a delete, above the table. The store keeps no action failure, so this panel is the only
// place that holds it (ginsys/shelly-manager#701); the create form covers it, so it goes to the notice then.
const { error: actionError, setError: setActionError, clearError: clearActionError } = usePageError(
  () => showCreateForm.value
)
// The failure of this form's own submission.
const createError = ref('')
const categoriesInput = ref('')
const blankRule = () => ({ name: '', channelId: '', alertLevel: 'all', minSeverity: '', enabled: true })
const newRule = ref(blankRule())

onMounted(() => {
  store.fetchRules()
  store.fetchChannels()
})

const getChannelName = computed(() => (channelId: string) => {
  return store.channelById(channelId)?.name || 'Unknown'
})

// The number of the form that is open now. A submission keeps the number it started with and touches
// the form only if the number is still the same: the answer of a cancelled form belongs to nobody.
let formSession = 0

function openCreateForm() {
  formSession++
  createError.value = ''
  showCreateForm.value = true
}

async function handleCreate() {
  const session = formSession
  createError.value = ''
  try {
    const categories = categoriesInput.value.split(',').map(s => s.trim()).filter(Boolean)
    await store.addRule({ ...newRule.value, categories })
    if (session !== formSession) return
    showCreateForm.value = false
    newRule.value = blankRule()
    categoriesInput.value = ''
  } catch (e) {
    const context = { action: 'Create rule', resource: 'Notification rule' }
    if (session === formSession && showCreateForm.value) {
      // The form stays open and shows the failure. After the user left the page, the app notice does.
      createError.value = (e as Error)?.message || 'Failed to create rule'
      reportLate(e, context)
    } else {
      // The form it was sent from is gone, so the page panel says it (the notice, if a newer form covers it).
      setActionError(e, context)
    }
  }
}

async function handleDelete(id: string) {
  if (!confirm('Are you sure you want to delete this rule?')) return
  clearActionError()
  try {
    await store.removeRule(id)
  } catch (e) {
    setActionError(e, { action: 'Delete rule', resource: 'Notification rule', resourceId: id })
  }
}
</script>

<style scoped>
.form-error {
  margin: 0 0 12px;
  padding: 8px 10px;
  color: #b91c1c;
  background: #fee2e2;
  border-radius: 4px;
  overflow-wrap: anywhere;
}
.badge {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  background: #e5e7eb;
}
.link-button {
  color: #2563eb;
  cursor: pointer;
  background: none;
  border: none;
  text-decoration: underline;
}
.text-red-600 {
  color: #dc2626;
}
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}
.modal {
  background: white;
  padding: 24px;
  border-radius: 8px;
  max-width: 500px;
  width: 90%;
  max-height: 90vh;
  overflow-y: auto;
}
.form-group {
  margin-bottom: 12px;
}
.form-group label {
  display: block;
  margin-bottom: 4px;
  font-weight: 500;
}
.form-input {
  width: 100%;
  padding: 8px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
}
.primary-button {
  padding: 8px 16px;
  background: #2563eb;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
.primary-button:hover {
  background: #1d4ed8;
}
.secondary-button {
  padding: 8px 16px;
  background: #e5e7eb;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
.secondary-button:hover {
  background: #d1d5db;
}
</style>
