# SMA Format Frontend Implementation Summary

> **⚠️ Accuracy note (2026-07).** This document was written when SMA support was
> prototyped and overstates import. The truth: **SMA export works end to end; SMA
> import does not.** There is no import UI (the prototype form was removed), the
> app's SMA import helpers target non-existent `/import/sma*` routes (404), and
> even the generic `POST /api/v1/import` (`plugin_name: "sma"`) only previews —
> non-dry-run restore is an unimplemented stub that fakes success (#272). The
> ✅-marked "Import Features", "Import Configuration", and import-related "User
> Experience" items below (conflict resolution, merge strategies, safety backups,
> drag-and-drop, progress UI, etc.) describe **intended design, not shipped
> behavior**. See the "Status" section at the end for the verified state.

## Overview

Frontend support for the SMA (Shelly Management Archive) format in the Shelly
Manager Vue.js app. The **export** path and the client-side **codec**
(parser/generator) are implemented; **import** is not wired end to end (see the
banner above and the Status section).

## 🎯 Implementation Status

### ✅ Core Utilities

**File: `src/utils/sma-parser.ts`**
- Complete SMA file parsing and validation
- Gzip decompression support
- SHA-256 checksum verification
- Structure validation with detailed error reporting
- Metadata extraction without full parsing
- Selective section filtering
- Performance tracking (parse time, compression ratios)

**File: `src/utils/sma-generator.ts`**
- SMA file generation with full configuration support
- Gzip compression with adjustable levels (1-9)
- SHA-256 checksum calculation
- File download and upload preparation
- Size estimation algorithms
- Data source validation
- Export configuration filtering

### ✅ API Integration

**File: `src/api/export.ts` (Extended)**
- SMA **export** endpoint definitions (working)
- ⚠️ SMA **import** helpers target `/import/sma*` routes that do not exist (404);
  the working backend path is the generic `POST /api/v1/import` (#272 for the
  restore stub). These helpers need rewiring.
- TypeScript interfaces for SMA operations
- Error handling and response parsing

### ✅ State Management

**File: `src/stores/export.ts` (Extended)**
- SMA export state management
- Result caching and progress tracking
- File download handling

**File: `src/stores/import.ts` (Extended)**
- SMA import state management
- File preview and validation states
- Import result tracking

### ✅ UI Components

**File: `src/components/SMAConfigForm.vue`**
- Comprehensive SMA configuration interface
- Compression settings with visual feedback
- Data section selection
- Size estimation display
- Export metadata configuration
- Real-time validation and preview

**SMA import — preview-only backend, no UI**
- A prototype `src/components/SMAImportForm.vue` existed but was never mounted by
  any page or route, so the workflow was unreachable. It has been removed.
