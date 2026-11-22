Changed

- A block now starts the IP's counting over: its suspicious request count, rate limit violation count, attack types and hourly record are emptied, so when the block ends one more suspicious request does not block it again. A blocked IP stays in the security metrics while the block lasts, and the monitor no longer drops an entry whose block is still running. Refs ginsys/shelly-manager#861.
