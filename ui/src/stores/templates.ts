import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  listTemplates,
  createTemplate,
  updateTemplate,
  deleteTemplate,
  type ConfigTemplate,
  type TemplateScope,
  type UpdateTemplateRequest,
} from '@/api/templates'

export const useTemplatesStore = defineStore('templates', () => {
  const templates = ref<ConfigTemplate[]>([])
  // The number of requests in flight. `loading` is true while any is, so a request that ends
  // cannot release the busy state of one that has not.
  const pending = ref(0)
  const loading = computed(() => pending.value > 0)
  const error = ref<string | null>(null)

  const scopeFilter = ref<TemplateScope | ''>('')
  const deviceTypeFilter = ref<string>('')
  const searchFilter = ref<string>('')

  const filteredTemplates = computed(() => {
    const search = searchFilter.value.trim().toLowerCase()
    const deviceType = deviceTypeFilter.value.trim().toLowerCase()

    return templates.value
      .filter((t) => {
        if (!deviceType) return true
        return (t.device_type || '').toLowerCase().includes(deviceType)
      })
      .filter((t) => {
        if (!search) return true
        return (t.name || '').toLowerCase().includes(search)
      })
      .sort((a, b) => (b.updated_at || '').localeCompare(a.updated_at || ''))
  })

  async function fetchTemplates() {
    pending.value++
    error.value = null

    try {
      const result = await listTemplates({ scope: scopeFilter.value || undefined })
      templates.value = result
    } catch (e: any) {
      error.value = e?.message || 'Failed to load templates'
      throw e
    } finally {
      pending.value--
    }
  }

  async function create(data: {
    name: string
    description?: string
    scope: TemplateScope | string
    device_type?: string
    config: Record<string, any>
  }) {
    pending.value++
    error.value = null

    try {
      const created = await createTemplate(data)
      templates.value = [created, ...templates.value]
      return created
    } catch (e: any) {
      error.value = e?.message || 'Failed to create template'
      throw e
    } finally {
      pending.value--
    }
  }

  async function update(id: number | string, data: UpdateTemplateRequest) {
    pending.value++
    error.value = null

    try {
      const updated = await updateTemplate(id, data)
      const index = templates.value.findIndex((t) => t.id === Number(id))
      if (index !== -1) templates.value[index] = updated.template
      return updated
    } catch (e: any) {
      error.value = e?.message || 'Failed to update template'
      throw e
    } finally {
      pending.value--
    }
  }

  async function remove(id: number | string) {
    pending.value++
    error.value = null

    try {
      await deleteTemplate(id)
      templates.value = templates.value.filter((t) => t.id !== Number(id))
    } catch (e: any) {
      error.value = e?.message || 'Failed to delete template'
      throw e
    } finally {
      pending.value--
    }
  }

  function reset() {
    // `pending` is not reset: a request still in flight releases its own share when it ends.
    templates.value = []
    error.value = null
    scopeFilter.value = ''
    deviceTypeFilter.value = ''
    searchFilter.value = ''
  }

  return {
    templates,
    filteredTemplates,
    loading,
    error,

    scopeFilter,
    deviceTypeFilter,
    searchFilter,

    fetchTemplates,
    create,
    update,
    remove,
    reset,
  }
})
