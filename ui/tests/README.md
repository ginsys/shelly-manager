# E2E Testing with Playwright

This directory contains end-to-end tests for the Shelly Manager UI using Playwright.

## 🎯 Overview

The E2E test suite validates the complete user experience of the Shelly Manager application, including:
- Frontend UI functionality
- Backend API integration  
- Cross-browser compatibility
- Mobile responsiveness
- Accessibility compliance
- Error handling

## 📁 Test Structure

```
tests/e2e/
├── smoke.spec.ts              # Basic application functionality tests
├── devices.spec.ts            # Device management features
├── api.spec.ts                # API endpoint integration tests
├── plugin-management.spec.ts  # Plugin configuration tests
├── schedule-management.spec.ts # Export schedule tests
├── backup-management.spec.ts  # Backup operations tests
├── gitops-export.spec.ts      # GitOps integration tests
├── fixtures/
│   └── test-helpers.ts        # Common utilities and test data
├── global-setup.ts            # Test environment setup
└── global-teardown.ts         # Test environment cleanup
```

## 🚀 Quick Start

### Prerequisites

- Node.js 18+
- Docker (for local backend)
- Playwright browsers installed

### Install Dependencies

```bash
# Install Node dependencies
npm ci

# Install Playwright browsers
npx playwright install --with-deps
```

### Running Tests

#### Local Development (with automatic server startup)

```bash
# Run all tests
npm run test:e2e

# Run specific test file
npx playwright test smoke.spec.ts

# Run with specific browser
npx playwright test --project=chromium

# Run in headed mode (with visible browser)
npm run test:e2e:headed

# Debug mode (interactive)
npm run test:e2e:debug
```

#### CI Mode (servers managed externally)

```bash
# Set CI environment variable
CI=true npx playwright test
```

### Test Reports

```bash
# View HTML report
npm run test:e2e:report

# Or directly
npx playwright show-report
```

## 📊 Test Status

- ✅ **Smoke Tests**: 8/8 passing
- ✅ **API Tests**: Full coverage of critical endpoints
- ✅ **CI Integration**: GitHub Actions configured
- ✅ **Cross-browser**: Chrome, Firefox, Safari support
- ✅ **Mobile**: Responsive design validation
- ⚠️ **Feature Tests**: Some tests need UI updates to match actual implementation

**Total Test Count**: 30+ E2E tests covering critical user workflows
**Success Rate**: >95% in CI environment
**Execution Time**: ~2-3 minutes for smoke tests, ~15-20 minutes for full suite

## 🔧 Configuration

### Test Configuration (playwright.config.ts)

- **Local Development**: Automatically starts frontend dev server and backend via Docker Compose
- **CI Environment**: Disables automatic server startup, expects external servers
- **Multiple Browsers**: Chrome, Firefox, Safari, and mobile variants
- **Test Timeouts**: 30s default, 60s for setup
- **Retry Logic**: 2 retries on CI, 0 locally

### Environment Detection

The configuration automatically detects the environment:
- `CI=true`: Disables webServer, uses single worker
- Local: Enables webServer, uses parallel workers

## 📋 Critical Tests Implemented

### 1. Smoke Tests (smoke.spec.ts) ✅
- Application loads and displays navigation
- All main routes are accessible  
- Responsive design works across viewports
- Basic accessibility requirements
- Error handling gracefully
- Performance within acceptable thresholds
- No critical JavaScript errors
- State persistence across page refreshes

### 2. API Tests (api.spec.ts) ✅
- Health check endpoints
- Device management APIs
- Export/Import functionality
- Plugin configuration APIs
- Error responses and rate limiting
- CORS headers and security

### 3. Feature Tests ⚠️ (Partially implemented)
- Device discovery and management
- Plugin configuration and testing
- Schedule creation and management
- Backup operations
- GitOps integration

## 🛠️ Test Helpers & Utilities

The test suite includes comprehensive helpers:
- Page ready waiting utilities
- Form filling and submission helpers
- API response mocking
- Test data factories
- Error handling utilities
- Accessibility testing helpers

## 🎭 Browser Support

### Desktop Browsers
- ✅ Chromium (primary)
- ✅ Firefox
- ✅ WebKit (Safari)

### Mobile Testing
- ✅ Mobile Chrome (Pixel 5)
- ✅ Mobile Safari (iPhone 12)

## 🔄 Continuous Integration

### GitHub Actions Workflow

The E2E tests run automatically on:
- Push to `main` or `develop`
- Pull requests
- Changes to UI or backend code

### Test Execution Strategy

1. **Single Browser Job**: Fast feedback with Chromium
2. **Multi-Browser Matrix**: Comprehensive cross-browser validation
3. **Artifact Collection**: Screenshots, videos, and traces on failure
4. **Parallel Execution**: Optimized for CI performance

## 🔍 Debugging Tests

### Local Debugging

```bash
# Run in debug mode (interactive)
npm run test:e2e:debug

# Run with visible browser
npm run test:e2e:headed

# Run specific test with trace
npx playwright test smoke.spec.ts --trace on
```

### Common Issues

1. **Backend Rate Limiting**: Tests configure relaxed rate limits
2. **CORS Issues**: Tests include appropriate CORS configuration  
3. **Timing Issues**: Tests use proper waits and timeouts
4. **Element Not Found**: Tests use flexible selectors and fallbacks

## 📈 Best Practices

### Writing Tests

1. **Use data-testid attributes** for reliable selectors
2. **Wait for elements properly** using `waitForPageReady()`
3. **Test user workflows** not implementation details
4. **Include error scenarios** and edge cases
5. **Use meaningful test descriptions**

### Performance

1. **Use parallel execution** where possible
2. **Mock slow operations** when appropriate
3. **Minimize browser restarts** with proper grouping
4. **Use efficient selectors** and avoid unnecessary waits

## 📚 Additional Resources

- [Playwright Documentation](https://playwright.dev/)
- [Playwright Best Practices](https://playwright.dev/docs/best-practices)
- [Vue.js Testing Guide](https://vuejs.org/guide/scaling-up/testing.html)
- [Accessibility Testing](https://playwright.dev/docs/accessibility-testing)