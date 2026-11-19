<template>
  <!-- Always mounted, so a screen reader announces what arrives. Mounted once, in the layout. -->
  <div class="app-notice" role="alert" data-testid="app-notice">
    <ErrorDisplay :error="notice" :severity="noticeSeverity" :show-retry="false" @dismiss="dismissNotice" />
  </div>
</template>

<script setup lang="ts">
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import { notice, noticeSeverity, dismissNotice } from '@/composables/useNotice'
</script>

<style scoped>
/* Above the page dialogs (z-index 1000), below the admin key dialog (2000). */
.app-notice {
  position: fixed;
  top: 64px;
  right: 16px;
  max-width: 28rem;
  /* A long error or a short screen scrolls inside the notice, so its Dismiss button stays reachable. */
  max-height: calc(100vh - 80px);
  overflow-y: auto;
  z-index: 1500;
}
</style>
