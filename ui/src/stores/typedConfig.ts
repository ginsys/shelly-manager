import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getConfigSchema, type ConfigSchema } from '@/api/typedConfig'

export const useTypedConfigStore = defineStore('typedConfig', () => {
  // Schema cache: cache key (device, model, scope or default) -> schema
  const schemaCache = ref<Map<string, ConfigSchema>>(new Map())

  // Fetch configuration schema (with caching)
  async function fetchSchema(deviceType?: string, deviceId?: number | string, scope?: string) {
    let cacheKey = 'default'
    if (deviceId !== undefined && deviceId !== '') cacheKey = `device:${deviceId}`
    else if (deviceType) cacheKey = deviceType
    else if (scope) cacheKey = `scope:${scope}`

    const cached = schemaCache.value.get(cacheKey)
    if (cached) return cached

    const schema = await getConfigSchema(deviceType, deviceId, scope)
    schemaCache.value.set(cacheKey, schema)
    return schema
  }

  return { fetchSchema }
})
