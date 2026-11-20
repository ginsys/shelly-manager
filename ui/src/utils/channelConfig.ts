// The config of a notification channel, built from the create form's fields. The shapes are the Go structs
// in internal/notification/models.go (EmailConfig, WebhookConfig, SlackConfig). Whether a config is valid
// (an email needs a recipient, a webhook a URL) is the server's rule and its refusal text is shown as it
// comes: this builder only says what the user typed, so an empty required field is sent as empty.
// The edit form (ginsys/shelly-manager#783) starts from a channel's read config, where the server has masked
// the credentials; editFromConfig and buildEditConfig below say which are kept and which are replaced.

import type { ChannelType } from '@/api/notification'

export interface ChannelFields {
  recipients: string
  subject: string
  webhookUrl: string
  method: string
  headers: { key: string; value: string }[]
  secret: string
  slackUrl: string
  slackChannel: string
  slackUsername: string
}

export const WEBHOOK_METHODS = ['POST', 'PUT', 'PATCH']

export function emptyChannelFields(): ChannelFields {
  return {
    recipients: '',
    subject: '',
    webhookUrl: '',
    method: 'POST',
    headers: [{ key: '', value: '' }],
    secret: '',
    slackUrl: '',
    slackChannel: '',
    slackUsername: ''
  }
}

// An optional field is left out when it is empty, as the Go structs omit it.
function optional(config: Record<string, unknown>, name: string, value: string) {
  const text = value.trim()
  if (text) config[name] = text
}

// The request goes through axios, whose config merge drops these three names from a body, so a header with
// one of them would be silently lost. They are refused here, where the user can see the row.
const UNSENDABLE_HEADER_NAMES = new Set(['__proto__', 'constructor', 'prototype'])

// Headers are a name-to-value object, so two rows with one name (HTTP names ignore case) would silently
// keep one of them: that is refused here too.
function headerObject(rows: ChannelFields['headers']): { headers?: Record<string, string>; error?: string } {
  const entries: [string, string][] = []
  const seen = new Set<string>()
  for (const row of rows) {
    const name = row.key.trim()
    if (!name) continue
    if (UNSENDABLE_HEADER_NAMES.has(name.toLowerCase())) return { error: `Header name ${name} cannot be used` }
    if (seen.has(name.toLowerCase())) return { error: `Header ${name} appears twice` }
    seen.add(name.toLowerCase())
    entries.push([name, row.value])
  }
  return { headers: Object.fromEntries(entries) }
}

export function buildChannelConfig(
  type: ChannelType,
  f: ChannelFields
): { config: Record<string, unknown>; error?: undefined } | { config?: undefined; error: string } {
  if (type === 'email') {
    const config: Record<string, unknown> = {
      recipients: f.recipients.split(/[\n,]/).map((r) => r.trim()).filter(Boolean)
    }
    optional(config, 'subject', f.subject)
    return { config }
  }
  if (type === 'webhook') {
    const { headers, error } = headerObject(f.headers)
    if (error) return { error }
    const config: Record<string, unknown> = { url: f.webhookUrl.trim(), method: f.method }
    if (headers && Object.keys(headers).length > 0) config.headers = headers
    if (f.secret) config.secret = f.secret
    return { config }
  }
  const config: Record<string, unknown> = { webhook_url: f.slackUrl.trim() }
  optional(config, 'channel', f.slackChannel)
  optional(config, 'username', f.slackUsername)
  return { config }
}

// The keys of a config that the server masks whole on every read (internal/notification/config_secrets.go).
// The server masks a slot only when it holds a value, so a masked key in a read means a stored credential.
const MASKED_SLOTS: Record<ChannelType, string[]> = {
  email: [],
  webhook: ['url', 'headers', 'secret'],
  slack: ['webhook_url']
}

// The keys the form shows. Every other key of a config (template, timeout, retries, icon_emoji) is not
// edited here and goes back as the read gave it, because a PUT replaces the whole config.
const OWNED_KEYS: Record<ChannelType, string[]> = {
  email: ['recipients', 'subject'],
  webhook: ['url', 'method', 'headers', 'secret'],
  slack: ['webhook_url', 'channel', 'username']
}

// The server compares a member name by Unicode simple case folding (encoding/json and strings.EqualFold), so
// "URL" and the long-s spelling of "secret" are the same name to it. Upper then lower case folds the same way
// for the names used here; toLowerCase alone would leave the long s apart and send the member twice.
function fold(name: string): string {
  return name.toUpperCase().toLowerCase()
}

function keyOf(config: Record<string, unknown>, name: string): string | undefined {
  return Object.keys(config).find((k) => fold(k) === name)
}

function textOf(config: Record<string, unknown>, name: string): string {
  const key = keyOf(config, name)
  const value = key === undefined ? undefined : config[key]
  return typeof value === 'string' ? value : ''
}

export interface ChannelEdit {
  fields: ChannelFields
  // Per masked slot: does the read hold a stored credential, and does the user keep it (the default).
  stored: Record<string, boolean>
  keep: Record<string, boolean>
}

// The state of the edit form for a channel's read config. A masked value is never put in an input.
export function editFromConfig(type: ChannelType, config: Record<string, unknown>): ChannelEdit {
  const fields = emptyChannelFields()
  if (type === 'email') {
    const recipients = config[keyOf(config, 'recipients') ?? '']
    fields.recipients = Array.isArray(recipients) ? recipients.join('\n') : ''
    fields.subject = textOf(config, 'subject')
  } else if (type === 'webhook') {
    fields.method = textOf(config, 'method') || 'POST'
  } else {
    fields.slackChannel = textOf(config, 'channel')
    fields.slackUsername = textOf(config, 'username')
  }
  const stored: Record<string, boolean> = {}
  for (const slot of MASKED_SLOTS[type]) stored[slot] = textOf(config, slot) !== ''
  return { fields, stored, keep: { ...stored } }
}

// The config to send for an edit: every key of the read that the form does not own, the form's own keys as
// typed, and for each kept slot the placeholder exactly as the server sent it, which the server takes back
// as the stored value. A replaced slot goes as typed, and an empty one goes as no key (or, for a required
// URL, as empty, and the server says why).
export function buildEditConfig(
  type: ChannelType,
  read: Record<string, unknown>,
  fields: ChannelFields,
  keep: Record<string, boolean>
): ReturnType<typeof buildChannelConfig> {
  const typed: ChannelFields = { ...fields }
  if (keep.url) typed.webhookUrl = ''
  if (keep.headers) typed.headers = []
  if (keep.secret) typed.secret = ''
  if (keep.webhook_url) typed.slackUrl = ''
  const built = buildChannelConfig(type, typed)
  if (built.error !== undefined) return built

  const owned = new Set(OWNED_KEYS[type])
  const config: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(read)) {
    if (!owned.has(fold(key))) config[key] = value
  }
  Object.assign(config, built.config)
  for (const slot of MASKED_SLOTS[type]) {
    const key = keyOf(read, slot)
    if (keep[slot] && key !== undefined) config[slot] = read[key]
  }
  return { config }
}
