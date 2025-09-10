# SMA (Shelly Management Archive) Format Support

This document explains how to use the SMA format support in the Shelly Manager frontend.

## Overview

The SMA format is a specialized archive format designed for comprehensive backup and restoration of Shelly device management data. It provides:

- **Complete Data Coverage**: Devices, templates, configurations, discovered devices, network settings, plugin configurations, and system settings
- **Data Integrity**: SHA-256 checksums and validation for reliable data transfer
- **Compression**: Gzip compression for reduced file sizes
- **Version Compatibility**: Forward and backward compatibility with version management
- **Selective Import/Export**: Choose specific data sections to include

## Features

### Export Features

1. **Format Selection**: Available in backup creation forms
2. **Compression Configuration**: Adjustable compression levels (1-9)
3. **Data Section Selection**: Choose which data to include
4. **Integrity Verification**: Automatic checksum generation
5. **Metadata Tracking**: Export tracking and attribution

### Import Features

1. **File Validation**: Structure and integrity validation
2. **Preview Mode**: See what will be imported before applying
3. **Conflict Detection**: Identify potential conflicts with existing data
4. **Selective Import**: Choose which sections to import
5. **Merge Strategies**: Control how conflicts are resolved
6. **Dry Run**: Preview changes without applying them

## Usage Guide

### Exporting SMA Files

1. **Navigate to Backup Management**
   - Go to the Backup Management page
   - Click "Create Backup"

2. **Select SMA Format**
   - Choose "SMA - Shelly Manager Archive" as the output format
   - SMA-specific configuration options will appear

3. **Configure SMA Options**
   - **Compression**: Enable/disable compression and set compression level
   - **Data Integrity**: Include SHA-256 checksums for validation
   - **Data Sections**: Select which data sections to include
   - **Metadata**: Optionally specify creator and export type

4. **Device Selection**
   - Choose to export all devices or select specific devices
   - Template dependencies will be automatically included

5. **Review and Create**
   - Review size estimates and configuration
   - Click "Create Backup" to generate the SMA file

### Importing SMA Files

1. **Open Import Dialog**
   - Use the SMA import functionality (implementation depends on UI structure)
   - Or upload SMA files through the general import interface

2. **Select SMA File**
   - Click to browse or drag and drop .sma files
   - File validation occurs automatically

3. **Preview File Contents**
   - Click "Preview File" to analyze the SMA file
   - Review validation status, content summary, and potential conflicts

4. **Configure Import Options**
   - **Validation**: Enable checksum and structure validation
   - **Merge Strategy**: Choose how to handle conflicts (overwrite/merge/skip)
   - **Sections**: Select which data sections to import
   - **Safety**: Enable dry run and backup before import

5. **Execute Import**
   - Review final configuration
   - Click "Import Data" or "Run Dry Run" to proceed

## File Structure

### SMA File Format

- **Extension**: `.sma`
- **Format**: Compressed JSON (Gzip)
- **Encoding**: UTF-8
- **MIME Type**: `application/octet-stream`

### Content Structure

```typescript
interface SMAArchive {
  sma_version: string        // Format version (currently "1.0")
  format_version: string     // Schema version (currently "2024.1")
  metadata: {
    export_id: string
    created_at: string
    created_by?: string
    export_type: 'manual' | 'scheduled' | 'api'
    system_info: { ... }
    integrity: {
      checksum: string     // SHA-256 of content
      record_count: number
      file_count: number
    }
  }
  devices: Device[]
  templates: Template[]
  discovered_devices: DiscoveredDevice[]
  network_settings: NetworkSettings
  plugin_configurations: PluginConfig[]
  system_settings: SystemSettings
}
```

## Configuration Options

### Export Configuration

```typescript
interface SMAExportConfiguration {
  // Compression settings
  compression: boolean              // Enable Gzip compression
  compression_level: number         // Compression level 1-9
  
  // Integrity settings
  include_checksums: boolean        // Include SHA-256 checksums
  
  // Data selection
  include_discovered: boolean       // Include discovered devices
  include_network_settings: boolean // Include network configuration
  include_plugin_configs: boolean   // Include plugin settings
  include_system_settings: boolean  // Include system settings
  
  // Metadata
  created_by?: string               // Export creator identifier
  export_type: 'manual' | 'scheduled' | 'api'
}
```

### Import Configuration

```typescript
interface SMAImportConfiguration {
  // Validation settings
  validate_checksums: boolean       // Verify data integrity
  validate_structure: boolean       // Verify format structure
  
  // Import behavior
  dry_run: boolean                  // Preview mode
  backup_before: boolean            // Create backup first
  
  // Conflict resolution
  merge_strategy: 'overwrite' | 'merge' | 'skip'
  
  // Section selection
  import_sections: string[]         // Which sections to import
}
```

## Error Handling

### Common Errors

1. **File Format Errors**
   - Invalid .sma file extension
   - Corrupted or invalid Gzip compression
   - Malformed JSON structure

2. **Validation Errors**
   - Checksum mismatch (data corruption)
   - Unsupported SMA version
   - Missing required fields
   - Invalid data types

