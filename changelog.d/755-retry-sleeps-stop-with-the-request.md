Fixed

- A call to a device that retries no longer sleeps through the pause between attempts after its request has ended. The Gen1 and Gen2 clients now stop at once when the request's time is up, so a device that does not answer holds a request for the time the request allows and no longer for several seconds more. Part of ginsys/shelly-manager#755.
