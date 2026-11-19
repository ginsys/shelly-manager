Changed
- The devices list, the device page, the device configuration page and the configuration history
  page no longer open a browser dialog when an action fails or finishes. A failed action shows an
  error panel with its error code and suggested actions, where it happened: under the toolbar, or
  inside the dialog that is open. The result of an apply shows in a card on the configuration page
  (it names the device when you have moved to another one), and an empty template id is
  flagged next to the field. The other pages still use dialogs, tracked in
  ginsys/shelly-manager#614.
