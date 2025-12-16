# README Export Documentation

**Priority**: HIGH - Post-Commit Required
**Status**: done
**Effort**: 30 minutes

## Context

The README needs documentation about the new export formats (JSON, YAML, SMA) to help users understand their options.

## Success Criteria

- [x] Markdown renders correctly on GitHub
- [x] All links work
- [x] Examples are accurate and tested
- [x] Users understand all available export options

## Implementation

**File**: `README.md`

**Location**: After "Features" section, before "Installation" section

Add new section:

```markdown
## Export Formats

Shelly Manager supports multiple export formats for backing up and sharing device configurations:

### Database Backup (`.db`)
- **Use Case**: Full system backup including all data
- **Format**: SQLite database file
- **Compression**: GZIP or ZIP
- **Best For**: Disaster recovery, system migration

### JSON Export (`.json`)
- **Use Case**: Structured data export for processing
- **Format**: JSON with devices, templates, and metadata
- **Compression**: None, GZIP, or ZIP
- **Best For**: API integration, data analysis, automation
- **Example**:
  ```bash
  curl -X POST http://localhost:8080/api/v1/export/json \
    -H "Authorization: Bearer $API_KEY" \
    -d '{"compression_algo": "gzip"}'
  ```

### YAML Export (`.yaml`)
- **Use Case**: Human-readable configuration export
- **Format**: YAML with devices and templates
- **Compression**: None, GZIP, or ZIP
- **Best For**: GitOps workflows, manual review, documentation
- **Example**:
  ```bash
  curl -X POST http://localhost:8080/api/v1/export/yaml \
    -H "Authorization: Bearer $API_KEY" \
    -d '{"compression_algo": "none"}'
  ```

### SMA Archive (`.sma`)
- **Use Case**: Shelly Manager Archive format
- **Format**: Multi-format archive with metadata
- **Compression**: Built-in compression
- **Best For**: Complete exports with all formats included

### Compression Options

| Algorithm | File Size | Speed | Use Case |
|-----------|-----------|-------|----------|
| `none` | Largest | Fastest | Small datasets, local storage |
| `gzip` | Medium | Medium | General purpose, good balance |
| `zip` | Medium | Medium | Windows compatibility |

**Note**: For hobbyist use, GZIP is recommended for most scenarios.
```

## Validation

- Markdown renders correctly on GitHub
- Links work (if any added)
- Examples are accurate and tested

## Dependencies

- Phase 1 complete (features working)

## Risk

None - Documentation only
