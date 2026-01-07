import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listTemplates,
  createTemplate,
  updateTemplate,
  deleteTemplate,
  type ConfigTemplate,
  type ListTemplatesParams
} from '@/api/templates'
import type { Metadata } from '@/api/types'

export const useTemplatesStore = defineStore('templates', () => {
  // State
  const templates = ref<ConfigTemplate[]>([])
  const currentTemplate = ref<ConfigTemplate | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const meta = ref<Metadata | undefined>(undefined)

  // Filters
  const deviceTypeFilter = ref<string | undefined>(undefined)
  const searchFilter = ref<string | undefined>(undefined)
  const page = ref(1)
  const pageSize = ref(25)

  // Actions

  // Fetch templates list
  async function fetchTemplates(params?: ListTemplatesParams) {
    loading.value = true
    error.value = null
    try {
      const result = await listTemplates({
        page: params?.page || page.value,
        pageSize: params?.pageSize || pageSize.value,
        deviceType: params?.deviceType || deviceTypeFilter.value,
        search: params?.search || searchFilter.value
      })
      templates.value = result.items
      meta.value = result.meta
    } catch (e: any) {
      error.value = e?.message || 'Failed to load templates'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Create new template
  async function create(data: Partial<ConfigTemplate>) {
    loading.value = true
    error.value = null
    try {
      const template = await createTemplate(data)
      templates.value.unshift(template)
      return template
    } catch (e: any) {
      error.value = e?.message || 'Failed to create template'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Update existing template
  async function update(id: number | string, data: Partial<ConfigTemplate>) {
    loading.value = true
    error.value = null
    try {
      const template = await updateTemplate(id, data)
      const index = templates.value.findIndex(t => t.id === Number(id))
      if (index !== -1) {
        templates.value[index] = template
      }
      if (currentTemplate.value?.id === Number(id)) {
        currentTemplate.value = template
      }
      return template
    } catch (e: any) {
      error.value = e?.message || 'Failed to update template'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Delete template
  async function remove(id: number | string) {
    loading.value = true
    error.value = null
    try {
      await deleteTemplate(id)
      templates.value = templates.value.filter(t => t.id !== Number(id))
      if (currentTemplate.value?.id === Number(id)) {
        currentTemplate.value = null
      }
    } catch (e: any) {
      error.value = e?.message || 'Failed to delete template'
      throw e
    } finally {
      loading.value = false
    }
  }

  // Set current template
  function setCurrentTemplate(template: ConfigTemplate | null) {
    currentTemplate.value = template
  }

  // Set filters
  function setDeviceTypeFilter(deviceType: string | undefined) {
    deviceTypeFilter.value = deviceType
  }

  function setSearchFilter(search: string | undefined) {
    searchFilter.value = search
  }

  function setPage(newPage: number) {
    page.value = newPage
  }

  function setPageSize(newPageSize: number) {
    pageSize.value = newPageSize
  }

  // Reset store
  function reset() {
    templates.value = []
    currentTemplate.value = null
    loading.value = false
    error.value = null
    meta.value = undefined
    deviceTypeFilter.value = undefined
    searchFilter.value = undefined
    page.value = 1
    pageSize.value = 25
  }

  return {
    // State
    templates,
    currentTemplate,
    loading,
    error,
    meta,
    deviceTypeFilter,
    searchFilter,
    page,
    pageSize,

    // Actions
    fetchTemplates,
    create,
    update,
    remove,
    setCurrentTemplate,
    setDeviceTypeFilter,
    setSearchFilter,
    setPage,
    setPageSize,
    reset
  }
})
