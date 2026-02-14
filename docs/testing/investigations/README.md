# Testing Investigations Archive

This directory contains historical records of exploratory testing sessions and investigations.

## Structure

Each investigation is stored in a dated subdirectory: `YYYY-MM-DD-subject/`

## Investigations

### 2025-12-07: Device Endpoints
**Directory**: [2025-12-07-device-endpoints/](2025-12-07-device-endpoints/)

Comprehensive testing of all 27 device API endpoints. Found 2 critical bugs causing 500 errors.

**Results**:
- 24 / 27 endpoints working (89%)
- 2 bugs tracked via GitHub Issues (originally tasks 207 and 208)

---

## Purpose

This archive preserves:
- Exploratory testing scripts used for one-time investigations
- Bug discovery evidence and analysis
- Test patterns and approaches that informed permanent test development
- Historical context for task creation

## Guidelines

**When to Archive**:
- Temporary investigation scripts that served their purpose
- One-time exploratory tests not suitable for CI
- Bug reproduction scripts with clear task linkage

**Each Investigation Should Include**:
- README.md with summary and findings
- Original test scripts
- Links to tasks created from findings
- Evidence (logs, screenshots, etc.)

## Related Documentation

- [Testing Documentation](../README.md)
- [GitHub Issues](https://github.com/ginsys/shelly-manager/issues)
- [API Documentation](../../api/api-overview.md)
