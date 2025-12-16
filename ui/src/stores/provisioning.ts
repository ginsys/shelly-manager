import { defineStore } from 'pinia'
import {
  getTasks,
  getTask,
  createTask,
  cancelTask,
  bulkProvision,
  getAgents,
  getAgentStatus,
  type ProvisioningTask,
  type ProvisioningAgent,
  type GetTasksParams,
  type BulkProvisionRequest
} from '@/api/provisioning'

export const useProvisioningStore = defineStore('provisioning', {
  state: () => ({
    // Tasks
    tasks: [] as ProvisioningTask[],
    currentTask: null as ProvisioningTask | null,
    tasksLoading: false,
    tasksError: '' as string,
    tasksPage: 1,
    tasksLimit: 50,
    tasksStatusFilter: '' as string,

    // Agents
    agents: [] as ProvisioningAgent[],
    agentsLoading: false,
    agentsError: '' as string
  }),

  getters: {
    pendingTasks: (state) => state.tasks.filter((t) => t.status === 'pending'),
    runningTasks: (state) => state.tasks.filter((t) => t.status === 'running'),
    completedTasks: (state) => state.tasks.filter((t) => t.status === 'completed'),
    failedTasks: (state) => state.tasks.filter((t) => t.status === 'failed'),

    onlineAgents: (state) => state.agents.filter((a) => a.status === 'online'),
    busyAgents: (state) => state.agents.filter((a) => a.status === 'busy'),
    offlineAgents: (state) => state.agents.filter((a) => a.status === 'offline'),

    taskById: (state) => (id: string) => state.tasks.find((t) => t.id === id),
    agentById: (state) => (id: string) => state.agents.find((a) => a.id === id)
  },

  actions: {
    // Tasks
    async fetchTasks(params?: GetTasksParams) {
      this.tasksLoading = true
      this.tasksError = ''
      try {
        const options = {
          status: params?.status || this.tasksStatusFilter || undefined,
          page: params?.page || this.tasksPage,
          limit: params?.limit || this.tasksLimit
        }
        this.tasks = await getTasks(options)
        if (params?.page) this.tasksPage = params.page
        if (params?.limit) this.tasksLimit = params.limit
        if (params?.status !== undefined) this.tasksStatusFilter = params.status
      } catch (e: any) {
        this.tasksError = e?.message || 'Failed to load tasks'
      } finally {
        this.tasksLoading = false
      }
    },

    async fetchTask(id: string) {
      this.tasksLoading = true
      this.tasksError = ''
      try {
        this.currentTask = await getTask(id)
      } catch (e: any) {
        this.tasksError = e?.message || 'Failed to load task'
      } finally {
        this.tasksLoading = false
      }
    },

    async addTask(data: Partial<ProvisioningTask>) {
      this.tasksLoading = true
      this.tasksError = ''
      try {
        const newTask = await createTask(data)
        this.tasks.unshift(newTask)
        return newTask
      } catch (e: any) {
        this.tasksError = e?.message || 'Failed to create task'
        throw e
      } finally {
        this.tasksLoading = false
      }
    },

    async cancelProvisioningTask(id: string) {
      this.tasksLoading = true
      this.tasksError = ''
      try {
        await cancelTask(id)
        const task = this.tasks.find((t) => t.id === id)
        if (task) {
          task.status = 'failed'
        }
        if (this.currentTask?.id === id) {
          this.currentTask.status = 'failed'
        }
      } catch (e: any) {
        this.tasksError = e?.message || 'Failed to cancel task'
        throw e
      } finally {
        this.tasksLoading = false
      }
    },

    async createBulkTasks(request: BulkProvisionRequest) {
      this.tasksLoading = true
      this.tasksError = ''
      try {
        const newTasks = await bulkProvision(request)
        this.tasks.unshift(...newTasks)
        return newTasks
      } catch (e: any) {
        this.tasksError = e?.message || 'Failed to create bulk tasks'
        throw e
      } finally {
        this.tasksLoading = false
      }
    },

    // Agents
    async fetchAgents() {
      this.agentsLoading = true
      this.agentsError = ''
      try {
        this.agents = await getAgents()
      } catch (e: any) {
        this.agentsError = e?.message || 'Failed to load agents'
      } finally {
        this.agentsLoading = false
      }
    },

    async refreshAgentStatus(id: string) {
      this.agentsLoading = true
      this.agentsError = ''
      try {
        const updatedAgent = await getAgentStatus(id)
        const index = this.agents.findIndex((a) => a.id === id)
        if (index !== -1) {
          this.agents[index] = updatedAgent
        }
        return updatedAgent
      } catch (e: any) {
        this.agentsError = e?.message || 'Failed to refresh agent status'
        throw e
      } finally {
        this.agentsLoading = false
      }
    },

    setTasksStatusFilter(status: string) {
      this.tasksStatusFilter = status
      this.tasksPage = 1
    },

    setTasksPage(page: number) {
      this.tasksPage = page
    },

    setTasksLimit(limit: number) {
      this.tasksLimit = limit
    },

    clearCurrentTask() {
      this.currentTask = null
    }
  }
})
