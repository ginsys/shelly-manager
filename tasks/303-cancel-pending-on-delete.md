# Cancel Pending Requests on Device Delete

**Priority**: MEDIUM
**Status**: not-started
**Effort**: 2 hours

## Context
When deleting a device, pending `/status` and `/energy` requests complete after the DELETE succeeds, causing error messages to appear despite the deletion being successful.

User feedback: "I'm going to delete a device, the interface gives me an error, but the device gets deleted"

## Evidence
Timeline from logs:
- DELETE at 00:07:18.295 returns 204 (success)
- /energy request at 00:07:23.533 fails (device already deleted)
- /status request at 00:07:28.536 fails (device already deleted)

## Success Criteria
- [ ] Use AbortController to cancel pending requests when delete is initiated
- [ ] Don't display errors for cancelled/aborted requests
- [ ] Show only delete success/failure status to user
- [ ] Add unit tests for request cancellation
- [ ] Run `npm run test` to ensure no regressions

## Implementation Approach
1. Create AbortController when component mounts
2. Pass abort signal to all device API requests
3. Call abort() when delete is initiated
4. Filter out AbortError in error handling

## Files to Investigate
- `web/src/views/DevicesPage.vue`
- `web/src/composables/useDeviceApi.ts` (or similar)
- `web/src/api/` (API client implementation)

## Validation
```bash
# Delete a device with pending requests
# Should see only success message, no errors

npm run test
```
