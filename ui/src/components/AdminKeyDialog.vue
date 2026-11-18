<template>
  <div
    v-if="promptOpen"
    class="akd-backdrop"
    data-testid="admin-key-dialog"
    role="dialog"
    aria-modal="true"
    aria-labelledby="akd-title"
    @keydown.escape="cancelAdminKeyPrompt"
  >
    <form class="akd-panel" @submit.prevent="submit">
      <h2 id="akd-title" class="akd-title">Admin key</h2>
      <p class="akd-help">
        Enter the admin key for this server. It is kept for this browser tab only and is forgotten when the tab closes.
      </p>
      <label class="akd-label" for="akd-input">Admin key</label>
      <input
        id="akd-input"
        ref="field"
        v-model="key"
        class="akd-input"
        data-testid="admin-key-input"
        type="password"
        autocomplete="off"
        spellcheck="false"
      />
      <p class="akd-message" data-testid="admin-key-message" aria-live="polite">{{ promptMessage }}</p>
      <div class="akd-actions">
        <button type="button" class="akd-btn" data-testid="admin-key-cancel" @click="cancelAdminKeyPrompt">Cancel</button>
        <button type="submit" class="akd-btn primary" data-testid="admin-key-submit">Use key</button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { promptOpen, promptMessage, submitAdminKey, cancelAdminKeyPrompt } from '@/api/adminKey'

const key = ref('')
const field = ref<HTMLInputElement | null>(null)

function focusField() {
  key.value = ''
  nextTick(() => field.value?.focus())
}

// Each opening starts with an empty field: a refused key is never left in it.
watch(promptOpen, (open) => {
  if (open) focusField()
})
onMounted(() => {
  if (promptOpen.value) focusField()
})
// A dialog that goes away must not leave a request waiting for a key nobody can enter.
onBeforeUnmount(() => {
  if (promptOpen.value) cancelAdminKeyPrompt()
})

function submit() {
  const value = key.value
  key.value = ''
  submitAdminKey(value)
}
</script>

<style scoped>
.akd-backdrop {
  position: fixed;
  inset: 0;
  z-index: 2000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.55);
}
.akd-panel {
  width: min(420px, calc(100vw - 32px));
  padding: 20px;
  border-radius: 8px;
  background: #fff;
  color: #1f2937;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.25);
}
.akd-title { margin: 0 0 8px; font-size: 18px; }
.akd-help { margin: 0 0 12px; font-size: 13px; color: #4b5563; }
.akd-label { display: block; margin-bottom: 4px; font-size: 13px; font-weight: 600; }
.akd-input {
  width: 100%;
  box-sizing: border-box;
  padding: 8px 10px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font: inherit;
}
.akd-message { min-height: 18px; margin: 8px 0 0; font-size: 13px; color: #b91c1c; }
.akd-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
.akd-btn {
  padding: 6px 14px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  background: #fff;
  color: #374151;
  cursor: pointer;
}
.akd-btn.primary { border-color: #2563eb; background: #2563eb; color: #fff; }
</style>
