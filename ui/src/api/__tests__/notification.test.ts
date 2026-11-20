import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the axios client
vi.mock('../client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn()
    }
  }
})

import api from '../client'
import {
  getChannels,
  getChannel,
  createChannel,
  updateChannel,
  setChannelEnabled,
  deleteChannel,
  testChannel,
  getRules,
  createRule,
  deleteRule,
  getHistory
} from '../notification'

// The fixtures below are what the server sends (internal/notification models and handlers): snake_case
// fields and numeric ids. A fixture in the pages' own shape is how ginsys/shelly-manager#250 hid.
const wireChannel = (over: Record<string, unknown> = {}) => ({
  id: 1,
  name: 'Email Channel',
  type: 'email',
  enabled: true,
  config: { recipients: ['ops@example.com'] },
  description: 'Ops mail',
  created_at: '2023-01-01T00:00:00Z',
  updated_at: '2023-01-02T00:00:00Z',
  ...over
})

const wireRule = (over: Record<string, unknown> = {}) => ({
  id: 7,
  name: 'Critical Drift',
  description: '',
  enabled: true,
  channel_id: 1,
  alert_level: 'critical',
  min_severity: 'warning',
  categories: ['security', 'device'],
  created_at: '2023-01-01T00:00:00Z',
  updated_at: '2023-01-01T00:00:00Z',
  ...over
})

const ok = (data: unknown, meta?: unknown) => ({
  data: { success: true, data, meta, timestamp: '2023-01-01T00:00:00Z' }
})

