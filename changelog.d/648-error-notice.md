Added
- A failure that arrives after you left the page, or while a dialog of the page is open over its
  error panel, now shows in one notice at the top right of the app, with the action and the device
  it belongs to, until you dismiss it (ginsys/shelly-manager#648). A second failure replaces the
  first. It covers the devices list, the device page, the device configuration page and the configuration
  history page. A
  failure on the page you are on, with no dialog over it, still shows in the page panel. The result
  of an apply that worked is still shown only on the configuration page.
