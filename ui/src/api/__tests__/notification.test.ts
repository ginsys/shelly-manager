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
  deleteChannel,
  getRules,
  createRule,
  deleteRule,
  getHistory,
  type NotificationChannel,
  type NotificationRule,
  type NotificationHistory
} from '../notification'

describe('notification api', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
    ;(api.put as any).mockReset()
    ;(api.delete as any).mockReset()
  })

  describe('channels', () => {
    it('getChannels returns list of channels', async () => {
      const mockChannels: NotificationChannel[] = [
        {
          id: '1',
          name: 'Email Channel',
          type: 'email',
          config: { smtp_host: 'smtp.example.com' },
          enabled: true,
          createdAt: '2023-01-01T00:00:00Z',
          updatedAt: '2023-01-01T00:00:00Z'
        }
      ]

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { channels: mockChannels },
          timestamp: new Date().toISOString()
        }
      })

      const channels = await getChannels()
      expect(channels).toEqual(mockChannels)
      expect(api.get).toHaveBeenCalledWith('/notifications/channels')
    })

    it('getChannel returns a single channel', async () => {
      const mockChannel: NotificationChannel = {
        id: '1',
        name: 'Slack Channel',
        type: 'slack',
        config: { webhook_url: 'https://hooks.slack.com/...' },
        enabled: true,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: mockChannel,
          timestamp: new Date().toISOString()
        }
      })

      const channel = await getChannel('1')
      expect(channel).toEqual(mockChannel)
      expect(api.get).toHaveBeenCalledWith('/notifications/channels/1')
    })

    it('createChannel creates a new channel', async () => {
      const newChannel: Partial<NotificationChannel> = {
        name: 'Webhook Channel',
        type: 'webhook',
        config: { url: 'https://example.com/webhook' },
        enabled: true
      }

      const createdChannel: NotificationChannel = {
        id: '2',
        ...newChannel,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      } as NotificationChannel

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: createdChannel,
          timestamp: new Date().toISOString()
        }
      })

      const channel = await createChannel(newChannel)
      expect(channel).toEqual(createdChannel)
      expect(api.post).toHaveBeenCalledWith('/notifications/channels', newChannel)
    })

    it('updateChannel updates an existing channel', async () => {
      const updates: Partial<NotificationChannel> = {
        name: 'Updated Channel',
        enabled: false
      }

      const updatedChannel: NotificationChannel = {
        id: '1',
        name: 'Updated Channel',
        type: 'email',
        config: {},
        enabled: false,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-02T00:00:00Z'
      }

      ;(api.put as any).mockResolvedValue({
        data: {
          success: true,
          data: updatedChannel,
          timestamp: new Date().toISOString()
        }
      })

      const channel = await updateChannel('1', updates)
      expect(channel).toEqual(updatedChannel)
      expect(api.put).toHaveBeenCalledWith('/notifications/channels/1', updates)
    })

    it('deleteChannel deletes a channel', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: {
          success: true,
          timestamp: new Date().toISOString()
        }
      })

      await deleteChannel('1')
      expect(api.delete).toHaveBeenCalledWith('/notifications/channels/1')
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
    it('getRules returns list of rules', async () => {
      const mockRules: NotificationRule[] = [
        {
          id: '1',
          name: 'Device Offline Alert',
          channelId: '1',
          eventTypes: ['device.offline'],
          filters: {},
          enabled: true,
          createdAt: '2023-01-01T00:00:00Z',
          updatedAt: '2023-01-01T00:00:00Z'
        }
      ]

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { rules: mockRules },
          timestamp: new Date().toISOString()
        }
      })

      const rules = await getRules()
      expect(rules).toEqual(mockRules)
      expect(api.get).toHaveBeenCalledWith('/notifications/rules')
    })

    it('createRule creates a new rule', async () => {
      const newRule: Partial<NotificationRule> = {
        name: 'Export Failed Alert',
        channelId: '1',
        eventTypes: ['export.failed'],
        filters: {},
        enabled: true
      }

      const createdRule: NotificationRule = {
        id: '2',
        ...newRule,
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      } as NotificationRule

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: createdRule,
          timestamp: new Date().toISOString()
        }
      })

      const rule = await createRule(newRule)
      expect(rule).toEqual(createdRule)
      expect(api.post).toHaveBeenCalledWith('/notifications/rules', newRule)
    })

    it('deleteRule deletes a rule', async () => {
      ;(api.delete as any).mockResolvedValue({
        data: {
          success: true,
          timestamp: new Date().toISOString()
        }
      })

      await deleteRule('1')
      expect(api.delete).toHaveBeenCalledWith('/notifications/rules/1')
    })
  })

  describe('history', () => {
    it('getHistory returns list of notification history', async () => {
      const mockHistory: NotificationHistory[] = [
        {
          id: '1',
          channelId: '1',
          ruleId: '1',
          eventType: 'device.offline',
          status: 'sent',
          sentAt: '2023-01-01T00:00:00Z'
        }
      ]

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { history: mockHistory },
          timestamp: new Date().toISOString()
        }
      })

      const history = await getHistory()
      expect(history).toEqual(mockHistory)
      expect(api.get).toHaveBeenCalledWith('/notifications/history', { params: {} })
    })

    it('getHistory supports pagination', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { history: [] },
          timestamp: new Date().toISOString()
        }
      })

      await getHistory({ page: 2, limit: 50 })
      expect(api.get).toHaveBeenCalledWith('/notifications/history', {
        params: { page: 2, limit: 50 }
      })
    })

    it('getHistory includes failed status and error', async () => {
      const mockHistory: NotificationHistory[] = [
        {
          id: '2',
          channelId: '1',
          ruleId: '1',
          eventType: 'export.failed',
          status: 'failed',
          sentAt: '2023-01-01T00:00:00Z',
          error: 'SMTP connection timeout'
        }
      ]

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { history: mockHistory },
          timestamp: new Date().toISOString()
        }
      })

      const history = await getHistory()
      expect(history[0].status).toBe('failed')
      expect(history[0].error).toBe('SMTP connection timeout')
    })
  })
})
