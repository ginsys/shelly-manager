Security
- A failed webhook or Slack delivery no longer puts the request URL, which carries the webhook token, into its
  error text. The history `error` field, the failed test-send response and the log lines name the operation and
  the failure class only (for example `webhook request failed: Post timed out`), and the webhook success log
  line no longer logs the URL. History rows written before this change keep their old text until they are
  rewritten (ginsys/shelly-manager#652). Refs ginsys/shelly-manager#652.
