Fixed
- An unsaved edit of the device overrides is no longer lost when the page refreshes (after a rollback, Apply, Verify or Refresh). If the saved overrides change under an open edit, the edit is kept, a notice says so, and Save is off until Cancel loads the saved overrides. Fixes ginsys/shelly-manager#855.
