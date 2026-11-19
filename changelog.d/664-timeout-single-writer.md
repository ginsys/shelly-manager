Fixed
- A request that times out now has exactly one writer. The timeout middleware used to answer on
  the same response writer that its handler was still writing to, which is a data race, and
  `go test -race` reported it. The handler now writes through a guarded writer, so after the
  deadline its writes are dropped. Two cases change for the client: a handler that had already
  started its response keeps it and no longer gets the 408 JSON appended to its body, and headers
  the handler had set but not yet sent no longer appear on the 408 answer. A panic in the handler
  no longer ends the server process: it is logged and answered with a 500 when nothing was sent
  yet. Fixes ginsys/shelly-manager#664.
