<template>
  <main style="padding:16px">
    <div style="margin-bottom:16px">
      <router-link to="/notifications/channels" class="back-link">← Back to Channels</router-link>
    </div>

    <h1>Channel Details</h1>

    <ErrorDisplay
      :error="actionError"
      :show-retry="false"
      data-testid="channel-action-error"
      @dismiss="clearActionError"
    />

    <div v-if="store.channelsLoading">Loading...</div>
    <div v-else-if="store.channelsError" class="error">{{ store.channelsError }}</div>
    <div v-else-if="store.currentChannel" class="details">
      <div class="detail-row">
        <span class="label">ID:</span>
        <span>{{ store.currentChannel.id }}</span>
      </div>
      <div class="detail-row">
        <span class="label">Name:</span>
        <span>{{ store.currentChannel.name }}</span>
      </div>
      <div class="detail-row">
        <span class="label">Type:</span>
        <span class="badge">{{ store.currentChannel.type }}</span>
      </div>
      <div class="detail-row">
        <span class="label">Enabled:</span>
        <span>{{ store.currentChannel.enabled ? 'Yes' : 'No' }}</span>
      </div>
      <div class="detail-row">
        <span class="label">Created:</span>
        <span>{{ new Date(store.currentChannel.createdAt).toLocaleString() }}</span>
      </div>
      <div class="detail-row">
        <span class="label">Updated:</span>
        <span>{{ new Date(store.currentChannel.updatedAt).toLocaleString() }}</span>
      </div>
      <div class="detail-row">
        <span class="label">Config:</span>
        <form v-if="edit" class="edit-form" @submit.prevent="save">
          <p v-if="saveError" class="form-error" role="alert" data-testid="channel-save-error">{{ saveError }}</p>
          <ChannelConfigFields
            :type="store.currentChannel.type"
            :fields="edit.fields"
            :stored="edit.stored"
            :keep="edit.keep"
          />
          <div style="display:flex;gap:8px;margin-top:16px">
            <button type="submit" class="primary-button" :disabled="saving" data-testid="channel-save">
              {{ saving ? 'Saving...' : 'Save' }}
            </button>
            <button type="button" class="secondary-button" :disabled="saving" data-testid="channel-edit-cancel" @click="cancelEdit">
              Cancel
            </button>
          </div>
        </form>
        <pre v-else style="background:#f3f4f6;padding:8px;border-radius:4px">{{ JSON.stringify(store.currentChannel.config, null, 2) }}</pre>
      </div>

      <div style="margin-top:24px;display:flex;gap:8px;align-items:center">
        <!-- A toggle that ran during a save would read the old config and PUT it back after the save. -->
        <button class="primary-button" :disabled="saving" data-testid="channel-toggle" @click="toggleEnabled">
          {{ store.currentChannel.enabled ? 'Disable' : 'Enable' }}
        </button>
        <button class="primary-button" :disabled="testing" data-testid="channel-test-button" @click="sendTest">
          {{ testing ? 'Sending...' : 'Send Test' }}
        </button>
        <button v-if="!edit" class="primary-button" data-testid="channel-edit" @click="startEdit">Edit</button>
        <span v-if="testResult" class="test-result" role="status" data-testid="channel-test-result">{{ testResult }}</span>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useNotificationsStore } from '@/stores/notifications'
import { testChannel } from '@/api/notification'
import { buildEditConfig, editFromConfig, type ChannelEdit } from '@/utils/channelConfig'
import ChannelConfigFields from '@/components/ChannelConfigFields.vue'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import { useLateFailure, usePageError } from '@/composables/useNotice'

const route = useRoute()
const store = useNotificationsStore()
const reportLate = useLateFailure()
// The failure of an update, above the details. The store keeps no action failure (ginsys/shelly-manager#701).
const { error: actionError, setError: setActionError, clearError: clearActionError } = usePageError()

onMounted(() => {
  const id = route.params.id as string
  if (id) {
    store.fetchChannel(id)
  }
})

async function toggleEnabled() {
  if (!store.currentChannel) return
  // The channel can be deleted while the update is pending, so the id is read before the call.
  const { id, enabled } = store.currentChannel
  clearActionError()
  try {
    await store.setChannelEnabled(id, !enabled)
  } catch (e) {
    setActionError(e, { action: 'Update channel', resource: 'Notification channel', resourceId: id })
  }
}

// The edit form (ginsys/shelly-manager#783). Its state is derived from the channel as read, every time it
// opens, so what was typed before a Cancel never comes back. The type is fixed: a change of type is not an edit.
const edit = ref<ChannelEdit | null>(null)
const saveError = ref('')
const saving = ref(false)

function startEdit() {
  if (!store.currentChannel) return
  saveError.value = ''
  edit.value = editFromConfig(store.currentChannel.type, store.currentChannel.config)
}

function cancelEdit() {
  edit.value = null
  saveError.value = ''
}

async function save() {
  const channel = store.currentChannel
  if (!channel || !edit.value || saving.value) return
  saveError.value = ''
  const built = buildEditConfig(channel.type, channel.config, edit.value.fields, edit.value.keep)
  if (built.error !== undefined) {
    saveError.value = built.error
    return
  }
  saving.value = true
  try {
    await store.saveChannel(channel.id, { ...channel, config: built.config })
    cancelEdit()
  } catch (e) {
    // The form stays open with what was typed. After the user left the page, the app notice says it.
    saveError.value = (e as Error)?.message || 'Failed to update channel'
    reportLate(e, { action: 'Update channel', resource: 'Notification channel', resourceId: channel.id })
  } finally {
    saving.value = false
  }
}

// Sends one synthetic notification through the channel. A failure goes to the page's error panel; a
// success is a line beside the button.
const testing = ref(false)
const testResult = ref('')

async function sendTest() {
  if (!store.currentChannel || testing.value) return
  const { id } = store.currentChannel
  clearActionError()
  testResult.value = ''
  testing.value = true
  try {
    testResult.value = await testChannel(id)
  } catch (e) {
    setActionError(e, { action: 'Send test notification', resource: 'Notification channel', resourceId: id })
  } finally {
    testing.value = false
  }
}
</script>

<style scoped>
.back-link {
  color: #2563eb;
  text-decoration: none;
}
.back-link:hover {
  text-decoration: underline;
}
.error {
  color: #dc2626;
  padding: 12px;
  background: #fee2e2;
  border-radius: 6px;
}
.details {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 24px;
}
.detail-row {
  display: flex;
  padding: 12px 0;
  border-bottom: 1px solid #f3f4f6;
}
.detail-row:last-child {
  border-bottom: none;
}
.label {
  font-weight: 600;
  width: 150px;
}
.badge {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  background: #e5e7eb;
}
.primary-button {
  padding: 8px 16px;
  background: #2563eb;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
.primary-button:hover:not(:disabled) {
  background: #1d4ed8;
}
.primary-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.test-result {
  color: #065f46;
}
.edit-form {
  flex: 1;
}
.form-error {
  margin: 0 0 12px;
  padding: 8px 10px;
  color: #b91c1c;
  background: #fee2e2;
  border-radius: 4px;
  overflow-wrap: anywhere;
}
.secondary-button {
  padding: 8px 16px;
  background: #e5e7eb;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
.secondary-button:hover:not(:disabled) {
  background: #d1d5db;
}
.secondary-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
