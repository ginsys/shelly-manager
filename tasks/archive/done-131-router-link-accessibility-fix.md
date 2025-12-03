# Router-Link Accessibility Fix

**Priority**: CRITICAL
**Status**: completed
**Completed**: 2025-12-03

## Summary
Improved accessibility for navigation and key links by adding appropriate ARIA attributes to router-links:
- Added `aria-current="page"` when active for top navigation links.
- Added `aria-label` for nav items, dropdown menu items, and inline links.
- Added `role="menuitem"` to dropdown items.

## Files Updated
- ui/src/layouts/MainLayout.vue
- ui/src/pages/DevicesPage.vue (device detail links)
- ui/src/pages/DeviceDetailPage.vue (back and config links)
- ui/src/pages/DeviceConfigPage.vue (back link)

## Rationale
These changes improve screen-reader support and clarify focus/context for keyboard users, aligning with WCAG guidance.

## Validation
- Manual keyboard navigation and screen reader hints
- Basic UI smoke tests continue to pass
