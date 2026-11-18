import { ref } from 'vue'

// The admin key for this browser tab. Every reader (the request interceptor, the metrics socket URL,
// the rotation call) goes through this module, so no component touches sessionStorage or
// window.__ADMIN_KEY__ itself.
//
// The key an operator types lives in sessionStorage: gone when the tab closes, never written to
// localStorage and never part of the bundle. VITE_ADMIN_KEY and window.__ADMIN_KEY__ (from
// /app-config.js) stay as development sources and rank below an entered key.

const STORAGE_NAME = 'shelly_admin_key'
const REJECTED_MESSAGE = 'That key was not accepted.'

// Used when sessionStorage is unavailable (blocked storage), so the key still lasts the page.
let memoryKey: string | null = null

function readStored(): string | null {
  try {
    const v = sessionStorage.getItem(STORAGE_NAME)
    if (v) return v
  } catch {
    // fall through to memory
  }
  return memoryKey
}

/** True while an operator-entered key exists; drives the header's "Forget key" button. */
export const hasSessionKey = ref(readStored() !== null)

export function getAdminKey(): string | undefined {
  const env: any = (import.meta as any)?.env || {}
  return readStored() || env.VITE_ADMIN_KEY || (window as any).__ADMIN_KEY__ || undefined
}

export function setAdminKey(key: string): void {
  memoryKey = key
  try {
    sessionStorage.setItem(STORAGE_NAME, key)
  } catch {
    // the in-memory copy carries it
  }
  hasSessionKey.value = true
}

export function forgetAdminKey(): void {
  memoryKey = null
  try {
    sessionStorage.removeItem(STORAGE_NAME)
  } catch {
    // nothing stored to remove
  }
  hasSessionKey.value = false
}

// --- the prompt ---------------------------------------------------------------------------------

export const promptOpen = ref(false)
export const promptMessage = ref('')

let pending: Promise<string | null> | null = null
let resolvePending: ((key: string | null) => void) | null = null
// Set by Cancel: a later 401 (a polling page, say) rejects quietly instead of reopening the dialog,
// until the operator opens it from the header or submits a key.
let declined = false

function settle(key: string | null) {
  const done = resolvePending
  pending = null
  resolvePending = null
  done?.(key)
}

/**
 * Ask for the key after an admin 401. Every request that waits shares one prompt. Resolves with the
 * entered key, or null when the operator cancelled (or already cancelled earlier).
 */
export function requestAdminKey(): Promise<string | null> {
  if (declined) return Promise.resolve(null)
  if (!pending) {
    pending = new Promise((resolve) => {
      resolvePending = resolve
    })
  }
  promptOpen.value = true
  return pending
}

/** Open the dialog by hand (the header button). */
export function openAdminKeyPrompt(): void {
  declined = false
  promptMessage.value = ''
  promptOpen.value = true
}

export function submitAdminKey(key: string): void {
  const trimmed = key.trim()
  if (!trimmed) return
  setAdminKey(trimmed)
  declined = false
  promptMessage.value = ''
  promptOpen.value = false
  settle(trimmed)
}

export function cancelAdminKeyPrompt(): void {
  declined = true
  promptMessage.value = ''
  promptOpen.value = false
  settle(null)
}

/** The server refused the key we just sent: drop it, say so, and ask again. */
export function rejectAdminKey(): void {
  forgetAdminKey()
  promptMessage.value = REJECTED_MESSAGE
  promptOpen.value = true
}
