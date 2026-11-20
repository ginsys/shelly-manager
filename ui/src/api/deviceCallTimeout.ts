// How long the browser waits for a call that contacts a device. The server gives such a call until its
// request deadline, 30 s (`DefaultSecurityConfig().RequestTimeout` in
// internal/api/middleware/security.go), so the browser waits a little longer and the answer, a result
// or an error that names the device, always arrives. If that deadline moves, this moves with it.
//
// Bulk apply has its own, longer one (`BULK_APPLY_TIMEOUT_MS`). Every other call keeps the shared
// client's 10 s, so a hung call that reads only stored data still fails fast. It is a module of its
// own, not part of ./client, so a test that replaces the client keeps this number.
export const DEVICE_CALL_TIMEOUT_MS = 35_000
