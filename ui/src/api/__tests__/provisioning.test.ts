import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the axios client
vi.mock('../client', () => {
  return {
    default: {
      get: vi.fn(),
      post: vi.fn()
    }
  }
})

import api from '../client'
import {
  getTasks,
  getTask,
  createTask,
  cancelTask,
  bulkProvision,
  getAgents,
  getAgentStatus,
  type ProvisioningTask,
  type ProvisioningAgent
} from '../provisioning'

describe('provisioning api', () => {
  beforeEach(() => {
    ;(api.get as any).mockReset()
    ;(api.post as any).mockReset()
  })

  describe('tasks', () => {
    it('getTasks returns list of tasks', async () => {
      const mockTasks: ProvisioningTask[] = [
        {
          id: '1',
          deviceId: 'dev-1',
          deviceName: 'Device 1',
          status: 'completed',
          taskType: 'configure',
          config: { setting: 'value' },
          createdAt: '2023-01-01T00:00:00Z',
          updatedAt: '2023-01-01T00:01:00Z'
        }
      ]

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { tasks: mockTasks },
          timestamp: new Date().toISOString()
        }
      })

      const tasks = await getTasks()
      expect(tasks).toEqual(mockTasks)
      expect(api.get).toHaveBeenCalledWith('/provisioning/tasks', { params: undefined })
    })

    it('getTasks supports filtering by status', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { tasks: [] },
          timestamp: new Date().toISOString()
        }
      })

      await getTasks({ status: 'pending', page: 2, limit: 50 })
      expect(api.get).toHaveBeenCalledWith('/provisioning/tasks', {
        params: { status: 'pending', page: 2, limit: 50 }
      })
    })

    it('getTask returns a single task', async () => {
      const mockTask: ProvisioningTask = {
        id: '1',
        deviceId: 'dev-1',
        deviceName: 'Device 1',
        status: 'running',
        taskType: 'update',
        config: {},
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: mockTask,
          timestamp: new Date().toISOString()
        }
      })

      const task = await getTask('1')
      expect(task).toEqual(mockTask)
      expect(api.get).toHaveBeenCalledWith('/provisioning/tasks/1')
    })

    it('createTask creates a new task', async () => {
      const newTask: Partial<ProvisioningTask> = {
        deviceId: 'dev-2',
        taskType: 'configure',
        config: { key: 'value' }
      }

      const createdTask: ProvisioningTask = {
        id: '2',
        deviceId: 'dev-2',
        deviceName: 'Device 2',
        status: 'pending',
        taskType: 'configure',
        config: { key: 'value' },
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:00:00Z'
      }

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: createdTask,
          timestamp: new Date().toISOString()
        }
      })

      const task = await createTask(newTask)
      expect(task).toEqual(createdTask)
      expect(api.post).toHaveBeenCalledWith('/provisioning/tasks', newTask)
    })

    it('cancelTask cancels a running task', async () => {
      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          timestamp: new Date().toISOString()
        }
      })

      await cancelTask('1')
      expect(api.post).toHaveBeenCalledWith('/provisioning/tasks/1/cancel')
    })

    it('getTasks throws error on API failure', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: false,
          error: { message: 'Database error' }
        }
      })

      await expect(getTasks()).rejects.toThrow('Database error')
    })
  })

  describe('bulk operations', () => {
    it('bulkProvision creates multiple tasks', async () => {
      const request = {
        deviceIds: ['dev-1', 'dev-2', 'dev-3'],
        config: { setting: 'value' }
      }

      const mockTasks: ProvisioningTask[] = [
        {
          id: '1',
          deviceId: 'dev-1',
          deviceName: 'Device 1',
          status: 'pending',
          taskType: 'configure',
          config: { setting: 'value' },
          createdAt: '2023-01-01T00:00:00Z',
          updatedAt: '2023-01-01T00:00:00Z'
        },
        {
          id: '2',
          deviceId: 'dev-2',
          deviceName: 'Device 2',
          status: 'pending',
          taskType: 'configure',
          config: { setting: 'value' },
          createdAt: '2023-01-01T00:00:00Z',
          updatedAt: '2023-01-01T00:00:00Z'
        },
        {
          id: '3',
          deviceId: 'dev-3',
          deviceName: 'Device 3',
          status: 'pending',
          taskType: 'configure',
          config: { setting: 'value' },
          createdAt: '2023-01-01T00:00:00Z',
          updatedAt: '2023-01-01T00:00:00Z'
        }
      ]

      ;(api.post as any).mockResolvedValue({
        data: {
          success: true,
          data: { tasks: mockTasks },
          timestamp: new Date().toISOString()
        }
      })

      const tasks = await bulkProvision(request)
      expect(tasks).toHaveLength(3)
      expect(tasks).toEqual(mockTasks)
      expect(api.post).toHaveBeenCalledWith('/provisioning/bulk', request)
    })
  })

  describe('agents', () => {
    it('getAgents returns list of agents', async () => {
      const mockAgents: ProvisioningAgent[] = [
        {
          id: 'agent-1',
          name: 'Agent 1',
          status: 'online',
          version: '1.0.0',
          capabilities: ['configure', 'update'],
          lastSeen: '2023-01-01T00:00:00Z'
        }
      ]

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { agents: mockAgents },
          timestamp: new Date().toISOString()
        }
      })

      const agents = await getAgents()
      expect(agents).toEqual(mockAgents)
      expect(api.get).toHaveBeenCalledWith('/provisioning/agents')
    })

    it('getAgentStatus returns agent status', async () => {
      const mockAgent: ProvisioningAgent = {
        id: 'agent-1',
        name: 'Agent 1',
        status: 'busy',
        version: '1.0.0',
        capabilities: ['configure', 'update', 'restart'],
        lastSeen: '2023-01-01T00:05:00Z'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: mockAgent,
          timestamp: new Date().toISOString()
        }
      })

      const agent = await getAgentStatus('agent-1')
      expect(agent).toEqual(mockAgent)
      expect(api.get).toHaveBeenCalledWith('/provisioning/agents/agent-1/status')
    })

    it('getAgents handles empty list', async () => {
      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: { agents: [] },
          timestamp: new Date().toISOString()
        }
      })

      const agents = await getAgents()
      expect(agents).toEqual([])
    })
  })

  describe('error handling', () => {
    it('handles task with error field', async () => {
      const taskWithError: ProvisioningTask = {
        id: '1',
        deviceId: 'dev-1',
        deviceName: 'Device 1',
        status: 'failed',
        taskType: 'configure',
        config: {},
        error: 'Device unreachable',
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:01:00Z'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: taskWithError,
          timestamp: new Date().toISOString()
        }
      })

      const task = await getTask('1')
      expect(task.status).toBe('failed')
      expect(task.error).toBe('Device unreachable')
    })

    it('handles task with result field', async () => {
      const taskWithResult: ProvisioningTask = {
        id: '1',
        deviceId: 'dev-1',
        deviceName: 'Device 1',
        status: 'completed',
        taskType: 'configure',
        config: {},
        result: { success: true, configuredSettings: 5 },
        createdAt: '2023-01-01T00:00:00Z',
        updatedAt: '2023-01-01T00:01:00Z'
      }

      ;(api.get as any).mockResolvedValue({
        data: {
          success: true,
          data: taskWithResult,
          timestamp: new Date().toISOString()
        }
      })

      const task = await getTask('1')
      expect(task.status).toBe('completed')
      expect(task.result).toEqual({ success: true, configuredSettings: 5 })
    })
  })
})
