import { onUnmounted } from 'vue'
import type { ErrorContext } from '@/types/errors'
import { useError } from './useError'

// The one app-level notice (ginsys/shelly-manager#648). `AppNotice` in the layout shows it. A page panel
// lives in the page component, so a failure that arrives after the user left the page, or while a
// dialog covers the panel, would be shown nowhere: it goes here instead. One notice, no queue: the
// next failure replaces it, and only the user's Dismiss clears it.
const { error: notice, setError: setNotice, clearError: dismissNotice } = useError()
export { notice, dismissNotice }

// The same shape as `useError`, for a page. Call it in `setup`. `isCovered` says whether a dialog of
// the page hides the panel when the failure arrives.
export function usePageError(isCovered?: () => boolean) {
  const { error, setError: setPanelError, clearError } = useError()
  let unmounted = false
  onUnmounted(() => {
    unmounted = true
  })

  function setError(err: unknown, context?: ErrorContext) {
    if (unmounted || isCovered?.()) setNotice(err, context)
    else setPanelError(err, context)
  }

  return { error, setError, clearError }
}