describe('notification api', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
    ;(api.delete as any).mockReset()
  })

  describe('channels', () => {
    it('getChannels maps the server shape to the page model', async () => {
      ;(api.get as any).mockResolvedValue(ok({ channels: [wireChannel()], total: 1 }))

      const channels = await getChannels()
      expect(channels).toEqual([
        {
          id: '1',
          name: 'Email Channel',
          type: 'email',
          enabled: true,
          config: { recipients: ['ops@example.com'] },
          description: 'Ops mail',
          createdAt: '2023-01-01T00:00:00Z',
          updatedAt: '2023-01-02T00:00:00Z'
        }
      ])
      expect(api.get).toHaveBeenCalledWith('/notifications/channels')
    })

    it('getChannel reads one channel by its numeric id', async () => {
      ;(api.get as any).mockResolvedValue(ok(wireChannel({ id: 3, type: 'slack', config: null })))

      const channel = await getChannel('3')
      expect(channel.id).toBe('3')
      expect(channel.type).toBe('slack')
      expect(channel.config).toEqual({})
      expect(api.get).toHaveBeenCalledWith('/notifications/channels/3')
    })

    it('getChannel refuses an id that is not a whole number before any request', async () => {
      await expect(getChannel('c1')).rejects.toThrow('Invalid notification id')
      expect(api.get).not.toHaveBeenCalled()
    })

    it('createChannel sends the server fields and maps the answer', async () => {
      ;(api.post as any).mockResolvedValue(ok(wireChannel({ id: 2, name: 'Hook', type: 'webhook' })))

      const channel = await createChannel({ name: 'Hook', type: 'webhook', enabled: false, config: { url: 'https://x' } })
      expect(channel.id).toBe('2')
      expect(api.post).toHaveBeenCalledWith('/notifications/channels', {
        name: 'Hook',
        type: 'webhook',
        enabled: false,
        config: { url: 'https://x' },
        description: ''
      })
    })

    // A refused create is a 400 that axios throws: the server's reason is in error.details, and error.message
    // is the fixed "Invalid channel configuration" (internal/notification/handlers.go CreateChannel).
    it('createChannel throws the server\'s reason for a refused config', async () => {
      ;(api.post as any).mockRejectedValue({
        message: 'Request failed with status code 400',
        response: {
          status: 400,
          data: {
            success: false,
            error: {
              code: 'VALIDATION_FAILED',
              message: 'Invalid channel configuration',
              details: 'invalid channel configuration: webhook config must have a URL'
            }
          }
        }
      })

      await expect(createChannel({ name: 'Hook', type: 'webhook', enabled: true, config: {} })).rejects.toThrow(
        'invalid channel configuration: webhook config must have a URL'
      )
    })

    it('createChannel falls back to the envelope message, then to the transport message', async () => {
      ;(api.post as any).mockRejectedValueOnce({
        message: 'Request failed with status code 400',
        response: { status: 400, data: { success: false, error: { message: 'Invalid JSON request body' } } }
      })
      await expect(createChannel({ name: 'Hook', type: 'webhook', enabled: true, config: {} })).rejects.toThrow(
        'Invalid JSON request body'
      )

      ;(api.post as any).mockRejectedValueOnce(new Error('Network Error'))
      await expect(createChannel({ name: 'Hook', type: 'webhook', enabled: true, config: {} })).rejects.toThrow(
        'Network Error'
      )
    })

    it('setChannelEnabled reads the channel and sends it back complete with only enabled changed', async () => {
      ;(api.get as any).mockResolvedValue(
        ok(wireChannel({ type: 'webhook', config: { url: '********', secret: '********' } }))
      )
      ;(api.put as any).mockResolvedValue(ok({ status: 'updated' }))

      await setChannelEnabled('1', false)

      expect(api.get).toHaveBeenCalledWith('/notifications/channels/1')
      expect(api.put).toHaveBeenCalledWith('/notifications/channels/1', {
        name: 'Email Channel',
        type: 'webhook',
        enabled: false,
        config: { url: '********', secret: '********' },
        description: 'Ops mail'
      })
    })

    // ginsys/shelly-manager#783: the edit form calls updateChannel with the config it built.
    it('updateChannel sends the whole channel to its numeric id', async () => {
      ;(api.put as any).mockResolvedValue(ok({ status: 'updated' }))

      await updateChannel('5', {
        name: 'Hook',
        type: 'webhook',
        enabled: true,
        config: { url: '********', headers: '********', method: 'POST' },
        description: 'Ops hook'
      })

      expect(api.put).toHaveBeenCalledWith('/notifications/channels/5', {
        name: 'Hook',
        type: 'webhook',
        enabled: true,
        config: { url: '********', headers: '********', method: 'POST' },
        description: 'Ops hook'
      })
    })

    // A refused update is the same 400 as a refused create (internal/notification/handlers.go UpdateChannel):
    // the reason is in error.details and error.message is the fixed "Invalid channel update".
    it('updateChannel throws the server\'s reason for a refused config', async () => {
      ;(api.put as any).mockRejectedValue({
        message: 'Request failed with status code 400',
        response: {
          status: 400,
          data: {
            success: false,
            error: {
              code: 'VALIDATION_FAILED',
              message: 'Invalid channel update',
              details: 'invalid channel configuration: email config must have at least one recipient'
            }
          }
        }
      })

      await expect(
        updateChannel('1', { name: 'Mail', type: 'email', enabled: true, config: { recipients: [] } })
      ).rejects.toThrow('invalid channel configuration: email config must have at least one recipient')
    })

    it('updateChannel falls back to the envelope message, then to the transport message', async () => {
      ;(api.put as any).mockRejectedValueOnce({
        message: 'Request failed with status code 400',
        response: { status: 400, data: { success: false, error: { message: 'Invalid JSON request body' } } }
      })
      await expect(
        updateChannel('1', { name: 'Mail', type: 'email', enabled: true, config: {} })
      ).rejects.toThrow('Invalid JSON request body')

      ;(api.put as any).mockRejectedValueOnce(new Error('Network Error'))
      await expect(
        updateChannel('1', { name: 'Mail', type: 'email', enabled: true, config: {} })
      ).rejects.toThrow('Network Error')
    })

    it('updateChannel refuses an id that is not a whole number before any request', async () => {
      await expect(updateChannel('c1', { name: 'x', type: 'email', enabled: true, config: {} })).rejects.toThrow(
        'Invalid notification id'
      )
      expect(api.put).not.toHaveBeenCalled()
    })

    // setChannelEnabled goes through the same function, so a refused toggle now says why as well.
    it('setChannelEnabled throws the server\'s reason when the update is refused', async () => {
      ;(api.get as any).mockResolvedValue(ok(wireChannel()))
      ;(api.put as any).mockRejectedValue({
        message: 'Request failed with status code 400',
        response: {
          status: 400,
          data: { success: false, error: { message: 'Invalid channel update', details: 'invalid channel configuration: x' } }
        }
      })

      await expect(setChannelEnabled('1', false)).rejects.toThrow('invalid channel configuration: x')
    })

    it('setChannelEnabled sends nothing when the read fails', async () => {
      ;(api.get as any).mockResolvedValue({ data: { success: false, error: { message: 'gone' } } })

      await expect(setChannelEnabled('1', true)).rejects.toThrow('gone')
      expect(api.put).not.toHaveBeenCalled()
    })

    it('deleteChannel deletes a channel', async () => {
      ;(api.delete as any).mockResolvedValue(ok({ status: 'deleted' }))

      await deleteChannel('1')
      expect(api.delete).toHaveBeenCalledWith('/notifications/channels/1')
    })

    it('testChannel posts to the test route and returns the server message', async () => {
      ;(api.post as any).mockResolvedValue(ok({ status: 'sent', message: 'Test notification sent successfully' }))

      await expect(testChannel('4')).resolves.toBe('Test notification sent successfully')
      expect(api.post).toHaveBeenCalledWith('/notifications/channels/4/test')
    })

    it('testChannel throws the server message on failure', async () => {
      ;(api.post as any).mockResolvedValue({ data: { success: false, error: { message: 'webhook request failed' } } })

      await expect(testChannel('4')).rejects.toThrow('webhook request failed')
    })

    it('getChannels throws error on API failure', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Database error' }
        }
      })

      await expect(getChannels()).rejects.toThrow('Database error')
    })
  })

  describe('rules', () => {
    it('getRules maps the server shape (alert level, severity, categories) to the page model', async () => {
      ;(api.get as any).mockResolvedValue(
        ok({ rules: [wireRule(), wireRule({ id: 8, categories: null, min_severity: '' })], total: 2 })
      )

      const rules = await getRules()
      expect(rules[0]).toEqual({
        id: '7',
        name: 'Critical Drift',
        description: '',
        channelId: '1',
        enabled: true,
        alertLevel: 'critical',
        minSeverity: 'warning',
        categories: ['security', 'device'],
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      })
      expect(rules[1].categories).toEqual([])
      expect(rules[1].minSeverity).toBe('')
      expect(api.get).toHaveBeenCalledWith('/notifications/rules')
    })

    it('createRule sends the server fields with a numeric channel_id', async () => {
      ;(api.post as any).mockResolvedValue(ok(wireRule({ id: 9 })))

      const rule = await createRule({
        name: 'Critical Drift',
        channelId: '1',
        enabled: false,
        alertLevel: 'all',
        minSeverity: '',
        categories: ['security']
      })
      expect(rule.id).toBe('9')
      expect(api.post).toHaveBeenCalledWith('/notifications/rules', {
        name: 'Critical Drift',
        description: '',
        enabled: false,
        channel_id: 1,
        alert_level: 'all',
        min_severity: '',
        categories: ['security']
      })
    })

    it('createRule refuses a channel id that is not a whole number before any request', async () => {
      await expect(
        createRule({ name: 'r', channelId: '', enabled: true, alertLevel: 'all', minSeverity: '', categories: [] })
      ).rejects.toThrow('Invalid notification id')
      expect(api.post).not.toHaveBeenCalled()
    })

    it('deleteRule deletes a rule', async () => {
      ;(api.delete as any).mockResolvedValue(ok({ status: 'deleted' }))

      await deleteRule('7')
      expect(api.delete).toHaveBeenCalledWith('/notifications/rules/7')
    })
  })

  describe('history', () => {
    const wireHistory = (over: Record<string, unknown> = {}) => ({
      id: 10,
      rule_id: 7,
      channel_id: 1,
      trigger_type: 'drift_detected',
      status: 'sent',
      sent_at: '2023-01-01T00:00:10Z',
      created_at: '2023-01-01T00:00:00Z',
      ...over
    })

    it('getHistory maps rows and reads the page and the total from meta', async () => {
      const pagination = { page: 1, page_size: 50, total_pages: 3, has_next: true, has_previous: false }
      ;(api.get as any).mockResolvedValue(
        ok({ history: [wireHistory()] }, { count: 1, total_count: 120, pagination })
      )

      const page = await getHistory()
      expect(page.items).toEqual([
        {
          id: '10',
          channelId: '1',
          ruleId: '7',
          triggerType: 'drift_detected',
          status: 'sent',
          sentAt: '2023-01-01T00:00:10Z',
          createdAt: '2023-01-01T00:00:00Z',
          error: undefined
        }
      ])
      expect(page.pagination).toEqual(pagination)
      expect(page.total).toBe(120)
      expect(api.get).toHaveBeenCalledWith('/notifications/history', { params: { limit: 50, offset: 0 } })
    })

    it('getHistory sends limit and offset, not page', async () => {
      ;(api.get as any).mockResolvedValue(ok({ history: [] }, { total_count: 0 }))

      await getHistory({ page: 3, limit: 25 })
      expect(api.get).toHaveBeenCalledWith('/notifications/history', { params: { limit: 25, offset: 50 } })
    })

    it('getHistory keeps a failed row with its error, and a pending row has no sent time', async () => {
      ;(api.get as any).mockResolvedValue(
        ok({
          history: [
            wireHistory({ id: 11, status: 'failed', error: 'SMTP connection timeout' }),
            wireHistory({ id: 12, status: 'pending', sent_at: undefined })
          ]
        })
      )

      const { items, total } = await getHistory()
      expect(items[0].status).toBe('failed')
      expect(items[0].error).toBe('SMTP connection timeout')
      expect(items[1].sentAt).toBeUndefined()
      expect(items[1].createdAt).toBe('2023-01-01T00:00:00Z')
      expect(total).toBe(2)
    })
  })
})
