Changed
- The device configuration page no longer keeps a second copy of an action failure. The
  configuration store sets its error only when a load fails, and an action failure shows once, in the
  page's error panel (ginsys/shelly-manager#650). An apply that changed the device no longer reports
  a failed apply when the status read after it fails: the result card shows and the failed reload
  shows as a load error.
