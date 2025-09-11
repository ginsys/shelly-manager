#!/usr/bin/env node

// Navigation validation script - tests actual navigation in browser
// Run this with: node validate-navigation.js

const puppeteer = require('puppeteer');

const testRoutes = [
  { path: '/', name: 'Devices', shouldContain: 'Devices' },
  { path: '/export/schedules', name: 'Schedule Management', shouldContain: 'Schedule Management' },
  { path: '/export/backup', name: 'Backup Management', shouldContain: 'Backup' },
  { path: '/export/gitops', name: 'GitOps Export', shouldContain: 'GitOps' },
  { path: '/export/history', name: 'Export History', shouldContain: 'Export History' },
  { path: '/import/history', name: 'Import History', shouldContain: 'Import History' },
  { path: '/plugins', name: 'Plugin Management', shouldContain: 'Plugin' },
  { path: '/dashboard', name: 'Metrics Dashboard', shouldContain: 'Metrics' },
  { path: '/admin', name: 'Admin Settings', shouldContain: 'Admin' }
];

async function validateNavigation() {
  const browser = await puppeteer.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const page = await browser.newPage();
  
  console.log('🧪 Testing Navigation Integration...\n');
  
  let passed = 0;
  let failed = 0;
  
  try {
    // Test if development server is running
    console.log('📡 Checking if dev server is running at http://localhost:5173...');
    await page.goto('http://localhost:5173', { waitUntil: 'networkidle2', timeout: 5000 });
    console.log('✅ Dev server is running\n');
    
    // Test navigation menu exists
    console.log('📋 Testing Navigation Menu Structure...');
    
    await page.waitForSelector('.nav', { timeout: 3000 });
    console.log('✅ Navigation menu found');
    
    // Check main navigation links
    const navLinks = await page.$$eval('.nav-link', links => 
      links.map(link => link.textContent.trim())
    );
    
    const expectedLinks = ['Devices', 'Export & Import', 'Plugins', 'Metrics', 'Admin'];
    for (const expectedLink of expectedLinks) {
      if (navLinks.includes(expectedLink)) {
        console.log(`✅ Found nav link: ${expectedLink}`);
        passed++;
      } else {
        console.log(`❌ Missing nav link: ${expectedLink}`);
        failed++;
      }
    }
    
    // Test dropdown menu
    console.log('\n🔽 Testing Dropdown Menu...');
    
    // Hover over dropdown to make it visible
    await page.hover('.dropdown-trigger');
    await page.waitForSelector('.dropdown-menu', { visible: true, timeout: 2000 });
    console.log('✅ Dropdown menu opens on hover');
    
    const dropdownItems = await page.$$eval('.dropdown-item', items => 
      items.map(item => item.textContent.trim())
    );
    
    const expectedDropdownItems = ['Schedule Management', 'Backup Management', 'GitOps Export', 'Export History', 'Import History'];
    for (const expectedItem of expectedDropdownItems) {
      if (dropdownItems.some(item => item.includes(expectedItem))) {
        console.log(`✅ Found dropdown item: ${expectedItem}`);
        passed++;
      } else {
        console.log(`❌ Missing dropdown item: ${expectedItem}`);
        failed++;
      }
    }
    
    // Test route navigation
    console.log('\n🚀 Testing Route Navigation...');
    
    for (const route of testRoutes) {
      try {
        await page.goto(`http://localhost:5173${route.path}`, { 
          waitUntil: 'networkidle2', 
          timeout: 5000 
        });
        
        // Wait a moment for the page to render
        await page.waitForTimeout(500);
        
        // Check if page loaded (not 404)
        const pageContent = await page.content();
        
        if (!pageContent.includes('404') && !pageContent.includes('not found')) {
          console.log(`✅ Route ${route.path} (${route.name}) loads successfully`);
          passed++;
          
          // Check for breadcrumb if not home page
          if (route.path !== '/') {
            try {
              await page.waitForSelector('.breadcrumb', { timeout: 1000 });
              console.log(`✅ Breadcrumb navigation found for ${route.name}`);
              passed++;
            } catch (e) {
              console.log(`⚠️  Breadcrumb not found for ${route.name} (this might be expected)`);
            }
          }
        } else {
          console.log(`❌ Route ${route.path} (${route.name}) returned 404`);
          failed++;
        }
      } catch (error) {
        console.log(`❌ Route ${route.path} (${route.name}) failed to load: ${error.message}`);
        failed++;
      }
    }
    
    // Test active states
    console.log('\n🎯 Testing Active Navigation States...');
    
    // Go to plugins page and check if nav link is active
    await page.goto('http://localhost:5173/plugins', { waitUntil: 'networkidle2' });
    
    const pluginsLinkClasses = await page.$eval('a[href="/plugins"]', el => el.className);
    if (pluginsLinkClasses.includes('active')) {
      console.log('✅ Active state works for plugins link');
      passed++;
    } else {
      console.log('❌ Active state not working for plugins link');
      failed++;
    }
    
    // Test dropdown active state
    await page.goto('http://localhost:5173/export/backup', { waitUntil: 'networkidle2' });
    
    const dropdownTriggerClasses = await page.$eval('.dropdown-trigger', el => el.className);
    if (dropdownTriggerClasses.includes('active')) {
      console.log('✅ Active state works for dropdown when on export page');
      passed++;
    } else {
      console.log('❌ Active state not working for dropdown');
      failed++;
    }
    
    // Test responsive navigation
    console.log('\n📱 Testing Responsive Navigation...');
    
    await page.setViewport({ width: 768, height: 1024 });
    await page.goto('http://localhost:5173', { waitUntil: 'networkidle2' });
    
    const navVisible = await page.$eval('.nav', nav => getComputedStyle(nav).display !== 'none');
    if (navVisible) {
      console.log('✅ Navigation visible on tablet viewport');
      passed++;
    } else {
      console.log('❌ Navigation hidden on tablet viewport');
      failed++;
    }
    
    await page.setViewport({ width: 320, height: 568 });
    
    const navStillVisible = await page.$eval('.nav', nav => getComputedStyle(nav).display !== 'none');
    if (navStillVisible) {
      console.log('✅ Navigation visible on mobile viewport');
      passed++;
    } else {
      console.log('❌ Navigation hidden on mobile viewport');
      failed++;
    }
    
  } catch (error) {
    console.error('❌ Test failed:', error.message);
    failed++;
  } finally {
    await browser.close();
  }
  
  console.log('\n📊 Test Results:');
  console.log(`✅ Passed: ${passed}`);
  console.log(`❌ Failed: ${failed}`);
  console.log(`📈 Success Rate: ${Math.round((passed / (passed + failed)) * 100)}%`);
  
  if (failed === 0) {
    console.log('\n🎉 All navigation tests passed! Navigation integration is working correctly.');
    process.exit(0);
  } else {
    console.log('\n⚠️  Some navigation tests failed. Please check the issues above.');
    process.exit(1);
  }
}

// Check if puppeteer is installed
try {
  require('puppeteer');
  validateNavigation();
} catch (error) {
  console.log('❌ Puppeteer not found. Please install it with: npm install --save-dev puppeteer');
  console.log('🔧 Alternatively, manually test navigation at http://localhost:5173');
  
  console.log('\n📋 Manual Testing Checklist:');
  console.log('1. Navigate to http://localhost:5173');
  console.log('2. Check that all main nav links are visible: Devices, Export & Import, Plugins, Metrics, Admin');
  console.log('3. Hover over "Export & Import" dropdown and verify all menu items appear');
  console.log('4. Click each nav link and verify pages load correctly');
  console.log('5. Check that breadcrumbs appear on non-home pages');
  console.log('6. Verify active states highlight correctly');
  console.log('7. Test responsive behavior on different screen sizes');
  console.log('\nRoutes to test:');
  testRoutes.forEach(route => {
    console.log(`   - ${route.path} (${route.name})`);
  });
}
