Fixed
- The device scanner, the generation detection, the Gen1 and Gen2 clients and the provisioner no
  longer follow an HTTP redirect from a device: the redirect is returned as the answer, so a device
  or a host that answers with one cannot send the manager to another address. The provisioner
  reports any answer outside 2xx as a failed request. Redirect behaviour of real devices was not
  checked on hardware. The unit tests also stopped sending packets off the host: they use a closed
  loopback port, a loopback range or a local test server, and the `scan-ap` CLI test runs with no
  WiFi tool on its PATH.
