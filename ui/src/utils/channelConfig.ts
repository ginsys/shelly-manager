// The config of a notification channel, built from the create form's fields. The shapes are the Go structs
// in internal/notification/models.go (EmailConfig, WebhookConfig, SlackConfig). Whether a config is valid
// (an email needs a recipient, a webhook a URL) is the server's rule and its refusal text is shown as it
// comes: this builder only says what the user typed, so an empty required field is sent as empty.

export type ChannelType = 'email' | 'webhook' | 'slack'

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

// Headers are a name-to-value object, so two rows with one name (HTTP names ignore case) would silently
// keep one of them: that is refused here, where the user can see both rows.
function headerObject(rows: ChannelFields['headers']): { headers?: Record<string, string>; error?: string } {
  const headers: Record<string, string> = {}
  const seen = new Set<string>()
  for (const row of rows) {
    const name = row.key.trim()
    if (!name) continue
    if (seen.has(name.toLowerCase())) return { error: `Header ${name} appears twice` }
    seen.add(name.toLowerCase())
    headers[name] = row.value
  }
  return { headers }
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
