<template>
  <div v-if="backup" class="modal-overlay" @click="emit('cancel')">
    <div class="modal-content confirm-modal" @click.stop>
      <h3>Confirm Delete</h3>
      <p>Are you sure you want to delete backup <strong>{{ backup.name }}</strong>?</p>
      <p class="warning">This action cannot be undone.</p>
      <div class="modal-actions">
        <button class="secondary-button" @click="emit('cancel')">Cancel</button>
        <button class="danger-button" @click="emit('confirm')">Delete Backup</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { BackupItem } from '@/api/export'

interface Props {
  backup: BackupItem | null
}

defineProps<Props>()

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 16px;
}

.modal-content {
  background: white;
  border-radius: 8px;
  max-width: 800px;
  width: 100%;
  max-height: 90vh;
  overflow: auto;
}

.confirm-modal {
  max-width: 500px;
  padding: 24px;
}

.confirm-modal h3 {
  margin: 0 0 16px 0;
  color: #1f2937;
}

.confirm-modal p {
  margin: 0 0 12px 0;
  color: #4b5563;
}

.warning {
  color: #dc2626;
  font-weight: 500;
}

.modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 24px;
}

.secondary-button, .danger-button {
  padding: 10px 20px;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.secondary-button {
  background: white;
  border: 1px solid #d1d5db;
  color: #374151;
}

.secondary-button:hover {
  background: #f3f4f6;
}

.danger-button {
  background-color: #dc2626;
  color: white;
  border: none;
}

.danger-button:hover {
  background-color: #b91c1c;
}
</style>
