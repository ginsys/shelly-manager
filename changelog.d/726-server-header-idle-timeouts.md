Changed
- The server now closes a new connection that has not sent its complete request header within 10
  seconds, and a keep-alive connection that sends no new request for 120 seconds (on such a
  connection the 10 seconds start once the first bytes of the next request arrive). Before, both
  were held open for ever. Nothing needs setting, and request deadlines are unchanged (30 seconds,
  and 5 minutes on `bulk-apply`). A client that sends its body or reads its answer slowly is not
  covered by these two limits. Fixes ginsys/shelly-manager#726.
