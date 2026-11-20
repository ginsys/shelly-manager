import { describe, it, expect } from 'vitest'
import { buildEditConfig, editFromConfig } from '../channelConfig'

// ginsys/shelly-manager#783: editing a channel's config. The read configs below are shaped as the server
// sends them (internal/notification/config_secrets.go): `url`, `webhook_url` and `headers` are masked whole
// and `secret` is masked by the key rule, each as the placeholder. The server takes a placeholder back as
// "keep the stored value" and any other value as a new credential.

const MASK = '********'

const webhookRead = {
  url: MASK,
  method: 'PUT',
  headers: MASK,
  secret: MASK,
  template: '{"text":"{{.Message}}"}',
  timeout: 10,
  retries: 5
}

describe('editFromConfig', () => {
  it('fills the plain fields from the read and never puts a masked value in an input', () => {
    const { fields, stored, keep } = editFromConfig('webhook', webhookRead)
    expect(fields.method).toBe('PUT')
    expect(fields.webhookUrl).toBe('')
    expect(fields.secret).toBe('')
    expect(fields.headers).toEqual([{ key: '', value: '' }])
    expect(stored).toEqual({ url: true, headers: true, secret: true })
    expect(keep).toEqual({ url: true, headers: true, secret: true })
  })

  it('reads an email config as one recipient per line', () => {
    const { fields, stored } = editFromConfig('email', { recipients: ['a@example.com', 'b@example.com'], subject: 'Alert' })
    expect(fields.recipients).toBe('a@example.com\nb@example.com')
    expect(fields.subject).toBe('Alert')
    expect(stored).toEqual({})
  })

  it('reads a slack config', () => {
    const { fields, stored, keep } = editFromConfig('slack', { webhook_url: MASK, channel: '#ops', username: 'shelly' })
    expect(fields.slackChannel).toBe('#ops')
    expect(fields.slackUsername).toBe('shelly')
    expect(stored).toEqual({ webhook_url: true })
    expect(keep).toEqual({ webhook_url: true })
  })

  it('marks a slot with no stored value as not stored, so there is nothing to keep', () => {
    // The server masks only a slot that holds a value, so a webhook with no headers and no secret has neither key.
    const { stored, keep } = editFromConfig('webhook', { url: MASK, method: 'POST' })
    expect(stored).toEqual({ url: true, headers: false, secret: false })
    expect(keep).toEqual({ url: true, headers: false, secret: false })
  })
})

