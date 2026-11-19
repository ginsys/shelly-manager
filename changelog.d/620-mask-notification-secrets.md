Security
- Notification channel credentials are no longer returned by the API. The channel list, the rules list, the
  history list and the create response send `********` in place of a webhook `url`, `webhook_url`, every
  `headers` value and any secret-named key, and a failed send's error no longer quotes the webhook URL in the
  history list or the test-send response. A channel update that sends `********` back keeps the stored value.
  Fixes ginsys/shelly-manager#620.
