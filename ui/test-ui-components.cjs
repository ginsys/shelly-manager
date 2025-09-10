const { chromium } = require('playwright');

async function testUIComponents() {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1200, height: 800 } });
  const page = await context.newPage();

  const results = {
    navigation: {},
    forms: {},
    interactions: {}
  };

  console.log('🔍 Testing UI Component Functionality');
  console.log('=====================================');

  try {
    // Test navigation dropdown
    console.log('\n📋 Testing Navigation Components');
    await page.goto('http://localhost:5174/', { waitUntil: 'networkidle' });
    
    // Test dropdown functionality
    const dropdown = page.locator('.nav-dropdown');
    await dropdown.hover();
    await page.waitForTimeout(500); // Wait for dropdown animation
    
    const dropdownVisible = await page.locator('.dropdown-menu').isVisible();
    results.navigation.dropdown = dropdownVisible;
    console.log(`  Dropdown menu on hover: ${dropdownVisible ? '✅' : '❌'}`);

    // Test dropdown links
    const dropdownLinks = await page.locator('.dropdown-item').count();
    results.navigation.dropdownLinks = dropdownLinks;
    console.log(`  Dropdown links count: ${dropdownLinks} items`);

    // Test main navigation links
    const navLinks = await page.locator('.nav-link').count();
    results.navigation.mainLinks = navLinks;
    console.log(`  Main navigation links: ${navLinks} items`);

    // Test breadcrumb on a sub-page
    console.log('\n🍞 Testing Breadcrumb Navigation');
    await page.goto('http://localhost:5174/export/schedules', { waitUntil: 'networkidle' });
    
    const breadcrumbVisible = await page.locator('.breadcrumb').isVisible();
    const breadcrumbItems = await page.locator('.breadcrumb-item').count();
    results.navigation.breadcrumb = { visible: breadcrumbVisible, items: breadcrumbItems };
    console.log(`  Breadcrumb visible: ${breadcrumbVisible ? '✅' : '❌'}`);
    console.log(`  Breadcrumb items: ${breadcrumbItems} items`);

    // Test forms on different pages
    console.log('\n📝 Testing Form Components');
    
    // Export Schedules form
    const scheduleForm = await page.locator('form, .form-section').count();
    results.forms.schedules = scheduleForm > 0;
    console.log(`  Schedule forms present: ${scheduleForm > 0 ? '✅' : '❌'}`);

    // Test Plugin Management page
    await page.goto('http://localhost:5174/plugins', { waitUntil: 'networkidle' });
    
    const pluginStats = await page.locator('.stats .card').count();
    const pluginFilters = await page.locator('.filter-group').count();
    results.forms.plugins = { stats: pluginStats, filters: pluginFilters };
    console.log(`  Plugin stats cards: ${pluginStats} cards`);
    console.log(`  Plugin filters: ${pluginFilters} filter groups`);

    // Test interactive elements
    console.log('\n⚡ Testing Interactive Elements');
    
    // Test button hover states
    const buttons = await page.locator('button, .btn').count();
    results.interactions.buttons = buttons;
    console.log(`  Interactive buttons found: ${buttons} buttons`);

    // Test search functionality if present
    const searchInputs = await page.locator('input[type="search"], input[placeholder*="search" i]').count();
    results.interactions.search = searchInputs;
    console.log(`  Search inputs found: ${searchInputs} inputs`);

    // Test if modals/dialogs exist
    const modals = await page.locator('.modal, .dialog, .popup').count();
    results.interactions.modals = modals;
    console.log(`  Modal/dialog elements: ${modals} elements`);

    console.log('\n📊 Component Test Summary');
    console.log('========================');
    console.log('Navigation:');
    console.log(`  - Dropdown functionality: ${results.navigation.dropdown ? '✅' : '❌'}`);
    console.log(`  - Main links: ${results.navigation.mainLinks}`);
    console.log(`  - Dropdown links: ${results.navigation.dropdownLinks}`);
    console.log(`  - Breadcrumb: ${results.navigation.breadcrumb?.visible ? '✅' : '❌'}`);
    
    console.log('Forms & Content:');
    console.log(`  - Schedule forms: ${results.forms.schedules ? '✅' : '❌'}`);
    console.log(`  - Plugin stats: ${results.forms.plugins?.stats} cards`);
    console.log(`  - Plugin filters: ${results.forms.plugins?.filters} groups`);
    
    console.log('Interactions:');
    console.log(`  - Buttons: ${results.interactions.buttons} found`);
    console.log(`  - Search inputs: ${results.interactions.search} found`);
    console.log(`  - Modals: ${results.interactions.modals} found`);

  } catch (error) {
    console.error('❌ Component test error:', error.message);
  }

  await browser.close();
  return results;
}

if (require.main === module) {
  testUIComponents().catch(error => {
    console.error('❌ Component test failed:', error);
    process.exit(1);
  });
}