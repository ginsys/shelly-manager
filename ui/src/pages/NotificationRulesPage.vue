<template>
  <main style="padding:16px">
    <h1>Notification Rules</h1>
    <p style="color:#6b7280;margin-bottom:16px">
      Define rules to trigger notifications based on events
    </p>

    <div style="margin-bottom:16px">
      <button class="primary-button" @click="showCreateForm = true">
        ➕ Create Rule
      </button>
    </div>

    <DataTable
      :rows="store.rules"
      :loading="store.rulesLoading"
      :error="store.rulesError"
      :cols="5"
      :rowKey="row => row.id"
    >
      <template #header>
        <th>Name</th>
        <th>Channel</th>
        <th>Event Types</th>
        <th>Enabled</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>{{ row.name }}</td>
        <td>{{ getChannelName(row.channelId) }}</td>
        <td><span class="badge">{{ row.eventTypes.join(', ') }}</span></td>
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
            <label>Event Types (comma-separated):</label>
            <input v-model="eventTypesInput" placeholder="device.offline,export.failed" required class="form-input" />
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

const store = useNotificationsStore()
const showCreateForm = ref(false)
const eventTypesInput = ref('')
const newRule = ref({
  name: '',
  channelId: '',
  eventTypes: [] as string[],
  filters: {},
  enabled: true
})

onMounted(() => {
  store.fetchRules()
  store.fetchChannels()
})

const getChannelName = computed(() => (channelId: string) => {
  return store.channelById(channelId)?.name || 'Unknown'
})

async function handleCreate() {
  try {
    newRule.value.eventTypes = eventTypesInput.value.split(',').map(s => s.trim()).filter(Boolean)
    await store.addRule(newRule.value)
    showCreateForm.value = false
    newRule.value = { name: '', channelId: '', eventTypes: [], filters: {}, enabled: true }
    eventTypesInput.value = ''
  } catch (e) {
    alert('Failed to create rule: ' + (e as Error).message)
  }
}

async function handleDelete(id: string) {
  if (!confirm('Are you sure you want to delete this rule?')) return
  try {
    await store.removeRule(id)
  } catch (e) {
    alert('Failed to delete rule: ' + (e as Error).message)
  }
}
</script>

<style scoped>
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