3. **Import Conflicts**
   - Duplicate device MAC addresses
   - Template name conflicts
   - Dependency resolution issues

### Error Recovery

- **Validation Failures**: Fix source data or disable validation
- **Conflicts**: Choose appropriate merge strategy
- **Corruption**: Re-export from source or use backup
- **Version Issues**: Update Shelly Manager or export from compatible version

## Performance Considerations

### File Sizes

Typical SMA file sizes (compressed):

- **Small setup** (1-20 devices): 50-500 KB
- **Medium setup** (20-100 devices): 500KB - 2MB  
- **Large setup** (100+ devices): 2-10 MB
- **Enterprise setup** (500+ devices): 10-50 MB

### Compression Ratios

- **Level 1 (Fast)**: ~15% reduction, very fast
- **Level 6 (Balanced)**: ~35% reduction, good balance
- **Level 9 (Best)**: ~45% reduction, slower processing

### Processing Performance

- **Export Time**: 1-5 seconds for typical setups
- **Import Time**: 2-10 seconds depending on data size and conflicts
- **Memory Usage**: 2-3x file size during processing

## Security Considerations

### Data Sensitivity

SMA files contain potentially sensitive information:

- Device IP addresses and network topology
- WiFi network names and passwords
- MQTT credentials and API keys
- Plugin configurations with secrets

### Security Best Practices

1. **Encryption**: Use external encryption for sensitive environments
2. **Access Control**: Restrict access to SMA files
3. **Secure Transmission**: Use secure channels for file transfer
4. **Storage**: Store in secure, access-controlled locations
5. **Cleanup**: Securely delete temporary SMA files
6. **Audit**: Log all import/export operations

## Troubleshooting

### Common Issues

1. **"Invalid SMA file" error**
   - Verify file has .sma extension
   - Check file isn't corrupted during transfer
   - Ensure file was exported from compatible version

2. **Checksum validation failed**
   - File was corrupted during transfer
   - Re-download or re-export the file
   - Disable checksum validation if source is trusted

3. **Import conflicts detected**
   - Review conflict details in preview
   - Choose appropriate merge strategy
   - Import selectively to avoid problematic sections

4. **Template dependency errors**
   - Ensure referenced templates are included in export
   - Import templates before devices
   - Use "merge" strategy to resolve dependencies

### Debug Information

Enable debug mode by:
1. Opening browser developer tools
2. Setting `localStorage.debug = 'sma:*'`
3. Refresh page and retry operation
4. Check console for detailed debug logs

## API Integration

### Backend Endpoints

The SMA format integrates with these backend endpoints:

- `POST /api/v1/export/sma` - Create SMA export
- `GET /api/v1/export/sma/{id}` - Get export result
- `GET /api/v1/export/sma/{id}/download` - Download SMA file
- `POST /api/v1/import/sma` - Import SMA file
- `POST /api/v1/import/sma-preview` - Preview SMA import

### Response Formats

All endpoints follow standard API response format with SMA-specific data structures as defined in the TypeScript interfaces.

## Browser Compatibility

### Required Features

- **File API**: For file reading and processing
- **Compression**: Gzip support via Pako library
- **Crypto**: SHA-256 hashing via crypto-browserify
- **Modern JavaScript**: ES2020+ features

### Supported Browsers

- Chrome 90+
- Firefox 88+  
- Safari 14+
- Edge 90+

### Limitations

- Large file processing may be slow on mobile devices
- Memory usage scales with file size
- Compression/decompression is CPU intensive

## Development

### Dependencies

- `pako`: Gzip compression/decompression
- `crypto-browserify`: SHA-256 hashing
- Vue 3, TypeScript, Pinia for frontend framework

### File Structure

```
src/utils/
├── sma-parser.ts           # SMA file parsing and validation
├── sma-generator.ts        # SMA file generation and compression
└── __tests__/
    ├── sma-parser.test.ts  # Parser unit tests
    └── sma-generator.test.ts # Generator unit tests

src/components/
├── SMAConfigForm.vue       # SMA export configuration
└── SMAImportForm.vue       # SMA import interface

src/api/
└── export.ts              # SMA API integration (extended)

src/stores/
├── export.ts              # Export state management (SMA extended)
└── import.ts              # Import state management (SMA extended)
```

### Testing

Run SMA-specific tests:

```bash
npm test -- sma
```

### Contributing

When contributing SMA-related features:

1. Follow existing TypeScript patterns
2. Add comprehensive tests for new functionality
3. Update this documentation for API changes
4. Consider backward compatibility
5. Test with various file sizes and scenarios

## Version History

- **v1.0**: Initial SMA format support
  - Basic export/import functionality
  - Compression and integrity validation
  - Vue.js frontend integration
  - Comprehensive test coverage

## Support

For issues related to SMA format support:

1. Check browser console for error details
2. Verify file integrity and format
3. Test with smaller data sets
4. Check backend logs for server-side issues
5. Report bugs with sample (anonymized) data files