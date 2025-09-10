# Shelly Manager UI Guide - Export/Import System

## Overview

The Shelly Manager UI provides a comprehensive, user-friendly interface for managing the export/import system. Built with Vue.js 3, TypeScript, and modern web technologies, the interface offers powerful features while maintaining accessibility and responsiveness across all devices.

## Table of Contents

- [Key Features](#key-features)
- [Navigation and Layout](#navigation-and-layout)
- [Export Management](#export-management)
- [Import Management](#import-management)
- [Plugin Management](#plugin-management)
- [Metrics Dashboard](#metrics-dashboard)
- [Notification System](#notification-system)
- [Responsive Design](#responsive-design)
- [Accessibility Features](#accessibility-features)
- [Troubleshooting](#troubleshooting)

## Key Features

### 🎯 Core UI Components

#### Export/Import Dashboard
- **Unified Interface**: Single dashboard for all export/import operations
- **Real-time Status**: Live updates on operation progress
- **History View**: Comprehensive history with search and filtering
- **Quick Actions**: One-click access to common operations

#### Schema-Driven Forms
- **Dynamic Forms**: Forms generated from backend schemas
- **Real-time Validation**: Instant feedback on input validation
- **Preview Capabilities**: See results before execution
- **Smart Defaults**: Intelligent default values based on context

#### File Management
- **Drag & Drop**: Intuitive file upload with drag-and-drop support
- **Progress Tracking**: Visual progress indicators for large operations
- **Download Management**: Secure file downloads with expiration
- **File Validation**: Client-side file validation before upload

### 🛠️ Advanced Features

#### Plugin Integration
- **Plugin Discovery**: Automatic detection of available plugins
- **Configuration UI**: Dynamic configuration forms for each plugin
- **Status Monitoring**: Real-time plugin status and health checks
- **Custom Plugins**: Support for custom plugin registration

#### Metrics Integration
- **WebSocket Connection**: Real-time metrics via WebSocket
- **Dashboard Widgets**: Configurable dashboard widgets
- **Alert System**: Visual alerts and notifications
- **Performance Monitoring**: System performance visualization

## Navigation and Layout

### Main Navigation

The Shelly Manager UI follows a consistent navigation pattern:

```
Header Navigation:
├── Dashboard
├── Devices
├── Templates
├── Export/Import ⭐ NEW
│   ├── Export Operations
│   ├── Import Operations
│   ├── Plugin Management
│   └── History & Statistics
├── Metrics ⭐ NEW
├── Notifications ⭐ NEW
└── Settings
```

### Export/Import Section

#### Navigation Structure
- **Export Operations**: Create and manage export operations
- **Import Operations**: Upload and process import files
- **Plugin Management**: Configure and manage export/import plugins
- **History & Statistics**: View operation history and analytics

#### Breadcrumb Navigation
```
Home > Export/Import > Export Operations > Create Export
Home > Export/Import > Import Operations > Upload File
Home > Export/Import > Plugin Management > Configure Terraform
```

### Responsive Layout

#### Desktop Layout (≥1024px)
- **Sidebar Navigation**: Collapsible sidebar with full menu
- **Main Content Area**: Full-width content with sidebars
- **Action Panels**: Right-side panels for quick actions
- **Modal Dialogs**: Large modals for complex operations

#### Tablet Layout (768px - 1023px)
- **Collapsible Sidebar**: Auto-hiding sidebar navigation
- **Stacked Content**: Content areas stack vertically when needed
- **Touch-Optimized**: Larger touch targets and spacing
- **Swipe Gestures**: Support for swipe navigation

#### Mobile Layout (≤767px)
- **Bottom Navigation**: Primary navigation at bottom
- **Full-Screen Views**: Content uses full screen width
- **Mobile-First Forms**: Optimized form layouts for mobile
- **Progressive Enhancement**: Essential features first

## Export Management

### Export Operation Workflow

#### 1. Create New Export

**Navigation**: Export/Import > Export Operations > Create Export

**Workflow Steps**:
1. **Format Selection**
   - Visual format picker with descriptions
   - Format-specific configuration options
   - Real-time validation of format requirements

2. **Data Selection**
   - Device filter with multi-select capabilities
   - Template selection with dependency checking
   - Date range filters for time-based exports

3. **Configuration**
   - Format-specific configuration forms
   - Output destination selection
   - Compression and optimization settings

4. **Preview**
   - Export preview with record counts
   - File size estimation
   - Validation warnings and errors

5. **Execution**
   - Real-time progress tracking
   - Cancel capability during operation
   - Automatic download on completion

#### Export Form Components

##### Format Selection Component
```vue
<ExportFormatSelector
  :formats="availableFormats"
  v-model="selectedFormat"
  :show-descriptions="true"
  :allow-custom="false"
/>
```

Features:
- Visual format cards with icons and descriptions
- Real-time validation of format availability
- Context-sensitive help and documentation links
- Support for custom format plugins

##### Device Filter Component
```vue
<DeviceFilterForm
  :devices="devices"
  v-model="selectedDevices"
  :show-filters="true"
  :allow-bulk-select="true"
/>
```

Features:
- Search and filter devices by name, type, status
- Bulk selection with select all/none options
- Visual device status indicators
- Device count and selection summary

##### Export Configuration Component
```vue
<ExportConfigForm
  :format="selectedFormat"
  :schema="formatSchema"
  v-model="exportConfig"
  :show-preview="true"
/>
```

Features:
- Dynamic form generation from backend schemas
- Real-time validation with error highlighting
- Context-sensitive help tooltips
- Advanced options with expand/collapse

#### Export Preview Modal

The export preview provides a comprehensive view of what will be exported:

```typescript
interface ExportPreview {
  success: boolean
  record_count: number
  estimated_size: number
  warnings: string[]
  summary: {
    devices: number
    templates: number
    configurations: number
  }
  validation_errors: ValidationError[]
}
```

**Preview Features**:
- **Record Summary**: Detailed breakdown of what will be exported
- **Size Estimation**: Accurate file size prediction
- **Validation Results**: Comprehensive validation with error details
- **Warning System**: Non-blocking warnings with explanations
- **Export Options**: Final configuration review and modification

### Export History and Monitoring

#### Export History Table

The export history provides comprehensive tracking of all export operations:

**Table Columns**:
- **Date/Time**: Operation timestamp with timezone
- **Format**: Export format with icon
- **Status**: Visual status indicators (success, failed, in-progress)
- **Records**: Number of records exported
- **File Size**: Actual file size with compression ratio
- **Duration**: Operation duration
- **Actions**: Download, retry, delete options

**Filtering and Search**:
- **Date Range**: Filter exports by date range
- **Format Filter**: Filter by export format
- **Status Filter**: Filter by operation status
- **Search**: Full-text search across export metadata

#### Export Statistics Dashboard

Visual dashboard showing export analytics:

**Metrics Displayed**:
- Total exports by format (pie chart)
- Export success rate over time (line chart)
- Average export size by format (bar chart)
- Export frequency heatmap (calendar view)
- Top exported devices (list view)

## Import Management

### Import Operation Workflow

#### 1. File Upload and Validation

**Navigation**: Export/Import > Import Operations > Upload File

**Upload Methods**:
- **Drag & Drop**: Drag files directly onto upload zone
- **File Browser**: Click to browse and select files
- **URL Import**: Import from remote URLs (if configured)

**Validation Process**:
1. **File Format Detection**: Automatic format detection
2. **Size Validation**: Check file size limits
3. **Structure Validation**: Validate file structure and schema
4. **Security Scanning**: Check for potential security issues

#### 2. Import Preview and Validation

After file upload, the system provides a comprehensive preview:

```typescript
interface ImportPreview {
  success: boolean
  import_id: string
  records_to_import: number
  conflicts_detected: number
  changes: ImportChange[]
  summary: {
    will_create: number
    will_update: number
    will_delete: number
  }
  warnings: string[]
  validation_errors: ValidationError[]
}
```

**Preview Components**:

##### Import Summary Card
```vue
<ImportSummaryCard
  :preview="importPreview"
  :show-details="true"
  :allow-expand="true"
/>
```

Features:
- High-level import statistics
- Conflict detection and resolution suggestions
- Warning and error summaries
- Expandable detail views

##### Change Preview Table
```vue
<ImportChangeTable
  :changes="importPreview.changes"
  :show-filters="true"
  :allow-sorting="true"
/>
```

Features:
- Detailed list of all changes to be made
- Visual indicators for create/update/delete operations
- Conflict highlighting with resolution options
- Sortable and filterable change list

##### Conflict Resolution Panel
```vue
<ConflictResolutionPanel
  :conflicts="detectedConflicts"
  v-model="resolutionStrategy"
  :allow-per-item="true"
/>
```

Features:
- Individual conflict resolution options
- Bulk resolution strategies
- Preview of resolution outcomes
- Undo/reset conflict resolutions

#### 3. Import Configuration

Before executing the import, users can configure various options:

**Configuration Options**:
- **Dry Run**: Preview mode without making changes
- **Backup Before Import**: Create automatic backup
- **Conflict Resolution**: Choose default resolution strategy
- **Section Selection**: Choose which sections to import
- **Validation Level**: Set validation strictness

#### 4. Import Execution

**Execution Features**:
- **Real-time Progress**: Live progress bar with current operation
- **Cancellation**: Ability to cancel import during execution
- **Step-by-step Updates**: Detailed progress information
- **Error Handling**: Graceful error handling with recovery options

### Import History and Monitoring

#### Import History Interface

Similar to export history, with import-specific features:

**Additional Columns**:
- **Source File**: Original import file information
- **Conflicts**: Number of conflicts detected and resolved
- **Changes Applied**: Summary of changes made
- **Rollback**: Availability of rollback options

## Plugin Management

### Plugin Discovery and Configuration

#### Plugin Management Interface

**Navigation**: Export/Import > Plugin Management

**Plugin List View**:
- **Available Plugins**: List of all available plugins
- **Status Indicators**: Enabled/disabled status with health checks
- **Configuration Access**: Quick access to plugin configuration
- **Documentation Links**: Links to plugin-specific documentation

#### Plugin Configuration Forms

Each plugin has its own configuration interface:

```vue
<PluginConfigForm
  :plugin="selectedPlugin"
  :schema="pluginSchema"
  v-model="pluginConfig"
  :show-validation="true"
/>
```

**Configuration Features**:
- **Dynamic Forms**: Forms generated from plugin schemas
- **Validation**: Real-time validation of plugin settings
- **Testing**: Built-in test functionality for plugin validation
- **Reset Options**: Easy reset to default configurations

#### Plugin Testing Interface

**Test Capabilities**:
- **Connection Testing**: Test external service connections
- **Format Validation**: Validate export/import format support
- **Sample Operations**: Run sample export/import operations
- **Error Simulation**: Test error handling capabilities

### Built-in Plugin Interfaces

#### SMA Plugin Configuration
- **Compression Settings**: Compression level and options
- **Metadata Options**: Export metadata configuration
- **Validation Settings**: Integrity checking options
- **Performance Tuning**: Memory and processing options

#### Terraform Plugin Configuration
- **Provider Settings**: Terraform provider configuration
- **Resource Naming**: Resource naming conventions
- **Module Organization**: Module structure preferences
- **Variable Management**: Variable handling options

#### Ansible Plugin Configuration
- **Playbook Structure**: Playbook organization preferences
- **Task Grouping**: Task organization strategies
- **Variable Management**: Ansible variable handling
- **Inventory Integration**: Inventory file generation options

## Metrics Dashboard

### Real-time Metrics Interface

#### Dashboard Layout

**Navigation**: Metrics

**Dashboard Sections**:
- **System Overview**: System health and status
- **Export/Import Metrics**: Operation statistics and trends
- **Device Metrics**: Device status and performance
- **Performance Monitoring**: System performance indicators

#### WebSocket Integration

The metrics dashboard uses WebSocket for real-time updates:

```typescript
interface MetricsMessage {
  type: 'dashboard' | 'alert' | 'update'
  timestamp: string
  data: {
    system_status: SystemMetrics
    device_metrics: DeviceMetrics[]
    export_stats: ExportStatistics
    import_stats: ImportStatistics
  }
}
```

**Real-time Features**:
- **Live Updates**: Metrics update in real-time
- **Alert Notifications**: Visual alerts for important events
- **Performance Graphs**: Live performance charting
- **Status Indicators**: Real-time status changes

#### Dashboard Widgets

##### System Status Widget
```vue
<SystemStatusWidget
  :metrics="systemMetrics"
  :show-details="true"
  :refresh-interval="5000"
/>
```

Features:
- CPU, memory, and disk usage
- Service health indicators
- Uptime and availability metrics
- System load and performance

##### Export/Import Statistics Widget
```vue
<ExportImportStatsWidget
  :stats="exportImportStats"
  :time-range="timeRange"
  :show-trends="true"
/>
```

Features:
- Operation success rates
- Performance trends
- Volume statistics
- Error rate monitoring

##### Device Health Widget
```vue
<DeviceHealthWidget
  :devices="deviceMetrics"
  :show-offline="true"
  :alert-thresholds="alertConfig"
/>
```

Features:
- Device online/offline status
- Performance metrics per device
- Alert conditions and thresholds
- Health trend analysis

## Notification System

### Notification Management Interface

#### Notification Configuration

**Navigation**: Notifications

**Configuration Sections**:
- **Channels**: Configure notification channels (email, webhook, Slack)
- **Rules**: Define notification rules and triggers
- **History**: View notification history and delivery status
- **Testing**: Test notification delivery

#### Channel Configuration

##### Email Channel Configuration
```vue
<EmailChannelConfig
  v-model="emailConfig"
  :show-test="true"
  :allow-templates="true"
/>
```

Features:
- SMTP server configuration
- Recipient management
- Email template customization
- Delivery testing

##### Webhook Channel Configuration
```vue
<WebhookChannelConfig
  v-model="webhookConfig"
  :show-test="true"
  :allow-headers="true"
/>
```

Features:
- Webhook URL configuration
- HTTP header customization
- Authentication settings
- Payload testing

##### Slack Channel Configuration
```vue
<SlackChannelConfig
  v-model="slackConfig"
  :show-test="true"
  :channel-browser="true"
/>
```

Features:
- Slack workspace integration
- Channel selection
- Message formatting options
- Delivery testing

#### Notification Rules

##### Rule Configuration Interface
```vue
<NotificationRuleForm
  v-model="ruleConfig"
  :channels="availableChannels"
  :triggers="availableTriggers"
/>
```

Features:
- Trigger condition configuration
- Channel assignment
- Rate limiting settings
- Schedule configuration

##### Rule Testing
- **Test Notifications**: Send test notifications to verify configuration
- **Condition Simulation**: Simulate trigger conditions
- **Delivery Verification**: Verify notification delivery
- **Performance Testing**: Test notification performance under load

## Responsive Design

### Breakpoint Strategy

The UI uses a mobile-first responsive design strategy:

```scss
// Breakpoints
$mobile: 320px;
$tablet: 768px;
$desktop: 1024px;
$large: 1440px;

// Media queries
@media (min-width: $tablet) { /* Tablet styles */ }
@media (min-width: $desktop) { /* Desktop styles */ }
@media (min-width: $large) { /* Large screen styles */ }
```

### Component Responsiveness

#### Form Components

**Mobile Optimization**:
- **Stacked Layout**: Form fields stack vertically on mobile
- **Touch Targets**: Minimum 44px touch targets
- **Input Optimization**: Appropriate input types for mobile keyboards
- **Scroll Optimization**: Smooth scrolling and proper viewport handling

**Tablet Optimization**:
- **Two-Column Layout**: Forms use two-column layout when space allows
- **Touch-Friendly**: Optimized for touch interaction
- **Keyboard Support**: Full keyboard navigation support
- **Orientation Support**: Adapts to portrait/landscape changes

#### Table Components

**Mobile Strategy**:
- **Card Layout**: Tables transform to card layout on mobile
- **Essential Data**: Show only essential columns on small screens
- **Expandable Rows**: Tap to expand for full details
- **Horizontal Scrolling**: Allow horizontal scrolling for complex tables

**Tablet Strategy**:
- **Condensed Layout**: Reduced padding and font sizes
- **Touch-Friendly**: Larger touch targets for interactive elements
- **Priority Columns**: Show most important columns first
- **Overflow Handling**: Graceful handling of content overflow

#### Dashboard Components

**Responsive Grid**:
```typescript
// Grid configuration
const gridConfig = {
  mobile: { cols: 1, gap: 16 },
  tablet: { cols: 2, gap: 20 },
  desktop: { cols: 3, gap: 24 },
  large: { cols: 4, gap: 32 }
}
```

**Widget Adaptation**:
- **Size Scaling**: Widgets scale appropriately for screen size
- **Content Priority**: Most important content shown first
- **Interaction Optimization**: Touch-friendly on mobile, hover on desktop
- **Performance**: Reduced animation complexity on mobile

## Accessibility Features

### WCAG 2.1 AA Compliance

The UI is designed to meet WCAG 2.1 AA accessibility standards:

#### Keyboard Navigation

**Navigation Features**:
- **Tab Order**: Logical tab order throughout the interface
- **Skip Links**: Skip to main content links
- **Focus Indicators**: Visible focus indicators on all interactive elements
- **Keyboard Shortcuts**: Consistent keyboard shortcuts for common actions

**Interactive Elements**:
- **Button Accessibility**: All buttons properly labeled and accessible
- **Form Controls**: Proper labeling and description of all form controls
- **Modal Dialogs**: Proper focus management in modal dialogs
- **Menu Navigation**: Full keyboard navigation of all menus

#### Screen Reader Support

**Semantic HTML**:
- **Proper Headings**: Hierarchical heading structure (h1-h6)
- **Landmarks**: Proper use of landmark roles (main, nav, aside)
- **Lists**: Semantic list markup for grouped content
- **Tables**: Proper table headers and captions

**ARIA Attributes**:
- **Labels**: aria-label and aria-labelledby for complex elements
- **Descriptions**: aria-describedby for additional context
- **States**: aria-expanded, aria-selected for dynamic content
- **Live Regions**: aria-live for dynamic content updates

#### Visual Accessibility

**Color and Contrast**:
- **Color Contrast**: Minimum 4.5:1 contrast ratio for normal text
- **Color Independence**: Information not conveyed by color alone
- **High Contrast Mode**: Support for high contrast themes
- **Color Blindness**: Interface works for color-blind users

**Typography**:
- **Font Sizing**: Scalable fonts that work up to 200% zoom
- **Line Height**: Adequate line height for readability
- **Font Choices**: Clear, readable fonts throughout
- **Text Spacing**: Adequate spacing between text elements

### Accessibility Testing

**Automated Testing**:
- **axe-core Integration**: Automated accessibility testing in development
- **Lighthouse Audits**: Regular accessibility audits
- **Pa11y Integration**: Continuous accessibility monitoring
- **ESLint Rules**: Accessibility linting rules in development

**Manual Testing**:
- **Screen Reader Testing**: Regular testing with NVDA, JAWS, VoiceOver
- **Keyboard Testing**: Comprehensive keyboard-only navigation testing
- **Mobile Accessibility**: Testing with mobile screen readers
- **User Testing**: Regular testing with disabled users

## Troubleshooting

### Common UI Issues

#### Performance Issues

##### Slow Loading
**Symptoms**: Pages load slowly or appear to hang
**Causes**: Large data sets, network issues, server problems
**Solutions**:
1. Check browser network tab for slow requests
2. Verify server status and performance
3. Clear browser cache and cookies
4. Disable browser extensions temporarily

##### Memory Issues
**Symptoms**: Browser becomes unresponsive, high memory usage
**Causes**: Memory leaks, large data sets, browser limitations
**Solutions**:
1. Refresh the page to clear memory
2. Close unnecessary browser tabs
3. Check for browser memory leaks in developer tools
4. Reduce data set size or use pagination

#### Form and Input Issues

##### Validation Errors
**Symptoms**: Form validation fails unexpectedly
**Causes**: Client-server validation mismatch, network issues
**Solutions**:
1. Check browser console for validation errors
2. Verify form data matches server expectations
3. Try submitting with minimal required data
4. Check network connectivity and server status

##### File Upload Issues
**Symptoms**: File uploads fail or hang
**Causes**: File size limits, format restrictions, network issues
**Solutions**:
1. Verify file size is within limits
2. Check file format is supported
3. Try uploading a smaller test file
4. Check network connection stability

#### WebSocket Connection Issues

##### Connection Failures
**Symptoms**: Real-time updates not working, WebSocket errors
**Causes**: Network issues, server problems, authentication issues
**Solutions**:
1. Check browser console for WebSocket errors
2. Verify admin API key configuration
3. Check network firewall and proxy settings
4. Try refreshing the page to reconnect

##### Authentication Issues
**Symptoms**: WebSocket connection denied, 401 errors
**Causes**: Invalid or expired admin API key
**Solutions**:
1. Verify admin API key is properly configured
2. Check key has not expired
3. Try logging out and back in
4. Contact administrator for key verification

### Debug Information

#### Browser Developer Tools

**Console Debugging**:
```javascript
// Enable debug logging
localStorage.debug = 'shelly-manager:*'

// Enable specific component debugging
localStorage.debug = 'export:*,import:*,websocket:*'

// View current debug settings
console.log(localStorage.debug)
```

**Network Debugging**:
1. Open browser developer tools (F12)
2. Navigate to Network tab
3. Filter by API requests
4. Check for failed requests or slow responses
5. Examine request/response headers and payloads

**Performance Debugging**:
1. Open Performance tab in developer tools
2. Record performance while using the interface
3. Look for long-running tasks or memory leaks
4. Check for excessive DOM updates or reflows

#### Application State Debugging

**Vue DevTools**:
- Install Vue DevTools browser extension
- Inspect component state and props
- Monitor Pinia store state changes
- Track component re-renders and updates

**API State Debugging**:
```typescript
// Check current API state
console.log(useApiStore().state)

// Monitor WebSocket state
console.log(useWebSocketStore().connectionState)

// View export/import state
console.log(useExportStore().currentOperation)
```

### Error Recovery

#### Automatic Recovery

**Connection Recovery**:
- WebSocket automatically reconnects on connection loss
- API requests automatically retry on network failures
- Form state is preserved during temporary disconnections

**State Recovery**:
- Application state is persisted in localStorage
- Form data is automatically saved as user types
- Operation progress is maintained across page refreshes

#### Manual Recovery

**Clear Application State**:
```javascript
// Clear all stored application state
localStorage.clear()
sessionStorage.clear()

// Clear specific state
localStorage.removeItem('shelly-manager-state')
```

**Reset to Default Configuration**:
1. Navigate to Settings
2. Click "Reset to Defaults"
3. Confirm reset operation
4. Refresh page to apply changes

### Getting Help

#### Documentation Resources
- **User Guide**: This comprehensive UI guide
- **API Documentation**: Complete API reference
- **Plugin Documentation**: Plugin-specific guides
- **Video Tutorials**: Step-by-step video guides

#### Support Channels
- **GitHub Issues**: Report bugs and feature requests
- **Community Forum**: Community support and discussions
- **Documentation Wiki**: Community-maintained documentation
- **Developer Chat**: Real-time developer support

#### Reporting Issues

When reporting UI issues, please include:
1. **Browser Information**: Browser type, version, and platform
2. **Steps to Reproduce**: Detailed steps to reproduce the issue
3. **Expected Behavior**: What should happen
4. **Actual Behavior**: What actually happens
5. **Console Logs**: Any error messages from browser console
6. **Screenshots**: Visual evidence of the issue
7. **Configuration**: Relevant configuration settings

---

**Documentation Version**: 1.0  
**Last Updated**: 2024-01-15  
**Compatible Versions**: Shelly Manager UI v0.5.4+  
**Framework**: Vue.js 3 + TypeScript + Vite