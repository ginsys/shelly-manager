const { chromium } = require('playwright');

async function debugBrandVisibility() {
  const browser = await chromium.launch({ headless: false }); // Set to false to see what's happening
  const context = await browser.newContext({ viewport: { width: 1200, height: 800 } });
  const page = await context.newPage();

  try {
    console.log('🔍 Debugging Brand Visibility');
    
    await page.goto('http://localhost:5174/', {
      waitUntil: 'networkidle',
      timeout: 10000
    });

    // Wait for the page to load
    await page.waitForSelector('header.topbar', { timeout: 5000 });

    // Check if brand element exists
    const brandExists = await page.locator('.brand').count() > 0;
    console.log(`Brand element exists: ${brandExists ? '✅' : '❌'}`);

    if (brandExists) {
      // Get brand element properties
      const brandElement = page.locator('.brand');
      const isVisible = await brandElement.isVisible();
      const boundingBox = await brandElement.boundingBox();
      const styles = await brandElement.evaluate(el => {
        const computed = window.getComputedStyle(el);
        return {
          display: computed.display,
          visibility: computed.visibility,
          opacity: computed.opacity,
          width: computed.width,
          height: computed.height,
          position: computed.position,
          zIndex: computed.zIndex,
          fontSize: computed.fontSize,
          color: computed.color,
          backgroundColor: computed.backgroundColor
        };
      });

      console.log('Brand Properties:');
      console.log('  isVisible():', isVisible);
      console.log('  boundingBox:', boundingBox);
      console.log('  styles:', JSON.stringify(styles, null, 2));

      // Get the actual text content
      const textContent = await brandElement.textContent();
      console.log('  textContent:', textContent);

      // Check if it's being covered by something else
      const elementAtPoint = boundingBox ? await page.evaluate(({ x, y }) => {
        const element = document.elementFromPoint(x + 10, y + 10);
        return {
          tagName: element?.tagName,
          className: element?.className,
          id: element?.id
        };
      }, { x: boundingBox.x, y: boundingBox.y }) : null;
      
      if (elementAtPoint) {
        console.log('  elementAtPoint:', elementAtPoint);
      }
    }

    // Check topbar structure
    const topbarHTML = await page.locator('header.topbar').innerHTML();
    console.log('\nTopbar HTML structure:');
    console.log(topbarHTML.substring(0, 500) + '...');

    // Take a screenshot for visual reference
    await page.screenshot({ 
      path: 'test-results/brand-debug.png',
      clip: { x: 0, y: 0, width: 1200, height: 200 }
    });

  } catch (error) {
    console.error('Error during debugging:', error);
  }

  await browser.close();
}

if (require.main === module) {
  debugBrandVisibility().catch(error => {
    console.error('❌ Debug script failed:', error);
    process.exit(1);
  });
}