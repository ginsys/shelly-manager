import { vi, describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDeviceConfigNewStore } from '../deviceConfigNew'
import * as configNewApi from '@/api/configNew'
import type {
  ConfigStatusData,
  DesiredConfigData,
  DeviceConfiguration,
  DeviceTemplatesData,
  ConfigApplyData,
  ConfigVerifyData,
} from '@/api/configNew'
import type { ConfigTemplate } from '@/api/templates'

// Mock the API
vi.mock('@/api/configNew', () => ({
  getDeviceTemplatesNew: vi.fn(),
  setDeviceTemplatesNew: vi.fn(),
  addDeviceTemplateNew: vi.fn(),
  removeDeviceTemplateNew: vi.fn(),
  getDeviceOverridesNew: vi.fn(),
  setDeviceOverridesNew: vi.fn(),
  patchDeviceOverridesNew: vi.fn(),
  deleteDeviceOverridesNew: vi.fn(),
  getDesiredConfigNew: vi.fn(),
  getConfigStatusNew: vi.fn(),
  applyDeviceConfigNew: vi.fn(),
  verifyDeviceConfigNew: vi.fn(),
}))

const mockApi = vi.mocked(configNewApi)

describe('DeviceConfigNew Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
  })

  const mockTemplate: ConfigTemplate = {
    id: 1,
    name: 'Global MQTT',
    scope: 'global',
    config: { mqtt: { enabled: true, server: 'mqtt.local' } },
    created_at: '2023-01-01T00:00:00Z',
    updated_at: '2023-01-01T00:00:00Z',
  }

  const mockTemplatesData: DeviceTemplatesData = {
    templates: [mockTemplate],
    template_ids: [1],
  }

  const mockOverridesData = {
    overrides: { mqtt: { user: 'device_user' } },
  }

  const mockDesiredConfigData: DesiredConfigData = {
    config: {
      mqtt: { enabled: true, server: 'mqtt.local', user: 'device_user' },
    },
    sources: {
      'mqtt.enabled': 'template:1',
      'mqtt.server': 'template:1',
      'mqtt.user': 'override',
    },
  }

  const mockStatusData: ConfigStatusData = {
    status: 'applied',
    last_applied: '2023-01-01T00:00:00Z',
  }

  const mockApplyData: ConfigApplyData = {
    success: true,
    message: 'Config applied successfully',
  }

  const mockVerifyData: ConfigVerifyData = {
    matches: true,
    differences: [],
  }

  describe('Initial state', () => {
    it('should have correct initial state', () => {
      const store = useDeviceConfigNewStore()

      expect(store.templates).toEqual([])
      expect(store.templateIds).toEqual([])
      expect(store.overrides).toBeNull()
      expect(store.desiredConfig).toBeNull()
      expect(store.sources).toEqual({})
      expect(store.status).toBeNull()
      expect(store.lastApply).toBeNull()
      expect(store.lastVerify).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })
  })

  describe('refresh', () => {
    it('should load all config data successfully', async () => {
      const store = useDeviceConfigNewStore()

      mockApi.getDeviceTemplatesNew.mockResolvedValue(mockTemplatesData)
      mockApi.getDeviceOverridesNew.mockResolvedValue(mockOverridesData)
      mockApi.getDesiredConfigNew.mockResolvedValue(mockDesiredConfigData)
      mockApi.getConfigStatusNew.mockResolvedValue(mockStatusData)

      await store.refresh(123)

      expect(store.templates).toEqual([mockTemplate])
      expect(store.templateIds).toEqual([1])
      expect(store.overrides).toEqual(mockOverridesData.overrides)
      expect(store.desiredConfig).toEqual(mockDesiredConfigData.config)
      expect(store.sources).toEqual(mockDesiredConfigData.sources)
      expect(store.status).toEqual(mockStatusData)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should handle errors during refresh', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Network error'

      mockApi.getDeviceTemplatesNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.refresh(123)).rejects.toThrow(errorMessage)

      expect(store.error).toBe(errorMessage)
      expect(store.loading).toBe(false)
    })

    it('should set loading state during refresh', async () => {
      const store = useDeviceConfigNewStore()

      mockApi.getDeviceTemplatesNew.mockResolvedValue(mockTemplatesData)
      mockApi.getDeviceOverridesNew.mockResolvedValue(mockOverridesData)
      mockApi.getDesiredConfigNew.mockResolvedValue(mockDesiredConfigData)
      mockApi.getConfigStatusNew.mockResolvedValue(mockStatusData)

      const promise = store.refresh(123)
      expect(store.loading).toBe(true)

      await promise
      expect(store.loading).toBe(false)
    })
  })

  describe('addTemplate', () => {
    it('should add a template successfully', async () => {
      const store = useDeviceConfigNewStore()
      const updatedTemplates = [mockTemplate, { ...mockTemplate, id: 2, name: 'Template 2' }]

      mockApi.addDeviceTemplateNew.mockResolvedValue(updatedTemplates)

      await store.addTemplate(123, 2)

      expect(mockApi.addDeviceTemplateNew).toHaveBeenCalledWith({
        deviceId: 123,
        templateId: 2,
        position: undefined,
      })
      expect(store.templates).toEqual(updatedTemplates)
      expect(store.templateIds).toEqual([1, 2])
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should add a template at specific position', async () => {
      const store = useDeviceConfigNewStore()
      const updatedTemplates = [mockTemplate]

      mockApi.addDeviceTemplateNew.mockResolvedValue(updatedTemplates)

      await store.addTemplate(123, 2, 0)

      expect(mockApi.addDeviceTemplateNew).toHaveBeenCalledWith({
        deviceId: 123,
        templateId: 2,
        position: 0,
      })
    })

    it('should handle errors when adding template', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Template not found'

      mockApi.addDeviceTemplateNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.addTemplate(123, 999)).rejects.toThrow(errorMessage)

      expect(store.error).toBe(errorMessage)
      expect(store.loading).toBe(false)
    })
  })

  describe('removeTemplate', () => {
    it('should remove a template successfully', async () => {
      const store = useDeviceConfigNewStore()
      store.templates = [mockTemplate, { ...mockTemplate, id: 2 }]
      store.templateIds = [1, 2]

      mockApi.removeDeviceTemplateNew.mockResolvedValue([mockTemplate])

      await store.removeTemplate(123, 2)

      expect(mockApi.removeDeviceTemplateNew).toHaveBeenCalledWith(123, 2)
      expect(store.templates).toEqual([mockTemplate])
      expect(store.templateIds).toEqual([1])
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should handle errors when removing template', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Template not assigned'

      mockApi.removeDeviceTemplateNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.removeTemplate(123, 999)).rejects.toThrow(errorMessage)

      expect(store.error).toBe(errorMessage)
    })
  })

  describe('inherited configuration', () => {
    // mqtt.user is set by a template and by the device override: the desired configuration keeps
    // only the override, the inherited one keeps the template value the field falls back to.
    const withInherited: DesiredConfigData = {
      ...mockDesiredConfigData,
      inherited: { mqtt: { enabled: true, server: 'mqtt.local', user: 'template_user' } },
      inherited_sources: { 'mqtt.user': 'template:1' },
    }

    it('refresh loads the inherited configuration and its sources', async () => {
      const store = useDeviceConfigNewStore()
      mockApi.getDeviceTemplatesNew.mockResolvedValue(mockTemplatesData)
      mockApi.getDeviceOverridesNew.mockResolvedValue(mockOverridesData)
      mockApi.getDesiredConfigNew.mockResolvedValue(withInherited)
      mockApi.getConfigStatusNew.mockResolvedValue(mockStatusData)

      await store.refresh(123)

      expect(store.inherited).toEqual(withInherited.inherited)
      expect(store.inheritedSources).toEqual({ 'mqtt.user': 'template:1' })
    })

    it('fetchDesired loads it, and a response without it leaves none', async () => {
      const store = useDeviceConfigNewStore()
      mockApi.getDesiredConfigNew.mockResolvedValue(withInherited)
      await store.fetchDesired(123)
      expect(store.inherited).toEqual(withInherited.inherited)

      mockApi.getDesiredConfigNew.mockResolvedValue(mockDesiredConfigData)
      await store.fetchDesired(123)
      expect(store.inherited).toBeNull()
      expect(store.inheritedSources).toEqual({})
    })

    it('setTemplates takes the new inherited configuration from the response', async () => {
      const store = useDeviceConfigNewStore()
      mockApi.setDeviceTemplatesNew.mockResolvedValue({
        ...mockTemplatesData,
        desired_config: mockDesiredConfigData.config,
        sources: mockDesiredConfigData.sources,
        inherited: withInherited.inherited,
        inherited_sources: withInherited.inherited_sources,
      })

      await store.setTemplates(123, [1])

      expect(store.inherited).toEqual(withInherited.inherited)
      expect(store.inheritedSources).toEqual({ 'mqtt.user': 'template:1' })
    })

    it('reset clears it', () => {
      const store = useDeviceConfigNewStore()
      store.inherited = { mqtt: { enabled: true } }
      store.inheritedSources = { 'mqtt.enabled': 'template:1' }
      store.reset()
      expect(store.inherited).toBeNull()
      expect(store.inheritedSources).toEqual({})
    })
  })

  describe('setTemplates', () => {
    it('should set template order successfully', async () => {
      const store = useDeviceConfigNewStore()
      const result = {
        ...mockTemplatesData,
        desired_config: mockDesiredConfigData.config,
        sources: mockDesiredConfigData.sources,
      }

      mockApi.setDeviceTemplatesNew.mockResolvedValue(result)

      await store.setTemplates(123, [2, 1])

      expect(mockApi.setDeviceTemplatesNew).toHaveBeenCalledWith(123, [2, 1])
      expect(store.templates).toEqual(result.templates)
      expect(store.templateIds).toEqual(result.template_ids)
      expect(store.desiredConfig).toEqual(result.desired_config)
      expect(store.sources).toEqual(result.sources)
    })

    it('should handle partial response without desired_config', async () => {
      const store = useDeviceConfigNewStore()
      store.desiredConfig = { existing: 'config' }

      mockApi.setDeviceTemplatesNew.mockResolvedValue(mockTemplatesData)

      await store.setTemplates(123, [1])

      expect(store.desiredConfig).toEqual({ existing: 'config' }) // unchanged
    })
  })

  describe('saveOverrides', () => {
    it('should save overrides successfully', async () => {
      const store = useDeviceConfigNewStore()
      const newOverrides = { mqtt: { user: 'new_user' } }
      const result = {
        overrides: newOverrides,
        desired_config: mockDesiredConfigData.config,
      }

      mockApi.setDeviceOverridesNew.mockResolvedValue(result)

      await store.saveOverrides(123, newOverrides)

      expect(mockApi.setDeviceOverridesNew).toHaveBeenCalledWith(123, newOverrides)
      expect(store.overrides).toEqual(newOverrides)
      expect(store.desiredConfig).toEqual(result.desired_config)
    })

    it('should handle errors when saving overrides', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Validation failed'

      mockApi.setDeviceOverridesNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.saveOverrides(123, {})).rejects.toThrow(errorMessage)

      expect(store.error).toBe(errorMessage)
    })
  })

  describe('patchOverrides', () => {
    it('should patch overrides successfully', async () => {
      const store = useDeviceConfigNewStore()
      const patch = { mqtt: { port: 1883 } }
      const patched = { mqtt: { user: 'device_user', port: 1883 } }

      mockApi.patchDeviceOverridesNew.mockResolvedValue(patched)

      await store.patchOverrides(123, patch)

      expect(mockApi.patchDeviceOverridesNew).toHaveBeenCalledWith(123, patch)
      expect(store.overrides).toEqual(patched)
    })

    it('should handle errors when patching overrides', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Invalid patch'

      mockApi.patchDeviceOverridesNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.patchOverrides(123, {})).rejects.toThrow(errorMessage)

      expect(store.error).toBe(errorMessage)
    })
  })

  describe('clearOverrides', () => {
    it('should clear overrides successfully', async () => {
      const store = useDeviceConfigNewStore()
      store.overrides = { mqtt: { user: 'device_user' } }

      mockApi.deleteDeviceOverridesNew.mockResolvedValue(undefined)

      await store.clearOverrides(123)

      expect(mockApi.deleteDeviceOverridesNew).toHaveBeenCalledWith(123)
      expect(store.overrides).toBeNull()
    })

    it('should handle errors when clearing overrides', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Device not found'

      mockApi.deleteDeviceOverridesNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.clearOverrides(123)).rejects.toThrow(errorMessage)

      expect(store.error).toBe(errorMessage)
    })
  })

  describe('fetchDesired', () => {
    it('should fetch desired config successfully', async () => {
      const store = useDeviceConfigNewStore()

      mockApi.getDesiredConfigNew.mockResolvedValue(mockDesiredConfigData)

      const result = await store.fetchDesired(123)

      expect(mockApi.getDesiredConfigNew).toHaveBeenCalledWith(123)
      expect(store.desiredConfig).toEqual(mockDesiredConfigData.config)
      expect(store.sources).toEqual(mockDesiredConfigData.sources)
      expect(result).toEqual(mockDesiredConfigData)
    })

    it('should handle missing sources gracefully', async () => {
      const store = useDeviceConfigNewStore()
      const dataWithoutSources = {
        config: mockDesiredConfigData.config,
      }

      mockApi.getDesiredConfigNew.mockResolvedValue(dataWithoutSources as any)

      await store.fetchDesired(123)

      expect(store.sources).toEqual({})
    })
  })

  describe('fetchStatus', () => {
    it('should fetch status successfully', async () => {
      const store = useDeviceConfigNewStore()

      mockApi.getConfigStatusNew.mockResolvedValue(mockStatusData)

      const result = await store.fetchStatus(123)

      expect(mockApi.getConfigStatusNew).toHaveBeenCalledWith(123)
      expect(store.status).toEqual(mockStatusData)
      expect(result).toEqual(mockStatusData)
    })

    it('should handle errors when fetching status', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Device offline'

      mockApi.getConfigStatusNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.fetchStatus(123)).rejects.toThrow(errorMessage)

      expect(store.error).toBe(errorMessage)
    })
  })

  describe('apply', () => {
    it('should apply config successfully', async () => {
      const store = useDeviceConfigNewStore()

      mockApi.applyDeviceConfigNew.mockResolvedValue(mockApplyData)
      mockApi.getConfigStatusNew.mockResolvedValue(mockStatusData)

      const result = await store.apply(123)

      expect(mockApi.applyDeviceConfigNew).toHaveBeenCalledWith(123)
      expect(mockApi.getConfigStatusNew).toHaveBeenCalledWith(123)
      expect(store.lastApply).toEqual(mockApplyData)
      expect(store.status).toEqual(mockStatusData)
      expect(result).toEqual(mockApplyData)
    })

    it('should handle errors when applying config', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Apply failed'

      mockApi.applyDeviceConfigNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.apply(123)).rejects.toThrow(errorMessage)

      expect(store.error).toBe(errorMessage)
    })
  })

  describe('verify', () => {
    it('should verify config successfully', async () => {
      const store = useDeviceConfigNewStore()

      mockApi.verifyDeviceConfigNew.mockResolvedValue(mockVerifyData)

      const result = await store.verify(123)

      expect(mockApi.verifyDeviceConfigNew).toHaveBeenCalledWith(123)
      expect(store.lastVerify).toEqual(mockVerifyData)
      expect(result).toEqual(mockVerifyData)
    })

    it('should handle verification with differences', async () => {
      const store = useDeviceConfigNewStore()
      const verifyWithDiff: ConfigVerifyData = {
        matches: false,
        differences: [
          { path: 'mqtt.user', expected: 'device_user', actual: 'iot' },
        ],
      }

      mockApi.verifyDeviceConfigNew.mockResolvedValue(verifyWithDiff)

      const result = await store.verify(123)

      expect(store.lastVerify).toEqual(verifyWithDiff)
      expect(result.matches).toBe(false)
      expect(result.differences).toHaveLength(1)
    })

    it('should handle errors when verifying config', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Device unreachable'

      mockApi.verifyDeviceConfigNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.verify(123)).rejects.toThrow(errorMessage)

      expect(store.error).toBe(errorMessage)
    })
  })

  describe('reset', () => {
    it('should reset all state to initial values', () => {
      const store = useDeviceConfigNewStore()

      // Set some state
      store.templates = [mockTemplate]
      store.templateIds = [1]
      store.overrides = { mqtt: { user: 'test' } }
      store.desiredConfig = { mqtt: { enabled: true } }
      store.sources = { 'mqtt.enabled': 'template:1' }
      store.status = mockStatusData
      store.lastApply = mockApplyData
      store.lastVerify = mockVerifyData
      store.loading = true
      store.error = 'Some error'

      store.reset()

      expect(store.templates).toEqual([])
      expect(store.templateIds).toEqual([])
      expect(store.overrides).toBeNull()
      expect(store.desiredConfig).toBeNull()
      expect(store.sources).toEqual({})
      expect(store.status).toBeNull()
      expect(store.lastApply).toBeNull()
      expect(store.lastVerify).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })
  })

  describe('an action that settles after a reset', () => {
    // An action starts for device 7 and stays pending. The page resets the store and starts a
    // refresh for device 8, which also stays pending. The old action then settles. The store now
    // belongs to device 8, so the old action must hand its answer back to its caller and write
    // nothing: not its answer, not its error, and not its loading flag, which the refresh holds.
    function startActionThenResetThenRefreshOtherDevice() {
      const store = useDeviceConfigNewStore()

      let settleOld!: { resolve: (v: ConfigTemplate[]) => void; reject: (e: Error) => void }
      mockApi.addDeviceTemplateNew.mockReturnValue(
        new Promise<ConfigTemplate[]>((resolve, reject) => {
          settleOld = { resolve, reject }
        }),
      )
      const oldAction = store.addTemplate(7, 1)
      expect(store.loading).toBe(true)

      store.reset()

      const pending = () => new Promise<never>(() => {})
      mockApi.getDeviceTemplatesNew.mockReturnValue(pending())
      mockApi.getDeviceOverridesNew.mockReturnValue(pending())
      mockApi.getDesiredConfigNew.mockReturnValue(pending())
      mockApi.getConfigStatusNew.mockReturnValue(pending())
      void store.refresh(8)
      expect(store.loading).toBe(true)

      return { store, oldAction, settleOld }
    }

    it('keeps the store as it is when the old action succeeds late', async () => {
      const { store, oldAction, settleOld } = startActionThenResetThenRefreshOtherDevice()
      const before = {
        templates: [...store.templates],
        templateIds: [...store.templateIds],
        error: store.error,
      }

      settleOld.resolve([mockTemplate])
      await oldAction

      expect(store.loading).toBe(true)
      expect(store.templates).toEqual(before.templates)
      expect(store.templateIds).toEqual(before.templateIds)
      expect(store.error).toBe(before.error)
    })

    it('keeps the store as it is when the old action fails late', async () => {
      const { store, oldAction, settleOld } = startActionThenResetThenRefreshOtherDevice()
      const before = {
        templates: [...store.templates],
        templateIds: [...store.templateIds],
        error: store.error,
      }

      settleOld.reject(new Error('late failure of device 7'))
      // The caller still gets the failure of its own action.
      await expect(oldAction).rejects.toThrow('late failure of device 7')

      expect(store.loading).toBe(true)
      expect(store.templates).toEqual(before.templates)
      expect(store.templateIds).toEqual(before.templateIds)
      expect(store.error).toBe(before.error)
    })
  })

  describe('Error handling', () => {
    it('should clear previous errors on new operations', async () => {
      const store = useDeviceConfigNewStore()
      store.error = 'Previous error'

      mockApi.getDeviceTemplatesNew.mockResolvedValue(mockTemplatesData)
      mockApi.getDeviceOverridesNew.mockResolvedValue(mockOverridesData)
      mockApi.getDesiredConfigNew.mockResolvedValue(mockDesiredConfigData)
      mockApi.getConfigStatusNew.mockResolvedValue(mockStatusData)

      await store.refresh(123)

      expect(store.error).toBeNull()
    })

    it('should preserve error message on failure', async () => {
      const store = useDeviceConfigNewStore()
      const errorMessage = 'Custom error message'

      mockApi.addDeviceTemplateNew.mockRejectedValue(new Error(errorMessage))

      await expect(store.addTemplate(123, 2)).rejects.toThrow()

      expect(store.error).toBe(errorMessage)
    })

    it('should handle errors without message property', async () => {
      const store = useDeviceConfigNewStore()

      mockApi.getDeviceTemplatesNew.mockRejectedValue({ code: 500 })

      await expect(store.refresh(123)).rejects.toThrow()

      expect(store.error).toBe('Failed to load new config state')
    })
  })

  describe('Loading state management', () => {
    it('should manage loading state for all async operations', async () => {
      const store = useDeviceConfigNewStore()

      const operations = [
        () => {
          mockApi.getDeviceTemplatesNew.mockResolvedValue(mockTemplatesData)
          mockApi.getDeviceOverridesNew.mockResolvedValue(mockOverridesData)
          mockApi.getDesiredConfigNew.mockResolvedValue(mockDesiredConfigData)
          mockApi.getConfigStatusNew.mockResolvedValue(mockStatusData)
          return store.refresh(123)
        },
        () => {
          mockApi.addDeviceTemplateNew.mockResolvedValue([mockTemplate])
          return store.addTemplate(123, 1)
        },
        () => {
          mockApi.removeDeviceTemplateNew.mockResolvedValue([mockTemplate])
          return store.removeTemplate(123, 1)
        },
        () => {
          mockApi.setDeviceTemplatesNew.mockResolvedValue(mockTemplatesData)
          return store.setTemplates(123, [1])
        },
        () => {
          mockApi.setDeviceOverridesNew.mockResolvedValue(mockOverridesData)
          return store.saveOverrides(123, {})
        },
        () => {
          mockApi.patchDeviceOverridesNew.mockResolvedValue({})
          return store.patchOverrides(123, {})
        },
        () => {
          mockApi.deleteDeviceOverridesNew.mockResolvedValue(undefined)
          return store.clearOverrides(123)
        },
        () => {
          mockApi.getDesiredConfigNew.mockResolvedValue(mockDesiredConfigData)
          return store.fetchDesired(123)
        },
        () => {
          mockApi.getConfigStatusNew.mockResolvedValue(mockStatusData)
          return store.fetchStatus(123)
        },
        () => {
          mockApi.applyDeviceConfigNew.mockResolvedValue(mockApplyData)
          mockApi.getConfigStatusNew.mockResolvedValue(mockStatusData)
          return store.apply(123)
        },
        () => {
          mockApi.verifyDeviceConfigNew.mockResolvedValue(mockVerifyData)
          return store.verify(123)
        },
      ]

      for (const operation of operations) {
        expect(store.loading).toBe(false)
        const promise = operation()
        expect(store.loading).toBe(true)
        await promise
        expect(store.loading).toBe(false)
      }
    })
  })
})
