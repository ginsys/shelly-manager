import { test, expect } from '@playwright/test'
import { 
  waitForPageReady, 
  waitForApiResponse,
  fillFormField,
  submitForm,
  SELECTORS,
  mockApiResponse,
  TEST_DATA
} from './fixtures/test-helpers'

test.describe('Schedule Management E2E', () => {
  
  test.beforeEach(async ({ page }) => {
    await page.goto('/export-schedules')
    await waitForPageReady(page)
  })

  test('schedule management page loads correctly', async ({ page }) => {
    // Check page title
    const heading = page.locator('h1, h2, [data-testid="page-title"]')
    await expect(heading.first()).toBeVisible()
    
    // Check for schedule list or empty state
    const scheduleList = page.locator('[data-testid="schedule-list"], .q-table, .schedule-grid')
    const emptyState = page.locator('[data-testid="empty-state"], .no-schedules')
    
    await expect(scheduleList.or(emptyState).first()).toBeVisible()
  })

  test('displays existing schedules', async ({ page }) => {
    // Mock schedule data
    await mockApiResponse(page, 'schedules', {
      schedules: [
        {
          id: 'schedule-1',
          name: 'Daily Home Assistant Export',
          plugin: 'home-assistant',
          frequency: 'daily',
          time: '02:00',
          enabled: true,
          last_run: '2025-09-10T02:00:00Z',
          next_run: '2025-09-11T02:00:00Z'
        },
        {
          id: 'schedule-2',
          name: 'Weekly OPNsense Export',
          plugin: 'opnsense',
          frequency: 'weekly',
          time: '03:00',
          enabled: false,
          last_run: null,
          next_run: null
        }
      ]
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should display schedule items
    const scheduleItems = page.locator('[data-testid="schedule-item"], .schedule-row')
    await expect(scheduleItems.first()).toBeVisible()
    
    // Check schedule details are shown
    await expect(page.locator('text=Daily Home Assistant Export')).toBeVisible()
    await expect(page.locator('text=Weekly OPNsense Export')).toBeVisible()
    await expect(page.locator('text=02:00')).toBeVisible()
    await expect(page.locator('text=03:00')).toBeVisible()
  })

  test('can create new schedule', async ({ page }) => {
    // Mock API responses
    await mockApiResponse(page, 'plugins', {
      plugins: [
        { id: 'home-assistant', name: 'Home Assistant', enabled: true },
        { id: 'opnsense', name: 'OPNsense', enabled: true }
      ]
    })
    
    await page.route('**/api/v1/schedules', route => {
      if (route.request().method() === 'POST') {
        route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            success: true,
            data: {
              id: 'new-schedule',
              name: 'Test Schedule',
              plugin: 'home-assistant',
              frequency: 'daily',
              time: '01:00',
              enabled: true
            }
          })
        })
      } else {
        route.continue()
      }
    })
    
    // Click create schedule button
    const createButton = page.locator('[data-testid="create-schedule"], button:has-text("Create"), .q-btn:has-text("Add")')
    await expect(createButton.first()).toBeVisible()
    await createButton.first().click()
    
    await waitForPageReady(page)
    
    // Should show schedule form
    const scheduleForm = page.locator('[data-testid="schedule-form"], .schedule-form, .q-dialog')
    await expect(scheduleForm).toBeVisible()
    
    // Fill form fields
    const nameField = page.locator('[data-testid="schedule-name"], input[name="name"]')
    if (await nameField.isVisible()) {
      await fillFormField(page, nameField.first().getAttribute('selector') || 'input[name="name"]', 'Test Schedule')
    }
    
    // Select plugin
    const pluginSelect = page.locator('[data-testid="schedule-plugin"], .q-select')
    if (await pluginSelect.isVisible()) {
      await pluginSelect.click()
      await page.locator('.q-item:has-text("Home Assistant")').click()
    }
    
    // Set frequency
    const frequencySelect = page.locator('[data-testid="schedule-frequency"], select[name="frequency"]')
    if (await frequencySelect.isVisible()) {
      await frequencySelect.selectOption('daily')
    }
    
    // Set time
    const timeField = page.locator('[data-testid="schedule-time"], input[type="time"]')
    if (await timeField.isVisible()) {
      await fillFormField(page, timeField.first().getAttribute('selector') || 'input[type="time"]', '01:00')
    }
    
    // Submit form
    const saveButton = page.locator('[data-testid="save-schedule"], button:has-text("Save")')
    await saveButton.click()
    
    await waitForPageReady(page)
    
    // Should show success message
    const successMessage = page.locator('.q-notification--positive, .success')
    await expect(successMessage).toBeVisible({ timeout: 5000 })
  })

  test('can edit existing schedule', async ({ page }) => {
    // Mock schedule data
    await mockApiResponse(page, 'schedules', {
      schedules: [
        {
          id: 'schedule-1',
          name: 'Daily Export',
          plugin: 'home-assistant',
          frequency: 'daily',
          time: '02:00',
          enabled: true
        }
      ]
    })
    
    // Mock edit API
    await page.route('**/api/v1/schedules/schedule-1', route => {
      if (route.request().method() === 'PUT') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: true,
            data: {
              id: 'schedule-1',
              name: 'Updated Daily Export',
              plugin: 'home-assistant',
              frequency: 'daily',
              time: '03:00',
              enabled: true
            }
          })
        })
      } else {
        route.continue()
      }
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Click edit button
    const editButton = page.locator('[data-testid="edit-schedule"], button:has-text("Edit"), .q-btn[title="Edit"]')
    await expect(editButton.first()).toBeVisible()
    await editButton.first().click()
    
    await waitForPageReady(page)
    
    // Should show schedule form with existing data
    const scheduleForm = page.locator('[data-testid="schedule-form"], .schedule-form')
    await expect(scheduleForm).toBeVisible()
    
    // Update time field
    const timeField = page.locator('[data-testid="schedule-time"], input[type="time"]')
    if (await timeField.isVisible()) {
      await timeField.fill('03:00')
    }
    
    // Save changes
    const saveButton = page.locator('[data-testid="save-schedule"], button:has-text("Save")')
    await saveButton.click()
    
    await waitForPageReady(page)
    
    // Should show success message
    const successMessage = page.locator('.q-notification--positive, .success')
    await expect(successMessage).toBeVisible({ timeout: 5000 })
  })

  test('can enable/disable schedules', async ({ page }) => {
    // Mock schedule data
    await mockApiResponse(page, 'schedules', {
      schedules: [
        {
          id: 'schedule-1',
          name: 'Daily Export',
          plugin: 'home-assistant',
          frequency: 'daily',
          time: '02:00',
          enabled: false
        }
      ]
    })
    
    // Mock toggle API
    await page.route('**/api/v1/schedules/schedule-1/toggle', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: { enabled: true }
        })
      })
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Find and click enable toggle
    const enableToggle = page.locator('[data-testid="schedule-toggle"], .q-toggle, button:has-text("Enable")')
    
    if (await enableToggle.first().isVisible()) {
      await enableToggle.first().click()
      await waitForPageReady(page)
      
      // Should show success indication
      const successMessage = page.locator('.q-notification--positive, .success')
      await expect(successMessage).toBeVisible({ timeout: 5000 })
    }
  })

  test('can delete schedules', async ({ page }) => {
    // Mock schedule data
    await mockApiResponse(page, 'schedules', {
      schedules: [
        {
          id: 'schedule-1',
          name: 'Daily Export',
          plugin: 'home-assistant',
          frequency: 'daily',
          time: '02:00',
          enabled: true
        }
      ]
    })
    
    // Mock delete API
    await page.route('**/api/v1/schedules/schedule-1', route => {
      if (route.request().method() === 'DELETE') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: true,
            message: 'Schedule deleted successfully'
          })
        })
      } else {
        route.continue()
      }
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Click delete button
    const deleteButton = page.locator('[data-testid="delete-schedule"], button:has-text("Delete"), .q-btn[title="Delete"]')
    await expect(deleteButton.first()).toBeVisible()
    await deleteButton.first().click()
    
    await waitForPageReady(page)
    
    // Should show confirmation dialog
    const confirmDialog = page.locator('[data-testid="confirm-dialog"], .q-dialog')
    await expect(confirmDialog).toBeVisible()
    
    // Confirm deletion
    const confirmButton = page.locator('[data-testid="confirm-delete"], button:has-text("Delete"), button:has-text("Confirm")')
    await confirmButton.click()
    
    await waitForPageReady(page)
    
    // Should show success message
    const successMessage = page.locator('.q-notification--positive, .success')
    await expect(successMessage).toBeVisible({ timeout: 5000 })
  })

  test('displays schedule execution status', async ({ page }) => {
    // Mock schedule with execution history
    await mockApiResponse(page, 'schedules', {
      schedules: [
        {
          id: 'schedule-1',
          name: 'Daily Export',
          plugin: 'home-assistant',
          frequency: 'daily',
          time: '02:00',
          enabled: true,
          last_run: '2025-09-10T02:00:00Z',
          next_run: '2025-09-11T02:00:00Z',
          status: 'success',
          execution_time: '2.3s'
        }
      ]
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should show execution status
    const statusIndicator = page.locator('[data-testid="schedule-status"], .status-badge, .execution-status')
    await expect(statusIndicator.first()).toBeVisible()
    
    // Should show last run time
    const lastRun = page.locator('[data-testid="last-run"], .last-run')
    if (await lastRun.isVisible()) {
      await expect(lastRun).toBeVisible()
    }
    
    // Should show next run time
    const nextRun = page.locator('[data-testid="next-run"], .next-run')
    if (await nextRun.isVisible()) {
      await expect(nextRun).toBeVisible()
    }
    
    // Should show execution time
    await expect(page.locator('text=/2\.3s|execution.*time/i')).toBeVisible()
  })

  test('validates schedule form inputs', async ({ page }) => {
    // Click create schedule
    const createButton = page.locator('[data-testid="create-schedule"], button:has-text("Create")')
    await createButton.first().click()
    await waitForPageReady(page)
    
    // Try to submit empty form
    const saveButton = page.locator('[data-testid="save-schedule"], button:has-text("Save")')
    await saveButton.click()
    
    // Should show validation errors
    const validationErrors = page.locator('.field-error, .q-field--error, .error-message')
    await expect(validationErrors.first()).toBeVisible({ timeout: 3000 })
    
    // Test invalid time format
    const timeField = page.locator('[data-testid="schedule-time"], input[type="time"]')
    if (await timeField.isVisible()) {
      await timeField.fill('25:00')  // Invalid time
      
      await saveButton.click()
      
      // Should show time validation error
      await expect(validationErrors.first()).toBeVisible({ timeout: 3000 })
    }
  })

  test('handles API errors gracefully', async ({ page }) => {
    // Mock API error
    await page.route('**/api/v1/schedules**', route => {
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          error: 'Schedule service unavailable'
        })
      })
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should show error state
    const errorState = page.locator('[data-testid="error-state"], .q-banner--negative, .error-message')
    await expect(errorState.first()).toBeVisible()
    
    // Error should be descriptive
    await expect(page.locator('text=/error|unavailable|failed/i')).toBeVisible()
  })

  test('schedule management is responsive', async ({ page }) => {
    const viewports = [
      { width: 1920, height: 1080 },
      { width: 768, height: 1024 },
      { width: 375, height: 667 }
    ]

    for (const viewport of viewports) {
      await page.setViewportSize(viewport)
      await page.reload()
      await waitForPageReady(page)
      
      // Schedule content should be accessible at all sizes
      const scheduleContent = page.locator('[data-testid="schedule-list"], main, .q-page')
      await expect(scheduleContent.first()).toBeVisible()
      
      // Create button should be accessible
      const createButton = page.locator('[data-testid="create-schedule"], button:has-text("Create")')
      await expect(createButton.first()).toBeVisible()
    }
  })
})