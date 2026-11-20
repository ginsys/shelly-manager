Changed

- Downloading the configuration of several devices now takes up to 200 devices instead of 10. The download reads the database and contacts no device, so the old cap, copied from bulk apply, had no reason; a download of 200 fully populated configurations is about 1 MiB. More than 200 is still refused with a message that names the limit, and bulk apply keeps its limit of 10. Fixes ginsys/shelly-manager#751.
