# Web UI Requirements (Claude Memory)

## Validated Requirements (2025-08-10)

### Core Display & Navigation
- Default view: Table/list with sortable columns, card view as option
- Flexible card layouts: multiple per row vs single column
- Pagination: 10 devices default, dropdown for 20/30/50/100/All
- Search: All fields (name, IP, MAC, type, status, hostname, notes)
- Auto-refresh: Configurable intervals (1s/5s/10s/30s/off), default 30s
- Updates: Only existing devices, discovery remains separate manual process

### Device Management
- Core fields display: name, type, IP, MAC, model (firmware in detail view)
- Authentication status: filterable with 🔐 icon
- Device-specific UI profiles based on actual capabilities:
  - Single switch: one on/off/toggle
  - Dual switch: two separate controls
  - Dimmer: slider + on/off
  - Roller shutter: up/down/stop/position
  - Sensors: readings only, no controls
- Notes field implemented, extensible metadata design
- Modal confirmations (not browser alerts) for destructive operations
- Optimistic UI with timeout/fallback for control commands

### Configuration Management
- Import/export: Complete device configuration (EVERYTHING)
- Side-by-side diff UI for all config operations
- Full validation: schema, ranges, compatibility, network, dependencies, security
- Hierarchical templates: Global → Generation → Device-type → Individual
- Template inheritance support (simple implementation first)
- History tracking: configurable retention (count + time)
- View modes: full config + diff between any versions
- Device + template history/rollback (device-level priority)
- Automatic drift detection: every 4 hours (configurable)
- Any configuration change = drift
- Visual indicators + hook system for alerts
- Auto-sync option for GitOps mode

### WiFi Provisioning
- Separate UI page working with dedicated provisioning binary
- WiFi credentials: stored encrypted in database, not config file
- Multiple target networks supported (not fallbacks)
- No additional authentication required
- Retry logic: none for device config, 2-3 for WiFi connection issues
- Timeout: 30s default, configurable per session and in config file
- Missing device handling: mark as missing in API/UI

### DHCP Management
- IP assignment: auto-assign next available, allow manual override
- Single IP pool for all device types
- Hostname templates with variables: {type}, {id}, {name}, {mac-short}
- Conflict handling: report + propose fixes, user approves
- OPNSense sync: user validation required before any push
- Sync modes: manual + scheduled, both require validation
- Rollback support for failed syncs
- API credentials: encrypted in database, manual UI rotation

### System Features
- Comprehensive audit log for all operations
- Performance: start simple, prepare for batching at scale
- Error handling: graceful fallbacks, clear user feedback
- Security: data encryption preparation, extensible auth system

## Remaining Questions (High Priority)

### Performance & Scalability (Critical for 100+ devices)
- Q47: Large list handling - at what device count should pagination/virtualization kick in?
- Q48: Caching strategy - cache duration for device status (none, 1s, 5s)?
- Q49: Offline mode - should UI work partially when API unavailable?
- Q50: Background operations - should long operations run in background with progress indication?

### Browser Compatibility (Deployment needs)
- Q51: Legacy browser support - is IE11/Legacy Edge support needed?
- Q52: Design approach - desktop-first or mobile-first design approach?
- Q53: Touch gestures - support swipe actions on mobile devices?
- Q54: PWA capability - should it be installable as Progressive Web App?

### Security (Before production)
- Q55: Authentication - will UI require login in future?
- Q56: Session management - how long should sessions remain active?
- Q57: Data encryption - should sensitive data be encrypted in local storage?
- Q58: Audit logging - track all user actions for security audit?

### UI/UX Priorities (User experience)
- Q59: Feature prioritization - which features are blocking vs nice-to-have?
- Q60: Timeline - target completion date for each priority level?
- Q61: User feedback - how to collect and prioritize user feature requests?
- Q66: Dark mode - is dark theme a priority?
- Q67: Dashboard customization - should users customize dashboard layout?
- Q68: Notifications - in-app only or also email/push notifications?
- Q69: Accessibility - WCAG compliance level (A, AA, AAA)?

### Future Features (Lower priority)
- Q62: Scheduling - what types of schedules (time-based, sunrise/sunset, conditions)?
- Q63: Automation - rule engine complexity (simple if-then vs complex logic)?
- Q64: Integrations - which third-party systems to integrate with first?
- Q65: Monitoring - what metrics are most important (power, uptime, response time)?

**Status: 46/69 questions answered and validated**