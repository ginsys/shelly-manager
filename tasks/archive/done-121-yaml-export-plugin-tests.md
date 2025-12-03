# YAML Export Plugin Tests

**Priority**: CRITICAL
**Status**: completed
**Completed**: 2025-12-03

## Summary
YAML export plugin tests validate plugin info, successful export to YAML, content schema (devices/templates presence), compression variants (gzip/zip/none), and preview handling.

## Files
- internal/plugins/sync/yamlexport/yaml_test.go

## Highlights
- Verifies plugin reports name and supported format “yaml”
- Validates YAML output parses correctly with gopkg.in/yaml.v3
- Ensures expected top-level keys and filename extensions by compression
- Confirms preview path returns success and counts

## Validation
make test
