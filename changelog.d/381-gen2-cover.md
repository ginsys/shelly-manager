Added
- Applying a desired roller to a Gen2 or Gen3 device in cover profile (Plus 2PM, Pro 2PM, Pro Dual Cover) now writes
  the travel times, motor direction, input mode and swap, and the obstacle and safety settings with `Cover.SetConfig`
  on `cover:0`, sending only the keys set, and reads them back. A roller on a device in switch profile, a travel time
  outside 1 to 300 seconds, and a roller setting with no Cover key are refused before any write, and the apply never
  changes the profile. Changing the motor direction marks a restart owed. Refs ginsys/shelly-manager#381.
