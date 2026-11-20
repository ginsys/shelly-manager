<template>
  <main style="padding:16px">
    <h1>Notification Channels</h1>
    <p style="color:#6b7280;margin-bottom:16px">
      Manage notification channels for email, webhooks, and Slack
    </p>

    <div style="margin-bottom:16px">
      <button class="primary-button" @click="openCreateForm">
        ➕ Create Channel
      </button>
    </div>

    <ErrorDisplay
      :error="actionError"
      :show-retry="false"
      data-testid="channel-action-error"
      @dismiss="clearActionError"
    />

    <DataTable
      :rows="store.channels"
      :loading="store.channelsLoading"
      :error="store.channelsError"
      :cols="6"
      :rowKey="row => row.id"
    >
      <template #header>
        <th>Name</th>
        <th>Type</th>
        <th>Enabled</th>
        <th>Created</th>
        <th>Updated</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>{{ row.name }}</td>
        <td><span class="badge">{{ row.type }}</span></td>
        <td>{{ row.enabled ? '✓ Yes' : '✗ No' }}</td>
        <td>{{ new Date(row.createdAt).toLocaleDateString() }}</td>
        <td>{{ new Date(row.updatedAt).toLocaleDateString() }}</td>
        <td>
          <router-link :to="`/notifications/channels/${row.id}`" class="link-button">
            Edit
          </router-link>
          <button
            class="link-button text-red-600"
            @click="handleDelete(row.id)"
            style="margin-left:8px"
          >
            Delete
          </button>
        </td>
      </template>
    </DataTable>

    <!-- Simple create form modal -->
    <div v-if="showCreateForm" class="modal-overlay" @click="closeCreateForm">
      <div class="modal" @click.stop>
        <h2>Create Notification Channel</h2>
        <p v-if="createError" class="form-error" role="alert" data-testid="channel-create-error">{{ createError }}</p>
        <form @submit.prevent="handleCreate">
          <div class="form-group">
            <label>Name:</label>
            <input v-model="newChannel.name" required class="form-input" data-testid="channel-name" />
          </div>
          <div class="form-group">
            <label>Type:</label>
            <select v-model="newChannel.type" required class="form-input" data-testid="channel-type">
              <option value="email">Email</option>
              <option value="webhook">Webhook</option>
              <option value="slack">Slack</option>
            </select>
          </div>
          <!-- The config of the chosen type. The server decides what is valid and says why when it refuses. -->
          <template v-if="newChannel.type === 'email'">
            <div class="form-group">
              <label>Recipients:</label>
              <textarea
                v-model="fields.recipients"
                required
                rows="3"
                class="form-input"
                placeholder="One address per line, or separated by commas"
                data-testid="channel-recipients"
              />
            </div>
            <div class="form-group">
              <label>Subject (optional):</label>
              <input v-model="fields.subject" class="form-input" data-testid="channel-subject" />
            </div>
          </template>
          <template v-else-if="newChannel.type === 'webhook'">
            <div class="form-group">
              <label>URL:</label>
              <input
                v-model="fields.webhookUrl"
                required
                autocomplete="off"
                class="form-input"
                placeholder="https://"
                data-testid="channel-webhook-url"
              />
            </div>
            <div class="form-group">
              <label>Method:</label>
              <select v-model="fields.method" class="form-input" data-testid="channel-webhook-method">
                <option v-for="m in WEBHOOK_METHODS" :key="m" :value="m">{{ m }}</option>
              </select>
            </div>
            <div class="form-group">
              <label>Headers (optional):</label>
              <div v-for="(row, i) in fields.headers" :key="i" class="header-row">
                <input v-model="row.key" placeholder="Name" class="form-input" data-testid="channel-header-name" />
                <input v-model="row.value" placeholder="Value" autocomplete="off" class="form-input" data-testid="channel-header-value" />
                <button
                  type="button"
                  class="secondary-button"
                  data-testid="channel-remove-header"
                  @click="removeHeader(i)"
                >
                  Remove
                </button>
              </div>
              <button type="button" class="secondary-button" data-testid="channel-add-header" @click="addHeader">
                Add header
              </button>
            </div>
            <div class="form-group">
              <label>Secret (optional, signs the payload):</label>
              <input
                v-model="fields.secret"
                type="password"
                autocomplete="off"
                class="form-input"
                data-testid="channel-webhook-secret"
              />
            </div>
          </template>
          <template v-else>
            <div class="form-group">
              <label>Webhook URL:</label>
              <input
                v-model="fields.slackUrl"
                required
                autocomplete="off"
                class="form-input"
                placeholder="https://hooks.slack.com/services/..."
                data-testid="channel-slack-url"
              />
            </div>
            <div class="form-group">
              <label>Channel (optional):</label>
              <input v-model="fields.slackChannel" class="form-input" placeholder="#alerts" data-testid="channel-slack-channel" />
            </div>
            <div class="form-group">
              <label>Username (optional):</label>
              <input v-model="fields.slackUsername" class="form-input" data-testid="channel-slack-username" />
            </div>
          </template>
          <div class="form-group">
            <label>Enabled:</label>
            <input type="checkbox" v-model="newChannel.enabled" />
          </div>
          <div style="display:flex;gap:8px;margin-top:16px">
            <button type="submit" class="primary-button">Create</button>
            <button type="button" class="secondary-button" data-testid="channel-cancel" @click="closeCreateForm">
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useNotificationsStore } from '@/stores/notifications'
import { buildChannelConfig, emptyChannelFields, WEBHOOK_METHODS, type ChannelType } from '@/utils/channelConfig'
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
const newChannel = ref({
  name: '',
  type: 'email' as ChannelType,
  enabled: true,
  config: {}
})
// What the user typed for the chosen type. A change of type starts it again, so a field of the type left
// never travels with the new one.
const fields = ref(emptyChannelFields())
watch(
  () => newChannel.value.type,
  () => {
    fields.value = emptyChannelFields()
  }
)

