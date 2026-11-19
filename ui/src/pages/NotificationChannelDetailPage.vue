<template>
  <main style="padding:16px">
    <div style="margin-bottom:16px">
      <router-link to="/notifications/channels" class="back-link">← Back to Channels</router-link>
    </div>

    <h1>Channel Details</h1>

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
        <pre style="background:#f3f4f6;padding:8px;border-radius:4px">{{ JSON.stringify(store.currentChannel.config, null, 2) }}</pre>
      </div>

      <div style="margin-top:24px">
        <button class="primary-button" @click="toggleEnabled">
          {{ store.currentChannel.enabled ? 'Disable' : 'Enable' }}
        </button>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useNotificationsStore } from '@/stores/notifications'
import { useLateFailure } from '@/composables/useNotice'

const route = useRoute()
const store = useNotificationsStore()
const reportLate = useLateFailure()

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
  try {
    await store.modifyChannel(id, { enabled: !enabled })
  } catch (e) {
    // The store keeps the failure in `channelsError`, which the page shows while it is open.
    reportLate(e, { action: 'Update channel', resource: 'Notification channel', resourceId: id })
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
.primary-button:hover {
  background: #1d4ed8;
}
</style>
