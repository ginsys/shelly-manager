Added
- The devices page now has a select column and a Bulk Operations button that opens the bulk
  dialog (import, drift detection, drift report) for the ticked devices. The button stays off
  while nothing is ticked, the select-all box covers the rows on the page, and the choice is kept
  when you move to another page. The dialog lists the picked devices, lets you drop one, sends
  exactly those and shows a failed request in the dialog instead of a browser alert. Fixes
  ginsys/shelly-manager#136.
