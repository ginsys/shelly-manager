import { onUnmounted, ref } from 'vue'
import type { ErrorContext } from '@/types/errors'
import { useError } from './useError'

// The one app-level notice (ginsys/shelly-manager#648). `AppNotice` in the layout shows it. A page panel
// lives in the page component, so a failure that arrives after the user left the page, or while a
// dialog covers the panel, would be shown nowhere: it goes here instead. One notice, no queue: the
// next one replaces it, and only the user's Dismiss clears it.
const { error: notice, setError: setRawNotice, clearError: clearRawNotice } = useError()

// How `AppNotice` shows the notice. A failure is an `error`. A result that arrives late (a success, an
// unconfirmed apply) is not one, so it says `info` or `warning` (ginsys/shelly-manager#660).
export type NoticeSeverity = 'error' | 'warning' | 'info'
const noticeSeverity = ref<NoticeSeverity>('error')

function setNotice(err: unknown, context?: ErrorContext) {
  noticeSeverity.value = 'error'
  setRawNotice(err, context)
}

// A result is shown as a notice without an error code and without suggested actions.
function setNoticeResult(severity: Exclude<NoticeSeverity, 'error'>, message: string, context?: ErrorContext) {
  setRawNotice(message, context)
  if (notice.value) notice.value = { ...notice.value, code: '', suggestions: [] }
  noticeSeverity.value = severity
}

function dismissNotice() {
  noticeSeverity.value = 'error'
  clearRawNotice()
}

export { notice, noticeSeverity, dismissNotice }

// True once the page that called `useUnmounted` in `setup` is gone.
function useUnmounted() {
  let unmounted = false
  onUnmounted(() => {
    unmounted = true
  })
  return () => unmounted
}

// The same shape as `useError`, for a page. Call it in `setup`. `isCovered` says whether a dialog of
// the page hides the panel when the failure arrives.
export function usePageError(isCovered?: () => boolean) {
  const { error, setError: setPanelError, clearError } = useError()
  const isGone = useUnmounted()

  function setError(err: unknown, context?: ErrorContext) {
    if (isGone() || isCovered?.()) setNotice(err, context)
    else setPanelError(err, context)
  }

  return { error, setError, clearError }
}

// For a page whose store already shows a failed action where the user acted (a table row, a card, an
// open dialog). Call it in `setup`. The returned function does nothing while the page is open, and shows
// the failure in the app notice once the user has left, when that place no longer exists. `covered`
// says a dialog hides that place at the moment of the call, which only the page knows.
export function useLateFailure() {
  const isGone = useUnmounted()
  return (err: unknown, context?: ErrorContext, covered = false) => {
    if (isGone() || covered) setNotice(err, context)
  }
}

// The same for the result of an action that the page shows in a card while it is open. Once the user has
// left, the result goes to the app notice, because it may be the only word the user gets (an apply
// changes a physical device).
export function useLateResult() {
  const isGone = useUnmounted()
  return (severity: Exclude<NoticeSeverity, 'error'>, message: string, context?: ErrorContext) => {
    if (isGone()) setNoticeResult(severity, message, context)
  }
}
