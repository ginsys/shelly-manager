# Add Refresh Button to Devices List

**Priority**: LOW
**Status**: not-started
**Effort**: 1 hour

## Context
User feedback: "I don't see where to refresh the device list"

Currently no obvious way to manually trigger a device list refresh.

## Success Criteria
- [ ] Add refresh button/icon to devices page header
- [ ] Trigger device list reload on click
- [ ] Show loading state during refresh
- [ ] Disable button during refresh to prevent duplicate requests
- [ ] Add keyboard shortcut (e.g., R key) for refresh
- [ ] Run `npm run test` to ensure no regressions

## Implementation Approach
1. Add refresh icon button in page header
2. Connect to existing device list fetch function
3. Add loading spinner during refresh
4. Update tests for new UI element

## Files to Investigate
- `web/src/views/DevicesPage.vue`
- `web/src/components/DeviceList.vue` (if separate component)

## Validation
```bash
# Click refresh button on devices page
# Should reload device list with loading indicator

npm run test
npm run build
```
