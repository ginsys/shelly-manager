Security
- Notification channel credentials are no longer returned by the API. The channel list, the rules list, the
  history list and the create response send `********` in place of a webhook `url`, `webhook_url`, every
  `headers` value and any secret-named key. A channel update that sends `********` back keeps the stored value,
  and a config that repeats a member name is refused with 400. The request URL in a failed delivery's error text
  is not masked yet (ginsys/shelly-manager#652). Fixes ginsys/shelly-manager#620.
