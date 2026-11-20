Fixed
- The OPNsense plugin's schema example no longer shows `${OPNSENSE_API_KEY}` and `${OPNSENSE_API_SECRET}`, which nothing expands: it shows plain placeholders, and the `api_key` and `api_secret` descriptions say the value is sent as written. Fixes ginsys/shelly-manager#736.
