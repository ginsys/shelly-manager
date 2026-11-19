Security
- A failed webhook or Slack delivery no longer puts the request URL, which carries the webhook token, into its
  error text. The history `error` field, the failed test-send response and the log lines name the operation and
  the failure class only (for example `webhook request failed: Post timed out`), and the webhook success log
  line no longer logs the URL. History rows written by an earlier version are rewritten at start: the whole old
  text is replaced by its first words and a note (`failed to send webhook: details removed`). Fixes
  ginsys/shelly-manager#652.
