const { chromium } = require('playwright');

const viewports = [
  { name: 'Mobile', width: 375, height: 667 },
  { name: 'Tablet', width: 768, height: 1024 },
  { name: 'Desktop', width: 1200, height: 800 }
];

const routes = [
  '/',
  '/export/schedules',
  '/export/backup',
  '/plugins'
];

async function testResponsive() {
  const browser = await chromium.launch({ headless: true });
  
  console.log('🔍 Testing Responsive Design');
  console.log('=============================');

  for (const viewport of viewports) {
    console.log(`\n📱 Testing ${viewport.name} (${viewport.width}x${viewport.height})`);
    
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();

    for (const route of routes) {
      try {
        console.log(`  📄 Route: ${route}`);
        
        await page.goto(`http://localhost:5174${route}`, {
          waitUntil: 'networkidle',
          timeout: 10000
        });

        // Wait for main content
        await page.waitForSelector('main', { timeout: 5000 });

        // Check navigation visibility
        const nav = await page.locator('nav.nav').isVisible();
        const brand = await page.locator('.brand').isVisible();
        
        console.log(`    Navigation visible: ${nav ? '✅' : '❌'}`);
        console.log(`    Brand visible: ${brand ? '✅' : '❌'}`);

        // Check if content overflows
        const body = await page.locator('body').boundingBox();
        const hasHorizontalScroll = body.width > viewport.width;
        
        if (hasHorizontalScroll) {
          console.log(`    ⚠️ Potential horizontal overflow detected`);
        } else {
          console.log(`    ✅ No horizontal overflow`);
        }

        // Take screenshot
        const filename = `test-results/responsive-${viewport.name.toLowerCase()}-${route.replace(/\//g, '-')}.png`;
        await page.screenshot({ path: filename, fullPage: false });
        
      } catch (error) {
        console.log(`    ❌ Error: ${error.message}`);
      }
    }
    
    await context.close();
  }

  await browser.close();
  console.log('\n✅ Responsive testing completed');
}

if (require.main === module) {
  testResponsive().catch(error => {
    console.error('❌ Responsive test failed:', error);
    process.exit(1);
  });
}