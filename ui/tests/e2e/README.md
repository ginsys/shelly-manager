# E2E Testing Infrastructure

Comprehensive End-to-End testing setup for Shelly Manager using Playwright.

## Overview

This E2E testing infrastructure provides:
- ✅ **Multi-browser testing** (Chromium, Firefox, WebKit, Mobile)
- ✅ **API integration testing** with backend validation
- ✅ **WebSocket real-time features** testing
- ✅ **Export/Import workflows** comprehensive coverage
- ✅ **Responsive design** validation
- ✅ **CI/CD integration** with GitHub Actions

## Test Structure

```
tests/e2e/
├── README.md                 # This file
├── global-setup.ts          # Test environment setup
├── global-teardown.ts       # Test environment cleanup
├── export_history.spec.ts   # Export history functionality
├── export_preview.spec.ts   # Export preview forms
├── import_preview.spec.ts   # Import preview forms
├── metrics_dashboard.spec.ts # Metrics and WebSocket
└── api.spec.ts              # Backend API testing
```

## Running Tests

### Prerequisites

```bash
# Install dependencies
npm install

# Install Playwright browsers
npm run test:install
```

### Local Development

```bash
# Start backend (in separate terminal)
cd .. && make run

# Run all E2E tests
npm run test:e2e

# Run with UI (interactive mode)
npm run test:e2e:ui

# Run with browser visible
npm run test:e2e:headed

# Debug specific test
npm run test:e2e:debug -- tests/e2e/export_history.spec.ts
```

### Specific Test Categories

```bash
# Run only API tests
npx playwright test api.spec.ts

# Run only UI tests
npx playwright test --grep -v "API"

# Run specific browser
npx playwright test --project=chromium

# Run mobile tests
npx playwright test --project="Mobile Chrome"
```

## Test Configuration

### Playwright Config (`playwright.config.ts`)

- **Multi-browser support**: Chromium, Firefox, WebKit
- **Mobile testing**: Pixel 5, iPhone 12 simulation
- **API testing**: Separate project for backend validation
- **Auto-start services**: Frontend dev server + Docker backend
- **Rich reporting**: HTML, JSON, GitHub integration
- **Failure artifacts**: Screenshots, videos, traces

### Test Data Management

- **Global Setup**: Creates test devices via API
- **Global Teardown**: Cleans up test data
- **Isolation**: Each test runs independently
- **Mocking**: WebSocket and API error simulation

## Test Categories

### 1. Export History (`export_history.spec.ts`)
- ✅ Pagination functionality
- ✅ Plugin filtering
- ✅ Success/failure filtering
- ✅ Empty state handling
- ✅ Details view navigation
- ✅ Cross-page filter persistence

### 2. Export Preview (`export_preview.spec.ts`)
- ✅ Dynamic form generation from plugin schemas
- ✅ Plugin and format selection
- ✅ Configuration validation
- ✅ Preview generation and display
- ✅ Copy/download functionality
- ✅ localStorage persistence
- ✅ API error handling

### 3. Import Preview (`import_preview.spec.ts`)
- ✅ File upload functionality
- ✅ Text input with JSON validation
- ✅ Preview with create/update/skip counts
- ✅ Large file handling
- ✅ Import execution flow
- ✅ Warning display
- ✅ Configuration persistence

### 4. Metrics Dashboard (`metrics_dashboard.spec.ts`)
- ✅ WebSocket connection status
- ✅ Real-time chart updates
- ✅ Connection error handling
- ✅ Reconnection attempts
- ✅ Mobile responsiveness
- ✅ Health status indicators
- ✅ Data refresh functionality

### 5. API Integration (`api.spec.ts`)
- ✅ Health check endpoint
- ✅ Device management API
- ✅ Export/Import API endpoints
- ✅ Metrics API endpoints
- ✅ Plugin management API
- ✅ Error response handling
- ✅ Rate limiting
- ✅ CORS headers

## CI/CD Integration

