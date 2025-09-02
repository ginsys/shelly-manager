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
    <p v-if="message" :class="{ ok:ok, err:!ok }">{{ message }}</p>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { rotateAdminKey } from '@/api/admin'

const newKey = ref('')
const loading = ref(false)
const message = ref('')
const ok = ref(false)

async function onRotate() {
  loading.value = true
  message.value = ''
  try {
    await rotateAdminKey(newKey.value)
    ok.value = true
    message.value = 'Admin key rotated'
  } catch (e:any) {
    ok.value = false
    message.value = e?.message || 'Rotation failed'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
form { display:flex; gap:8px; align-items:center; margin:8px 0; }
.ok { color:#065f46 }
.err { color:#b91c1c }
</style>

