<template>
  <div>
    <div v-if="showBanner" class="version-banner">
      <strong>UI refresh recommended:</strong>
      The API has changed since this UI was built.
      <span class="hint">Dev: run <code>make ui-dev</code>. Production: run <code>make ui-build</code> then restart the server.</span>
      <button class="banner-btn" @click="reload">Reload UI</button>
      <button class="banner-btn subtle" @click="dismiss">Dismiss</button>
    </div>
    <MainLayout />
  </div>
</template>

<script setup lang="ts">
import MainLayout from './layouts/MainLayout.vue'
import api from './api/client'
import { onMounted, ref } from 'vue'

// Expose build metadata; Vite injects __UI_BUILD__ at build time.
;(window as any).__UI_BUILD__ = (window as any).__UI_BUILD__ || (typeof __UI_BUILD__ !== 'undefined' ? (__UI_BUILD__ as any) : {
  build_time: (new Date()).toISOString(),
  git_sha: 'dev'
})

const showBanner = ref(false)
const DISMISS_KEY = 'ui_version_banner_dismissed'

function dismiss() {
  try { localStorage.setItem(DISMISS_KEY, '1') } catch {}
  showBanner.value = false
}
function reload() {
  location.reload()
}

onMounted(async () => {
  // Skip if previously dismissed for this session
  try { if (localStorage.getItem(DISMISS_KEY) === '1') return } catch {}
  try {
    const res = await api.get('/version')
    if (res.data && res.data.success && res.data.data) {
      const apiInfo = res.data.data
      const uiBuild = (window as any).__UI_BUILD__ || {}
      // Heuristic: if UI build time is older than server start, suggest refresh
      const apiStarted = Date.parse(apiInfo.server_started_at || '') || 0
      const uiBuilt = Date.parse(uiBuild.build_time || '') || 0
      if (uiBuilt && apiStarted && uiBuilt < apiStarted) {
        showBanner.value = true
      }
    }
  } catch (e) {
    // Non-fatal; do not show banner on errors
  }
})
</script>

<style>
html, body, #app { height: 100%; margin: 0; }
.version-banner {
  position: sticky;
  top: 0;
  z-index: 1000;
  padding: 10px 16px;
  background: #fff7ed; /* orange-50 */
  border-bottom: 1px solid #fed7aa; /* orange-200 */
  color: #7c2d12; /* orange-900 */
  display: flex;
  align-items: center;
  gap: 12px;
}
.version-banner .hint { opacity: 0.9; margin-left: 6px; }
.version-banner code { background: #fff; padding: 1px 4px; border: 1px solid #f3f4f6; border-radius: 3px; }
.banner-btn { border: 1px solid #d1d5db; background: #fff; color: #374151; padding: 4px 8px; border-radius: 4px; cursor: pointer; }
.banner-btn.subtle { opacity: 0.8; }
.banner-btn:hover { background: #f9fafb; }
</style>
