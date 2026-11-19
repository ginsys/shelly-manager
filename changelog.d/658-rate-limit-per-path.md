Fixed
- A path rate limit now counts only the requests to its own paths, and the device control limit is
  enforced for the first time (ginsys/shelly-manager#658). Before, the bulk limit of 20 was
  compared with all the requests of a client IP: after 20 ordinary requests in the hour, the first
  bulk call was refused with 429 and the client was blocked on every route for 15 minutes. A path
  limit now refuses only its own paths until its one-hour window resets and blocks no other route.
  The control limit of 100 an hour never matched a real request before; it now counts per client IP
  and per device, and the longest matching path pattern wins. The default limit of 1000 requests an
  hour and its 15-minute block are unchanged.