function addHeader() {
  fields.value.headers.push({ key: '', value: '' })
}

function removeHeader(index: number) {
  fields.value.headers.splice(index, 1)
}

function resetForm() {
  newChannel.value = { name: '', type: 'email', enabled: true, config: {} }
  fields.value = emptyChannelFields()
}

// Cancel and a click on the backdrop end the form: what was typed, credentials included, goes with it.
function closeCreateForm() {
  showCreateForm.value = false
  resetForm()
}

onMounted(() => {
  store.fetchChannels()
})

// The number of the form that is open now. A submission keeps the number it started with and touches
// the form only if the number is still the same: the answer of a cancelled form belongs to nobody.
let formSession = 0

function openCreateForm() {
  formSession++
  createError.value = ''
  resetForm()
  showCreateForm.value = true
}

async function handleCreate() {
  const session = formSession
  createError.value = ''
  const built = buildChannelConfig(newChannel.value.type, fields.value)
  if (built.error !== undefined) {
    createError.value = built.error
    return
  }
  try {
    await store.addChannel({ ...newChannel.value, config: built.config })
    if (session !== formSession) return
    showCreateForm.value = false
    resetForm()
  } catch (e) {
    const context = { action: 'Create channel', resource: 'Notification channel' }
    if (session === formSession && showCreateForm.value) {
      // The form stays open and shows the failure. After the user left the page, the app notice does.
      createError.value = (e as Error)?.message || 'Failed to create channel'
      reportLate(e, context)
    } else {
      // The form it was sent from is gone, so the page panel says it (the notice, if a newer form covers it).
      setActionError(e, context)
    }
  }
}

async function handleDelete(id: string) {
  if (!confirm('Are you sure you want to delete this channel?')) return
  clearActionError()
  try {
    await store.removeChannel(id)
  } catch (e) {
    setActionError(e, { action: 'Delete channel', resource: 'Notification channel', resourceId: id })
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
.header-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
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
.link-button:hover {
  color: #1d4ed8;
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
.text-red-600 {
  color: #dc2626;
}
</style>