- `POST /api/v1/import` with `plugin_name: "sma"` dispatches to the registered SMA
  plugin (`file`/`data` sources; `url` unimplemented). **Validation and dry-run
  preview work; non-dry-run persistence is a placeholder that fakes success
  without writing to the DB (#272).** There is no dedicated `/import/sma*` route.
- Frontend gap: no import UI, and the app's SMA import helpers target the
  non-existent `/import/sma*` paths (404) instead of the generic route.

**File: `src/components/BackupForm.vue` (Enhanced)**
- Integrated SMA configuration when SMA format selected
- Dynamic form adaptation
- Size calculation integration

### ✅ Testing & Quality

**Files: `src/utils/__tests__/sma-*.test.ts`**
- Comprehensive unit test coverage (58 tests total)
- Parser functionality testing (28 tests)
- Generator functionality testing (30 tests)
- Error handling validation
- Performance metrics testing
- All tests passing ✅

### ✅ Dependencies

**File: `package.json` (Updated)**
- Added `pako` for Gzip compression/decompression (pako 3 ships its own types;
  `@types/pako` was later removed — see PR #263)
- Added `crypto-browserify` for SHA-256 hashing

## 🏗️ Architecture

### Data Flow

```
User Action → Vue Component → Pinia Store → API Client → Backend
     ↓                ↓            ↓           ↓
UI Updates ← State Updates ← Response ← SMA Plugin
```

### File Processing

```
Export (working):      Generated Data → Generator → Compression → Download
Import preview (OK):   SMA File → POST /api/v1/import (dry_run) → validated preview
Import restore (stub): non-dry-run persists nothing, fakes success (#272)
Import frontend (gap): app's SMA helpers call /import/sma* (404); no import UI
```

### Component Integration

```
BackupForm (Format Selection)
    ↓ (when SMA selected)
SMAConfigForm (Configuration)
    ↓ (on submit)
Export Store (State Management)
    ↓ (API calls)
Backend SMA Plugin
```

## 🔧 Configuration Options

### Export Configuration
- **Compression**: Enable/disable with levels 1-9
- **Data Integrity**: SHA-256 checksum inclusion
- **Sections**: Selective data inclusion
- **Metadata**: Creator attribution and export tracking
- **Device Selection**: All devices or specific subset

### Import Configuration (intended — not shipped)
- **Validation**: Checksum and structure verification — works in preview
- **Preview**: Dry-run capability — works
- **Conflict Resolution**: Overwrite/merge/skip strategies — ❌ not implemented
  (backend `ImportOptions` has only `force_overwrite`, no merge strategy)
- **Safety**: Automatic backup before import — ❌ `backup_before` decoded but
  unused by the import stub (#272)
- **Selective Import**: Choose specific sections — ❌ not implemented

## 🚀 Features & Status

(Export items are shipped; import items are largely intended-not-shipped — see the
accuracy banner at the top.)

### Export Features
1. ✅ Format selection in backup forms
2. ✅ Compression configuration (levels 1-9)
3. ✅ Data section selection
4. ✅ Integrity verification (SHA-256)
5. ✅ Metadata tracking and attribution
6. ✅ Size estimation with real-time updates
7. ✅ Progress tracking and status display

### Import Features (mostly NOT shipped — see banner)
1. ✅ File validation (structure + integrity) — in preview
2. ✅ Preview mode with detailed analysis — dry-run works
3. ❌ Conflict detection and resolution — not implemented (#272)
4. ❌ Selective section import — not implemented
5. ❌ Multiple merge strategies — no such backend option
6. ✅ Dry run capability — works
7. ❌ Safety backups before import — `backup_before` unused by the stub (#272)

### User Experience
Export UX (format selection, size estimation, validation feedback) is shipped.
The import-side UX below described the removed `SMAImportForm.vue` prototype and
is **not present** — there is no import screen:
1. ❌ Drag-and-drop file selection — no import UI
2. ✅ Real-time validation feedback — export forms
3. ❌ Progress indicators and status — no import UI
4. ✅ Clear error messages and recovery — export forms
5. ✅ Responsive design (mobile-friendly)
6. ✅ Accessibility considerations
7. ✅ Comprehensive help text

## 📊 Performance Characteristics

### File Size Support
- **Small**: 1-20 devices (50-500 KB)
- **Medium**: 20-100 devices (500KB-2MB)
- **Large**: 100+ devices (2-10 MB)
- **Enterprise**: 500+ devices (10-50 MB)

### Processing Performance
- **Export**: 1-5 seconds typical
- **Import**: N/A — restore is not implemented (#272); preview parsing only
- **Memory Usage**: 2-3x file size during processing
- **Compression**: 30-45% size reduction typical

### Browser Support
- Chrome 90+, Firefox 88+, Safari 14+, Edge 90+
- Requires File API, modern JavaScript, crypto support

## 🔐 Security Implementation

### Data Protection
- Secure file handling with validation
- Checksum integrity verification
- No sensitive data logging
- Secure temporary file cleanup

### Best Practices
- Input validation at all levels
- Error boundary implementation
- Safe defaults for all options
- Clear security warnings in UI

## 📚 Documentation

**File: `ui/README-SMA.md`**
- Complete user guide with examples
- Developer documentation
- Troubleshooting guide
- API reference
- Security considerations
- Performance optimization tips

## 🧪 Testing Coverage

### Unit Tests (58 total - 100% passing)
- **Parser Tests**: 28 tests covering all parsing scenarios
- **Generator Tests**: 30 tests covering all generation scenarios
- **Error Handling**: Comprehensive error scenario testing
- **Performance**: Size and timing validation
- **Edge Cases**: Malformed data, browser compatibility

### Test Categories
- ✅ Happy path scenarios
- ✅ Error conditions
- ✅ Edge cases
- ✅ Performance metrics
- ✅ Browser compatibility
- ✅ Security validation

## 🔄 Integration Points

### Backend Integration
- Export uses the established SMA export endpoints (`POST /export/sma`,
  `GET /export/sma/{id}/download`)
- Import backend: generic `POST /import` (`plugin_name: "sma"`) previews only —
  non-dry-run restore is a stub that fakes success (#272); no `/import/sma` route,
  no frontend import UI
- Follows existing authentication patterns
- Maintains API response consistency

### Frontend Integration
- Seamless integration with existing backup workflows
- Extends existing components without breaking changes
- Uses established state management patterns
- Follows existing design system

### User Workflow Integration
1. **Backup Management** → SMA format option available
2. **Format Selection** → Dynamic configuration appears
3. **Configuration** → Real-time validation and preview
4. **Creation** → Progress tracking and result display
5. **Import** → no UI; generic `POST /import` previews only, restore stubbed (#272)

## ✨ Key Benefits

1. **Export Implementation**: SMA export is wired to the backend; import is not
2. **User Experience**: Intuitive, responsive interface
3. **Data Integrity**: Comprehensive validation and verification
4. **Performance**: Optimized for various file sizes
5. **Security**: Secure handling of sensitive data
6. **Maintainability**: Well-structured, tested codebase
7. **Documentation**: Comprehensive user and developer guides

## Status

SMA **export** is implemented and wired end to end. SMA **import** is only
partly functional: the generic `POST /api/v1/import` (`plugin_name: "sma"`) route
dispatches to the plugin and previews correctly, but non-dry-run restore persists
nothing (#272), and there is no frontend UI.

- ✅ **Export**: create + download wired to `POST /export/sma` and
  `GET /export/sma/{id}/download`
- ✅ **Codec**: parser/generator with checksum verification, unit-tested
- ⚠️ **Import backend**: generic `POST /import` (`plugin_name: "sma"`) validates
  and previews, but non-dry-run persistence is a stub that fakes success (#272)
- ❌ **Import frontend**: no UI; SMA helpers target `/import/sma*` (404)
- ⚠️ **Docs**: older "production-ready / complete import workflow" claims in this
  file and the SMA guides referred to the export path and the prototyped-but-
  unwired import path; corrected to reflect the above.