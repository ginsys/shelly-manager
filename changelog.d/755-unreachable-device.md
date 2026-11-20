Fixed

- A device that does not answer no longer blocks its detail page or comes back as "Internal server error". The status, energy and current-configuration reads now stop before the server's 30 s request deadline and answer 504 `DEVICE_UNREACHABLE`, with a message that names the device and its address. The page shows the stored device at once and fills in each live read as it arrives, and a device last seen more than a day ago no longer shows "online" in the list. Fixes ginsys/shelly-manager#755.
