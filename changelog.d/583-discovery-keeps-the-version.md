Fixed

- A discovery pass no longer changes the version of a device it found unchanged, so a device form that was open during a pass can still be saved. The version now moves only when the address, the type, the firmware or the stored settings changed, and one helper sets the next version for every writer. Fixes ginsys/shelly-manager#583 and ginsys/shelly-manager#580.
