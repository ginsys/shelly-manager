<template>
  <div class="card login-card" data-testid="device-login-card">
    <h2 class="section-title">Device Login</h2>
    <p class="hint">
      The username and password the manager uses to reach this device. Clear the username to turn
      device authentication off.
    </p>
    <div class="form-group">
      <label for="device-login-username">Username</label>
      <input
        id="device-login-username"
        v-model="username"
        data-testid="device-login-username"
        type="text"
        class="form-input"
        autocomplete="off"
        :disabled="saving"
        @input="dirty = true"
      />
    </div>
    <div class="form-group">
      <label for="device-login-password">Password</label>
      <input
        id="device-login-password"
        v-model="password"
        data-testid="device-login-password"
        type="password"
        class="form-input"
        autocomplete="new-password"
        :disabled="saving"
        @input="dirty = true"
      />
      <div v-if="password === SAVED_SECRET_MASK" class="hint">
        A password is saved. Leave the mask to keep it.
      </div>
    </div>
    <div v-if="message" :class="['message', messageKind]" data-testid="device-login-message">{{ message }}</div>
    <button
      class="btn-primary"
      data-testid="device-login-save"
      :disabled="saving"
      @click="save"
    >
      {{ saving ? 'Saving...' : 'Save Login' }}
    </button>
  </div>
</template>

<script setup lang="ts">
// DeviceLoginCard edits the stored login of one device (PUT /devices/{id}/config/auth). The device
// routes show a saved password as the mask, so the card starts from the mask and sends it back
// unchanged: the server keeps the stored password. The override section "auth" is a different
// thing: it sets the device's own access protection.
import { ref, watch } from 'vue'
import { updateDeviceAuth } from '@/api/devices'
import { SAVED_SECRET_MASK } from '@/utils/config-form'

const props = defineProps<{
  deviceId: number | string
  // The device's settings text as the device routes return it, with the password masked.
  settings?: string
}>()
const emit = defineEmits<{ (e: 'saved'): void }>()

const username = ref('')
const password = ref('')
const saving = ref(false)
const message = ref('')
const messageKind = ref<'ok' | 'error'>('ok')

function text(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

// The manager reads the flat keys. It falls back to the nested pair older versions wrote only when
// both flat values are empty and the pair is complete (configuration.LegacyDeviceAuth), so the
// card picks one pair and never mixes the two.
function readLogin(settings: string | undefined): { username: string; password: string } {
  let doc: any = null
  try {
    doc = settings ? JSON.parse(settings) : null
  } catch {
    // Settings that are not JSON (the routes return the mask whole) hold no readable login.
  }
  if (!doc || typeof doc !== 'object') return { username: '', password: '' }
  const flat = { username: text(doc.auth_user), password: text(doc.auth_pass) }
  if (flat.username !== '' || flat.password !== '') return flat
  const nested = doc.auth && typeof doc.auth === 'object' ? doc.auth : {}
  const legacy = { username: text(nested.username), password: text(nested.password) }
  return legacy.username !== '' && legacy.password !== '' ? legacy : { username: '', password: '' }
}

// True once the user has typed in a field since the fields were last loaded or saved. A reload of
// the device settings then leaves the fields alone, so a refresh never discards an edit.
const dirty = ref(false)

watch(
  () => props.settings,
  (settings) => {
    if (dirty.value) return
    const login = readLogin(settings)
    username.value = login.username
    password.value = login.password
  },
  { immediate: true },
)

async function save() {
  message.value = ''
  // Match the API: a user without a password would be stored as enabled auth that is never used.
  if (username.value !== '' && password.value === '') {
    messageKind.value = 'error'
    message.value = 'A device login needs a password. Clear the username to turn device auth off.'
    return
  }
  // The inputs are disabled while the request runs; the snapshot is what was sent.
  const sent = { username: username.value, password: username.value === '' ? '' : password.value }
  saving.value = true
  try {
    await updateDeviceAuth(props.deviceId, sent)
    // Show the saved state, not what was typed or what a reload put there during the request.
    username.value = sent.username
    password.value = sent.username === '' ? '' : SAVED_SECRET_MASK
    dirty.value = false
    messageKind.value = 'ok'
    message.value = sent.username === '' ? 'Device authentication is off.' : 'Device login saved.'
    emit('saved')
  } catch (e: any) {
    messageKind.value = 'error'
    message.value = e?.message || 'Failed to save the device login'
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.login-card { padding: 16px; }
.section-title { margin: 0 0 8px; font-size: 16px; }
.hint { color: #64748b; font-size: 13px; margin: 4px 0 8px; }
.form-group { display: flex; flex-direction: column; gap: 4px; margin-bottom: 12px; max-width: 360px; }
.form-input { padding: 8px; border: 1px solid #cbd5e1; border-radius: 6px; }
.message { margin-bottom: 12px; font-size: 14px; }
.message.ok { color: #15803d; }
.message.error { color: #b91c1c; }
</style>
