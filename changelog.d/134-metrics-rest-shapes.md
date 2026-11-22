Fixed
- The metrics dashboard now reads what the server sends. The summary, notification and resolution
  panels show the real fields, a successful test alert is reported as a success instead of
  a failure, and one failing panel no longer blanks the others: the error banner names the panel
  that failed. The panels for
  exports, imports, recent notifications and resolutions by user are gone, because the server has
  no such numbers. Fixes ginsys/shelly-manager#134.
