Fixed
- A template made from a device import, and the common settings of several devices, no longer carry the device's latitude and longitude (`location` and `system.location`), because those belong to one device and a template is shared. The time zone is kept. Fixes ginsys/shelly-manager#800.
