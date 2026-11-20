<template>
  <template v-if="type === 'email'">
    <div class="form-group">
      <label>Recipients:</label>
      <textarea
        v-model="fields.recipients"
        required
        rows="3"
        class="form-input"
        placeholder="One address per line, or separated by commas"
        data-testid="channel-recipients"
      />
    </div>
    <div class="form-group">
      <label>Subject (optional):</label>
      <input v-model="fields.subject" class="form-input" data-testid="channel-subject" />
    </div>
  </template>
  <template v-else-if="type === 'webhook'">
    <div class="form-group">
      <label>URL:</label>
      <label v-if="stored.url" class="keep">
        <input v-model="keep.url" type="checkbox" data-testid="channel-keep-url" /> Keep the stored URL
      </label>
      <input
        v-if="!keep.url"
        v-model="fields.webhookUrl"
        required
        autocomplete="off"
        class="form-input"
        placeholder="https://"
        data-testid="channel-webhook-url"
      />
    </div>
    <div class="form-group">
      <label>Method:</label>
      <select v-model="fields.method" class="form-input" data-testid="channel-webhook-method">
        <option v-for="m in WEBHOOK_METHODS" :key="m" :value="m">{{ m }}</option>
      </select>
    </div>
    <div class="form-group">
      <label>Headers (optional):</label>
      <label v-if="stored.headers" class="keep">
        <input v-model="keep.headers" type="checkbox" data-testid="channel-keep-headers" /> Keep the stored headers
      </label>
      <template v-if="!keep.headers">
        <div v-for="(row, i) in fields.headers" :key="i" class="header-row">
          <input v-model="row.key" placeholder="Name" class="form-input" data-testid="channel-header-name" />
          <input v-model="row.value" placeholder="Value" autocomplete="off" class="form-input" data-testid="channel-header-value" />
          <button type="button" class="secondary-button" data-testid="channel-remove-header" @click="fields.headers.splice(i, 1)">
            Remove
          </button>
        </div>
        <button
          type="button"
          class="secondary-button"
          data-testid="channel-add-header"
          @click="fields.headers.push({ key: '', value: '' })"
        >
          Add header
        </button>
      </template>
    </div>
    <div class="form-group">
      <label>Secret (optional, signs the payload):</label>
      <label v-if="stored.secret" class="keep">
        <input v-model="keep.secret" type="checkbox" data-testid="channel-keep-secret" /> Keep the stored secret
      </label>
      <input
        v-if="!keep.secret"
        v-model="fields.secret"
        type="password"
        autocomplete="off"
        class="form-input"
        data-testid="channel-webhook-secret"
      />
    </div>
  </template>
  <template v-else>
    <div class="form-group">
      <label>Webhook URL:</label>
      <label v-if="stored.webhook_url" class="keep">
        <input v-model="keep.webhook_url" type="checkbox" data-testid="channel-keep-webhook_url" /> Keep the stored webhook URL
      </label>
      <input
        v-if="!keep.webhook_url"
        v-model="fields.slackUrl"
        required
        autocomplete="off"
        class="form-input"
        placeholder="https://hooks.slack.com/services/..."
        data-testid="channel-slack-url"
      />
    </div>
    <div class="form-group">
      <label>Channel (optional):</label>
      <input v-model="fields.slackChannel" class="form-input" placeholder="#alerts" data-testid="channel-slack-channel" />
    </div>
    <div class="form-group">
      <label>Username (optional):</label>
      <input v-model="fields.slackUsername" class="form-input" data-testid="channel-slack-username" />
    </div>
  </template>
</template>

<script setup lang="ts">
import type { ChannelType } from '@/api/notification'
import { WEBHOOK_METHODS, type ChannelFields } from '@/utils/channelConfig'

// The config fields of one channel type, for the create form and the edit form. The server decides what is
// valid and says why when it refuses. In the edit form a credential the server stores is a "Keep the stored
// ..." choice, on by default: while it is on, its input is not rendered at all (a hidden required input
// would block the form's submit) and the form sends the placeholder back.
// `fields`, `stored` and `keep` belong to the form that holds this component, which reads them when it
// submits. The create form passes only the first two.
withDefaults(
  defineProps<{
    type: ChannelType
    fields: ChannelFields
    stored?: Record<string, boolean>
    keep?: Record<string, boolean>
  }>(),
  { stored: () => ({}), keep: () => ({}) }
)
</script>

<style scoped>
.form-group {
  margin-bottom: 12px;
}
.form-group label {
  display: block;
  margin-bottom: 4px;
  font-weight: 500;
}
.form-group .keep {
  font-weight: 400;
}
.form-input {
  width: 100%;
  padding: 8px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
}
.header-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}
.secondary-button {
  padding: 8px 16px;
  background: #e5e7eb;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
.secondary-button:hover {
  background: #d1d5db;
}
</style>