### GitHub Actions (`../.github/workflows/e2e-tests.yml`)

**Two-tier testing strategy**:

1. **Full E2E Suite** (`e2e-tests`)
   - Docker Compose backend setup
   - PostgreSQL database
   - Complete workflow testing
   - Artifact collection on failure

2. **Cross-Browser Matrix** (`e2e-tests-matrix`) 
   - Chromium, Firefox, WebKit
   - Lightweight backend setup
   - Parallel execution
   - Per-browser artifact collection

**Triggers**:
- Push to `main`/`develop`
- PR to `main`/`develop`
- Changes to UI, backend, or workflows

## Test Data Strategy

### Setup Data
```javascript
// Global setup creates test devices
const testDevices = [
  {
    id: 'test-device-1',
    name: 'Test Shelly 1',
    type: 'SHSW-1',
    ip: '192.168.1.100',
    generation: 1
  },
  {
    id: 'test-device-2', 
    name: 'Test Shelly Plus 1',
    type: 'SNSW-001X16EU',
    ip: '192.168.1.101',
    generation: 2
  }
]
```

### Test Isolation
- Each test starts with clean state
- Global teardown removes test data
- No shared state between tests
- Independent browser contexts

## Debugging

### Common Issues

1. **Backend not ready**
   ```bash
   # Check backend status
   curl http://localhost:8080/api/v1/health
   
   # Start backend manually
   cd .. && make run
   ```

2. **WebSocket connection fails**
   ```bash
   # Check metrics endpoint
   curl http://localhost:8080/api/v1/metrics/status
   ```

3. **Test data issues**
   ```bash
   # Reset test database
   rm /tmp/shelly_test.db
   ```

### Debug Tools

```bash
# Run with browser open
npm run test:e2e:headed

# Interactive debugging
npm run test:e2e:debug

# Verbose output
npx playwright test --reporter=list

# Record test run
npx playwright test --record-video=on
```

### Test Reports

```bash
# View HTML report
npm run test:e2e:report

# CI artifacts location
test-results/
├── playwright-report/     # HTML report
├── results.json          # JSON results
├── screenshots/          # Failure screenshots
└── videos/              # Failure videos
```

## Best Practices

### Test Writing
- ✅ Use `data-testid` attributes for reliable element selection
- ✅ Handle optional elements with visibility checks
- ✅ Wait for network idle after navigation
- ✅ Use page object pattern for complex flows
- ✅ Mock external dependencies

### Performance
- ✅ Parallel test execution
- ✅ Browser reuse between tests
- ✅ Lightweight test data
- ✅ Selective test running
- ✅ Failure-only artifact collection

### Maintenance
- ✅ Regular browser updates
- ✅ Test data cleanup
- ✅ Configuration validation
- ✅ CI performance monitoring
- ✅ Cross-browser compatibility

## Metrics & Monitoring

### Test Coverage
- **UI Components**: Export/Import forms, Dashboard, History
- **API Endpoints**: 15+ endpoints tested
- **Browser Matrix**: 3 desktop + 2 mobile configurations
- **Error Scenarios**: Network failures, API errors, validation

### Performance Targets
- **Test Suite**: < 20 minutes full run
- **Individual Test**: < 2 minutes
- **Setup/Teardown**: < 30 seconds
- **CI Matrix**: < 30 minutes parallel

### Success Metrics
- ✅ **Functional Coverage**: All major user workflows
- ✅ **Cross-Browser**: 100% compatibility validation
- ✅ **API Integration**: Complete backend validation
- ✅ **Real-time Features**: WebSocket testing
- ✅ **Error Handling**: Comprehensive failure scenarios

## Future Enhancements

- [ ] **Visual Regression Testing**: Screenshot comparisons
- [ ] **Performance Testing**: Core Web Vitals monitoring
- [ ] **Load Testing**: Multiple concurrent users
- [ ] **Mobile App Testing**: Native mobile integration
- [ ] **A11y Testing**: Automated accessibility validation