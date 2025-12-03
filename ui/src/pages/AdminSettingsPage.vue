<template>
  <main style="padding:16px">
    <h1>Admin Settings</h1>
    <form @submit.prevent="onRotate">
      <label>
        New Admin Key:
        <input v-model="newKey" placeholder="enter new key" />
      </label>
      <button :disabled="loading || !newKey">Rotate</button>
    </form>
    <MessageBanner v-if="message" :text="message" type="success" @close="message = ''" />
    <ErrorDisplay
      v-else-if="hasError"
      :error="appError!"
      title="Admin key rotation failed"
      @retry="onRotate"
      @dismiss="clearError"
    />
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { rotateAdminKey } from '@/api/admin'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import MessageBanner from '@/components/shared/MessageBanner.vue'
import { useError } from '@/composables/useError'

const newKey = ref('')
const loading = ref(false)
const message = ref('')
const { error: appError, hasError, setError, clearError } = useError()

async function onRotate() {
  loading.value = true
  message.value = ''
  clearError()
  try {
    await rotateAdminKey(newKey.value)
    message.value = 'Admin key rotated'
  } catch (e:any) {
    setError(e, { action: 'Rotating admin key' })
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
form { display:flex; gap:8px; align-items:center; margin:8px 0; }
.ok { color:#065f46 }
</style>
