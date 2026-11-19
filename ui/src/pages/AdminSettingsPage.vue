<template>
  <main style="padding:16px">
    <h1>Admin Settings</h1>
    <form @submit.prevent="onRotate">
      <label for="admin-new-key">New admin key</label>
      <input
        id="admin-new-key"
        v-model="newKey"
        data-testid="admin-new-key"
        type="password"
        autocomplete="new-password"
        spellcheck="false"
      />
      <label for="admin-new-key-confirm">Repeat new admin key</label>
      <input
        id="admin-new-key-confirm"
        v-model="repeatKey"
        data-testid="admin-new-key-confirm"
        type="password"
        autocomplete="new-password"
        spellcheck="false"
      />
      <p v-if="mismatch" class="err" data-testid="admin-rotate-mismatch">The two keys do not match.</p>
      <p v-if="edgeSpace" class="err" data-testid="admin-rotate-edge-space">A key cannot start or end with a space.</p>
      <button type="submit" data-testid="admin-rotate-submit" :disabled="!canRotate">Rotate</button>
    </form>
    <p class="note" data-testid="admin-rotate-note">
      The new key applies to the running server only and is kept in memory: when the server restarts, the key from
      its configuration is back. Other browsers and tools that use the old key stop working until they are given the
      new key. This tab keeps using the new key until it is closed.
    </p>
    <p
      :class="{ ok: ok, err: !ok }"
      data-testid="admin-rotate-message"
      aria-live="polite"
    >{{ message }}</p>
  </main>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { rotateAdminKey } from '@/api/admin'

const newKey = ref('')
const repeatKey = ref('')
const loading = ref(false)
const message = ref('')
const ok = ref(false)

// The key reaches the server in a header, which loses leading and trailing whitespace. The server
// would store " k3y " as typed and then refuse every client that sends it, this tab included, so
// such a key is never sent. Spaces inside the key are fine and the key is otherwise sent as typed.
const edgeSpace = computed(() => newKey.value !== '' && newKey.value !== newKey.value.trim())
const mismatch = computed(() => repeatKey.value !== '' && repeatKey.value !== newKey.value)
const canRotate = computed(
  () => !loading.value && newKey.value !== '' && !edgeSpace.value && newKey.value === repeatKey.value
)

async function onRotate() {
  // A submit does not have to go through the button, so the rules are checked again here.
  if (!canRotate.value) return
  loading.value = true
  message.value = ''
  try {
    await rotateAdminKey(newKey.value)
    ok.value = true
    message.value = 'Admin key rotated. This tab now uses the new key.'
    newKey.value = ''
    repeatKey.value = ''
  } catch (e:any) {
    ok.value = false
    message.value = e?.message || 'Rotation failed'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
form { display:flex; flex-direction:column; gap:8px; align-items:flex-start; margin:8px 0; max-width:24rem; }
input { width:100%; }
.note { max-width:40rem; color:#4b5563 }
.ok { color:#065f46 }
.err { color:#b91c1c }
</style>
