Fixed
- Rotating the admin key now refuses a new key that starts or ends with whitespace, with the same
  400 validation error as a blank key, and keeps the stored key. A header value loses trailing
  whitespace, and leading whitespace too in `X-API-Key`, so most such keys could not be presented
  again and every client, the one that rotated included, was locked out until a restart. Whitespace
  inside a key is still allowed. Fixes ginsys/shelly-manager#680.