describe('buildEditConfig', () => {
  it('sends the placeholder back for every credential the user left alone', () => {
    const { fields, keep } = editFromConfig('webhook', webhookRead)
    const built = buildEditConfig('webhook', webhookRead, fields, keep)
    expect(built.config).toMatchObject({ url: MASK, headers: MASK, secret: MASK, method: 'PUT' })
  })

  it('sends a credential the user typed as typed, and keeps the others', () => {
    const { fields, keep } = editFromConfig('webhook', webhookRead)
    keep.url = false
    fields.webhookUrl = ' https://hooks.example.com/new '
    const built = buildEditConfig('webhook', webhookRead, fields, keep)
    expect(built.config).toMatchObject({ url: 'https://hooks.example.com/new', headers: MASK, secret: MASK })
  })

  it('sends a new secret as typed and a removed secret as no key', () => {
    const typed = editFromConfig('webhook', webhookRead)
    typed.keep.secret = false
    typed.fields.secret = 'new-signing-secret'
    expect(buildEditConfig('webhook', webhookRead, typed.fields, typed.keep).config).toMatchObject({
      secret: 'new-signing-secret'
    })

    const removed = editFromConfig('webhook', webhookRead)
    removed.keep.secret = false
    expect(buildEditConfig('webhook', webhookRead, removed.fields, removed.keep).config).not.toHaveProperty('secret')
  })

  it('keeps the stored headers when keep is on, whatever the header rows hold', () => {
    const { fields, keep } = editFromConfig('webhook', webhookRead)
    fields.headers = [{ key: 'X-Ignored', value: 'not sent' }]
    const built = buildEditConfig('webhook', webhookRead, fields, keep)
    expect(built.config?.headers).toBe(MASK)
  })

  it('replaces the headers with the new rows when keep is off', () => {
    const { fields, keep } = editFromConfig('webhook', webhookRead)
    keep.headers = false
    fields.headers = [
      { key: 'Authorization', value: 'Bearer new' },
      { key: ' X-Team ', value: 'ops' },
      { key: '', value: '' }
    ]
    const built = buildEditConfig('webhook', webhookRead, fields, keep)
    expect(built.config?.headers).toEqual({ Authorization: 'Bearer new', 'X-Team': 'ops' })
  })

  it('replaces the headers with an empty list as no headers key at all', () => {
    const { fields, keep } = editFromConfig('webhook', webhookRead)
    keep.headers = false
    const built = buildEditConfig('webhook', webhookRead, fields, keep)
    expect(built.config).not.toHaveProperty('headers')
  })

  it('keeps the config keys the form does not model, as the read gave them', () => {
    const { fields, keep } = editFromConfig('webhook', webhookRead)
    const built = buildEditConfig('webhook', webhookRead, fields, keep)
    expect(built.config).toMatchObject({ template: '{"text":"{{.Message}}"}', timeout: 10, retries: 5 })

    const slack = { webhook_url: MASK, channel: '#ops', icon_emoji: ':bell:', template: 't' }
    const s = editFromConfig('slack', slack)
    expect(buildEditConfig('slack', slack, s.fields, s.keep).config).toMatchObject({ icon_emoji: ':bell:', template: 't' })
  })

  it('does not send a key twice when the read spells a modelled key in capitals', () => {
    const read = { URL: MASK, method: 'POST' }
    const { fields, keep } = editFromConfig('webhook', read)
    const built = buildEditConfig('webhook', read, fields, keep)
    expect(Object.keys(built.config ?? {}).filter((k) => k.toLowerCase() === 'url')).toEqual(['url'])
    expect(built.config?.url).toBe(MASK)
  })

  it('sends no headers and no secret key when the read stored none', () => {
    const read = { url: MASK, method: 'POST' }
    const { fields, keep } = editFromConfig('webhook', read)
    const built = buildEditConfig('webhook', read, fields, keep)
    expect(built.config).toEqual({ url: MASK, method: 'POST' })
  })

  it('refuses the header names of the create form on a replacement too', () => {
    const { fields, keep } = editFromConfig('webhook', webhookRead)
    keep.headers = false
    fields.headers = [{ key: '__proto__', value: 'x' }]
    expect(buildEditConfig('webhook', webhookRead, fields, keep).error).toBe('Header name __proto__ cannot be used')

    fields.headers = [{ key: 'X-A', value: '1' }, { key: 'x-a', value: '2' }]
    expect(buildEditConfig('webhook', webhookRead, fields, keep).error).toBe('Header x-a appears twice')
  })

  it('builds an email config with the typed recipients and no stored slots', () => {
    const read = { recipients: ['old@example.com'], subject: 'Old', template: 'keep me' }
    const { fields, keep } = editFromConfig('email', read)
    fields.recipients = 'new@example.com, other@example.com'
    fields.subject = ''
    expect(buildEditConfig('email', read, fields, keep).config).toEqual({
      recipients: ['new@example.com', 'other@example.com'],
      template: 'keep me'
    })
  })

  it('sends an empty required URL as empty so the server says why', () => {
    const { fields, keep } = editFromConfig('slack', { webhook_url: MASK })
    keep.webhook_url = false
    expect(buildEditConfig('slack', { webhook_url: MASK }, fields, keep).config).toMatchObject({ webhook_url: '' })
  })
})
