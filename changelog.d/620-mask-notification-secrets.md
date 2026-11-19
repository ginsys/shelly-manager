Security
- Notification channel credentials are no longer returned by the API. The channel list, the rules list, the
  history list and the create response send `********` in place of the value (of any type) under a webhook
  `url`, `webhook_url`, `headers` or any secret-named key. A channel update that sends `********` for such a key
  keeps the stored value, and a config that repeats a member name is refused with 400. Refs ginsys/shelly-manager#620.
