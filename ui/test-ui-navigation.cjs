#!/usr/bin/env node

const { chromium } = require('playwright');

const routes = [
  { path: '/', name: 'devices', title: 'Devices' },
  { path: '/export/schedules', name: 'export-schedules', title: 'Schedule Management' },
  { path: '/export/backup', name: 'export-backup', title: 'Backup Management' },
  { path: '/export/gitops', name: 'export-gitops', title: 'GitOps Export' },
  { path: '/export/history', name: 'export-history', title: 'Export History' },
  { path: '/import/history', name: 'import-history', title: 'Import History' },
  { path: '/plugins', name: 'plugins', title: 'Plugin Management' },
  { path: '/dashboard', name: 'metrics', title: 'Metrics Dashboard' },
  { path: '/stats', name: 'stats', title: 'Statistics' },
  { path: '/admin', name: 'admin', title: 'Admin Settings' }
];

async function testRoutes() {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  const page = await context.newPage();
  
  const results = {
    passed: [],
    failed: [],
    warnings: []
  };

  console.log('🚀 Starting UI Navigation Tests');
  console.log('================================');

  for (const route of routes) {
    try {
      console.log(`\n🔍 Testing route: ${route.path}`);
      
      // Navigate to route
      const response = await page.goto(`http://localhost:5174${route.path}`, {
        waitUntil: 'networkidle',
        timeout: 10000
      });

      // Check if page loaded
      if (!response.ok()) {
        results.failed.push({
          route: route.path,
          error: `HTTP ${response.status()}: ${response.statusText()}`
        });
        console.log(`❌ HTTP error: ${response.status()}`);
        continue;
      }

      // Wait for main content to load
      try {
        await page.waitForSelector('main', { timeout: 5000 });
        console.log('✅ Main content loaded');
      } catch (error) {
        results.warnings.push({
          route: route.path,
          warning: 'Main content selector not found within 5s'
        });
        console.log('⚠️ Main content not found quickly');
      }

      // Check for common error indicators
      const has404 = await page.locator('text="404"').count() > 0;
      const hasError = await page.locator('text="Error"').count() > 0;
      const hasConsoleErrors = [];
      
      // Listen for console errors
      page.on('console', msg => {
        if (msg.type() === 'error') {
          hasConsoleErrors.push(msg.text());
        }
      });

      if (has404) {
        results.failed.push({
          route: route.path,
          error: '404 error found on page'
        });
        console.log('❌ 404 error detected');
        continue;
      }

      if (hasError) {
        results.warnings.push({
          route: route.path,
          warning: 'Error text found on page'
        });
        console.log('⚠️ Error text detected');
      }

      // Check navigation active state
      const navSelector = `nav a[href="${route.path}"]`;
      const navLink = page.locator(navSelector);
      const isActiveInNav = await navLink.count() > 0;
      
      if (isActiveInNav) {
        console.log('✅ Navigation link found');
      } else {
        console.log('ℹ️ No direct navigation link (may be in dropdown)');
      }

      // Take screenshot for visual verification
      await page.screenshot({
        path: `test-results/route-${route.name}-screenshot.png`,
        fullPage: false
      });

      results.passed.push({
        route: route.path,
        title: route.title,
        consoleErrors: hasConsoleErrors
      });
      console.log('✅ Route test passed');

    } catch (error) {
      results.failed.push({
        route: route.path,
        error: error.message
      });
      console.log(`❌ Route test failed: ${error.message}`);
    }
  }

  await browser.close();

  // Print summary
  console.log('\n\n📊 TEST SUMMARY');
  console.log('================');
  console.log(`✅ Passed: ${results.passed.length}`);
  console.log(`⚠️ Warnings: ${results.warnings.length}`);
  console.log(`❌ Failed: ${results.failed.length}`);

  if (results.failed.length > 0) {
    console.log('\n❌ FAILED ROUTES:');
    results.failed.forEach(fail => {
      console.log(`  ${fail.route}: ${fail.error}`);
    });
  }

  if (results.warnings.length > 0) {
    console.log('\n⚠️ WARNINGS:');
    results.warnings.forEach(warn => {
      console.log(`  ${warn.route}: ${warn.warning}`);
    });
  }

  if (results.passed.length > 0) {
    console.log('\n✅ PASSED ROUTES:');
    results.passed.forEach(pass => {
      console.log(`  ${pass.route}: ${pass.title}`);
      if (pass.consoleErrors.length > 0) {
        console.log(`    Console errors: ${pass.consoleErrors.join(', ')}`);
      }
    });
  }

  // Return exit code
  return results.failed.length > 0 ? 1 : 0;
}

if (require.main === module) {
  testRoutes()
    .then(exitCode => process.exit(exitCode))
    .catch(error => {
      console.error('❌ Test script failed:', error);
      process.exit(1);
    });
}

module.exports = { testRoutes };
