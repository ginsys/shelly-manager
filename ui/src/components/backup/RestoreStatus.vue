<template>
  <section v-if="visible" class="restore-status" data-testid="restore-status">
    <div v-if="!status?.supported" class="status-row info" data-testid="restore-unsupported">
      {{ RESTORE_SQLITE_ONLY }}. Backups can still be checked.
    </div>

    <div v-if="status?.pending" class="status-row pending" data-testid="restore-pending">
      <span>
        <strong>Restore staged:</strong> {{ nameOf(status.pending.backup_id) }}, staged
        {{ formatTime(status.pending.staged_at) }}. A server restart applies it.
      </span>
      <button
        type="button"
        class="cancel-button"
        data-testid="restore-cancel"
        :disabled="cancelling"
        @click="emit('cancel')"
      >
        {{ cancelling ? 'Cancelling...' : 'Cancel restore' }}
      </button>
    </div>

    <div
      v-if="status?.lastResult"
      :class="['status-row', status.lastResult.status]"
      data-testid="restore-last-result"
    >
      Last restore {{ status.lastResult.status }}<template v-if="status.lastResult.backup_id">
        ({{ nameOf(status.lastResult.backup_id) }})</template>,
      {{ formatTime(status.lastResult.at) }}<template v-if="status.lastResult.error">:
        {{ status.lastResult.error }}</template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RESTORE_SQLITE_ONLY, type BackupItem, type RestoreStatus } from '@/api/export'

interface Props {
  status: RestoreStatus | null
  backups: BackupItem[]
  cancelling?: boolean
}

const props = withDefaults(defineProps<Props>(), { cancelling: false })

const emit = defineEmits<{ cancel: [] }>()

const visible = computed(
  () => !!props.status && (!props.status.supported || !!props.status.pending || !!props.status.lastResult),
)

// The name of a backup on the current page; a backup on another page shows its id.
function nameOf(backupId: string): string {
  return props.backups.find((b) => b.backup_id === backupId)?.name || backupId
}

function formatTime(value: string): string {
  return new Date(value).toLocaleString()
}
</script>

<style scoped>
.restore-status {
  margin-bottom: 16px;
  display: grid;
  gap: 8px;
}

.status-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-radius: 6px;
  font-size: 0.875rem;
  background: #f3f4f6;
  border: 1px solid #e5e7eb;
  color: #374151;
}

.status-row.pending {
  background: #fffbeb;
  border-color: #fde68a;
  color: #92400e;
}

.status-row.applied {
  background: #ecfdf5;
  border-color: #a7f3d0;
  color: #065f46;
}

.status-row.failed {
  background: #fee2e2;
  border-color: #fecaca;
  color: #991b1b;
}

.cancel-button {
  padding: 6px 14px;
  border-radius: 6px;
  border: 1px solid #d1d5db;
  background: white;
  color: #374151;
  cursor: pointer;
}

.cancel-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
