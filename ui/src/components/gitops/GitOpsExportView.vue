<template>
  <main style="padding:16px">
    <div class="page-header">
      <h1>GitOps Export</h1>
      <button class="primary-button" @click="vm.showCreateForm = true">🚀 Create Export</button>
    </div>

    <GitOpsStatistics :stats="vm.statistics" />
    <GitOpsIntegrationStatus :status="vm.integrationStatus" />

    <GitOpsFilters
      :format="vm.filters.format"
      :success="vm.filters.success"
      :loading="vm.loading"
      @update:format="(v: string) => { vm.filters.format = v; vm.fetchExports() }"
      @update:success="(v?: boolean) => { vm.filters.success = v; vm.fetchExports() }"
      @refresh="vm.refreshData"
    />

    <GitOpsExportList
      :rows="vm.exports"
      :loading="vm.loading"
      :error="vm.error"
      @preview="vm.previewExport"
      @download="(id: string) => vm.downloadExport(id, '')"
      @delete="vm.confirmDelete"
    />

    <PaginationBar
      v-if="vm.meta?.pagination"
      :page="vm.meta.pagination.page"
      :totalPages="vm.meta.pagination.total_pages"
      :hasNext="vm.meta.pagination.has_next"
      :hasPrev="vm.meta.pagination.has_previous"
      @update:page="(p: number) => { vm.currentPage = p; vm.fetchExports() }"
    />

    <GitOpsCreateModal
      v-if="vm.showCreateForm"
      :available-devices="vm.availableDevices"
      :loading="vm.createLoading"
      :error="vm.createError"
      @submit="vm.handleCreateExport"
      @preview="vm.handlePreviewExport"
      @close="vm.closeCreateModal"
    />

    <GitOpsPreviewModal
      v-if="vm.showPreviewModal"
      :item="vm.previewExportItem"
      :data="vm.previewData"
      :downloading-id="vm.downloading"
      @close="vm.closePreviewModal"
      @download="(id: string, name: string) => vm.downloadExport(id, name)"
    />

    <ConfirmDialog
      v-if="vm.deleteConfirm"
      title="Confirm Delete"
      :message="`Are you sure you want to delete GitOps export <strong>${vm.deleteConfirm.name}</strong>?<br/>This action cannot be undone.`"
      confirmText="Delete Export"
      cancelText="Cancel"
      @cancel="vm.deleteConfirm = null"
      @confirm="vm.performDelete"
    />

    <MessageBanner v-if="vm.message.text" :text="vm.message.text" :type="vm.message.type" @close="vm.message.text = ''" />
  </main>
</template>

<script setup lang="ts">
import PaginationBar from '@/components/PaginationBar.vue'
import GitOpsFilters from '@/components/gitops/GitOpsFilters.vue'
import GitOpsExportList from '@/components/gitops/GitOpsExportList.vue'
import GitOpsStatistics from '@/components/gitops/GitOpsStatistics.vue'
import GitOpsIntegrationStatus from '@/components/gitops/GitOpsIntegrationStatus.vue'
import GitOpsCreateModal from '@/components/gitops/GitOpsCreateModal.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import MessageBanner from '@/components/shared/MessageBanner.vue'
import GitOpsPreviewModal from '@/components/gitops/GitOpsPreviewModal.vue'

defineProps<{ vm: any }>()
</script>

