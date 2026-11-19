import { defineStore } from 'pinia'
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
  type NotificationHistory,
  type GetHistoryParams
} from '@/api/notification'

export const useNotificationsStore = defineStore('notifications', {
  state: () => ({
    // Channels
    channels: [] as NotificationChannel[],
    currentChannel: null as NotificationChannel | null,
    channelsLoading: false,
    channelsError: '' as string,

    // Rules
    rules: [] as NotificationRule[],
    rulesLoading: false,
    rulesError: '' as string,

    // History
    history: [] as NotificationHistory[],
    historyLoading: false,
    historyError: '' as string,
    historyPage: 1,
    historyLimit: 50
  }),

  getters: {
    enabledChannels: (state) => state.channels.filter((c) => c.enabled),
    enabledRules: (state) => state.rules.filter((r) => r.enabled),

    sentNotifications: (state) => state.history.filter((h) => h.status === 'sent'),
    failedNotifications: (state) => state.history.filter((h) => h.status === 'failed'),
    pendingNotifications: (state) => state.history.filter((h) => h.status === 'pending'),

    channelById: (state) => (id: string) => state.channels.find((c) => c.id === id),
    rulesByChannel: (state) => (channelId: string) =>
      state.rules.filter((r) => r.channelId === channelId)
  },

  actions: {
    // Channels
    async fetchChannels() {
      this.channelsLoading = true
      this.channelsError = ''
      try {
        this.channels = await getChannels()
      } catch (e: any) {
        this.channelsError = e?.message || 'Failed to load channels'
      } finally {
        this.channelsLoading = false
      }
    },

    async fetchChannel(id: string) {
      this.channelsLoading = true
      this.channelsError = ''
      try {
        this.currentChannel = await getChannel(id)
      } catch (e: any) {
        this.channelsError = e?.message || 'Failed to load channel'
      } finally {
        this.channelsLoading = false
      }
    },

    // An action (add, modify, remove) rethrows its failure and leaves `channelsError` and the list alone:
    // `channelsError` is the failure of the last list load, and the page that acted shows the failure of
    // its own action (ginsys/shelly-manager#701).
    async addChannel(data: Partial<NotificationChannel>) {
      this.channelsLoading = true
      try {
        const newChannel = await createChannel(data)
        this.channels.push(newChannel)
        return newChannel
      } finally {
        this.channelsLoading = false
      }
    },

    async modifyChannel(id: string, data: Partial<NotificationChannel>) {
      this.channelsLoading = true
      try {
        const updated = await updateChannel(id, data)
        const index = this.channels.findIndex((c) => c.id === id)
        if (index !== -1) {
          this.channels[index] = updated
        }
        if (this.currentChannel?.id === id) {
          this.currentChannel = updated
        }
        return updated
      } finally {
        this.channelsLoading = false
      }
    },

    async removeChannel(id: string) {
      this.channelsLoading = true
      try {
        await deleteChannel(id)
        this.channels = this.channels.filter((c) => c.id !== id)
        if (this.currentChannel?.id === id) {
          this.currentChannel = null
        }
      } finally {
        this.channelsLoading = false
      }
    },

    // Rules
    async fetchRules() {
      this.rulesLoading = true
      this.rulesError = ''
      try {
        this.rules = await getRules()
      } catch (e: any) {
        this.rulesError = e?.message || 'Failed to load rules'
      } finally {
        this.rulesLoading = false
      }
    },

    // Like the channel actions: a failed action rethrows and leaves `rulesError` and the list alone.
    async addRule(data: Partial<NotificationRule>) {
      this.rulesLoading = true
      try {
        const newRule = await createRule(data)
        this.rules.push(newRule)
        return newRule
      } finally {
        this.rulesLoading = false
      }
    },

    async removeRule(id: string) {
      this.rulesLoading = true
      try {
        await deleteRule(id)
        this.rules = this.rules.filter((r) => r.id !== id)
      } finally {
        this.rulesLoading = false
      }
    },

    // History
    async fetchHistory(params?: GetHistoryParams) {
      this.historyLoading = true
      this.historyError = ''
      try {
        const options = {
          page: params?.page || this.historyPage,
          limit: params?.limit || this.historyLimit
        }
        this.history = await getHistory(options)
        if (params?.page) this.historyPage = params.page
        if (params?.limit) this.historyLimit = params.limit
      } catch (e: any) {
        this.historyError = e?.message || 'Failed to load history'
      } finally {
        this.historyLoading = false
      }
    },

    setHistoryPage(page: number) {
      this.historyPage = page
    },

    setHistoryLimit(limit: number) {
      this.historyLimit = limit
    },

    clearCurrentChannel() {
      this.currentChannel = null
    }
  }
})
