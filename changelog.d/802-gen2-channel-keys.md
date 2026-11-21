Fixed
- A Gen2 document key such as `switch:007`, `switch:+1` or `switch:01` is no longer taken as a channel,
  a `switch:1` next to `switch:01` no longer gives two relays with the same id, and a channel or
  section key whose value is not an object (`null`, text) is listed as unread instead of read.
  Apply is unchanged for the documents it writes. Fixes ginsys/shelly-manager#802.
