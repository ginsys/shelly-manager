<template>
  <main style="padding:16px">
    <h1>Notification Channels</h1>
    <p style="color:#6b7280;margin-bottom:16px">
      Manage notification channels for email, webhooks, and Slack
    </p>

    <div style="margin-bottom:16px">
      <button class="primary-button" @click="showCreateForm = true">
        ➕ Create Channel
      </button>
    </div>

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
    <div v-if="showCreateForm" class="modal-overlay" @click="showCreateForm = false">
      <div class="modal" @click.stop>
        <h2>Create Notification Channel</h2>
        <form @submit.prevent="handleCreate">
          <div class="form-group">
            <label>Name:</label>
            <input v-model="newChannel.name" required class="form-input" />
          </div>
          <div class="form-group">
            <label>Type:</label>
            <select v-model="newChannel.type" required class="form-input">
              <option value="email">Email</option>
              <option value="webhook">Webhook</option>
              <option value="slack">Slack</option>
            </select>
          </div>
          <div class="form-group">
            <label>Enabled:</label>
            <input type="checkbox" v-model="newChannel.enabled" />
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
import { ref, onMounted } from 'vue'
import { useNotificationsStore } from '@/stores/notifications'
import DataTable from '@/components/DataTable.vue'

const store = useNotificationsStore()
const showCreateForm = ref(false)
const newChannel = ref({
  name: '',
  type: 'email' as 'email' | 'webhook' | 'slack',
  enabled: true,
  config: {}
})

onMounted(() => {
  store.fetchChannels()
})

async function handleCreate() {
  try {
    await store.addChannel(newChannel.value)
    showCreateForm.value = false
    newChannel.value = { name: '', type: 'email', enabled: true, config: {} }
  } catch (e) {
    alert('Failed to create channel: ' + (e as Error).message)
  }
}

async function handleDelete(id: string) {
  if (!confirm('Are you sure you want to delete this channel?')) return
  try {
    await store.removeChannel(id)
  } catch (e) {
    alert('Failed to delete channel: ' + (e as Error).message)
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
