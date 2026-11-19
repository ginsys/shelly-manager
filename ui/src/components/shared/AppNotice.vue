<template>
  <!-- Always mounted, so a screen reader announces the failure when it arrives. Mounted once, in the layout. -->
  <div class="app-notice" role="alert" data-testid="app-notice">
    <ErrorDisplay :error="notice" :show-retry="false" @dismiss="dismissNotice" />
  </div>
</template>

<script setup lang="ts">
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import { notice, dismissNotice } from '@/composables/useNotice'
</script>

<style scoped>
/* Above the page dialogs (z-index 1000), below the admin key dialog (2000). */
.app-notice {
  position: fixed;
  top: 64px;
  right: 16px;
  max-width: 28rem;
  z-index: 1500;
}
</style>
