import { test, expect } from '@playwright/test'
import { 
  waitForPageReady, 
  waitForApiResponse,
  fillFormField,
  submitForm,
  SELECTORS,
  mockApiResponse
} from './fixtures/test-helpers'

test.describe('Backup Management E2E', () => {
  
  test.beforeEach(async ({ page }) => {
    await page.goto('/export/backup')
    await waitForPageReady(page)
  })

  test('backup management page loads correctly', async ({ page }) => {
    // Check page title
    const heading = page.locator('h1, h2, [data-testid="page-title"]')
    await expect(heading.first()).toBeVisible()
    
    // Check for backup list or empty state
    const backupList = page.locator('[data-testid="backup-list"], .q-table, .backup-grid')
    const emptyState = page.locator('[data-testid="empty-state"], .no-backups')
    
    await expect(backupList.or(emptyState).first()).toBeVisible()
  })

  test('displays existing backups', async ({ page }) => {
    // Mock backup data
    await mockApiResponse(page, 'backups', {
      backups: [
        {
          id: 'backup-1',
          name: 'Full System Backup',
          type: 'full',
          size: '2.5MB',
          created_at: '2025-09-10T10:00:00Z',
          devices_count: 15,
          status: 'completed'
        },
        {
          id: 'backup-2',
          name: 'Configuration Only',
          type: 'config',
          size: '512KB',
          created_at: '2025-09-09T10:00:00Z',
          devices_count: 15,
          status: 'completed'
        }
      ]
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should display backup items
    const backupItems = page.locator('[data-testid="backup-item"], .backup-row')
    await expect(backupItems.first()).toBeVisible()
    
    // Check backup details are shown
    await expect(page.locator('text=Full System Backup')).toBeVisible()
    await expect(page.locator('text=Configuration Only')).toBeVisible()
    await expect(page.locator('text=2.5MB')).toBeVisible()
    await expect(page.locator('text=512KB')).toBeVisible()
  })

  test('can create new backup', async ({ page }) => {
    // Mock backup creation API
    await page.route('**/api/v1/backups', route => {
      if (route.request().method() === 'POST') {
        route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            success: true,
            data: {
              id: 'new-backup',
              name: 'Test Backup',
              type: 'full',
              status: 'in_progress'
            }
          })
        })
      } else {
        route.continue()
      }
    })
    
    // Click create backup button
    const createButton = page.locator('[data-testid="create-backup"], button:has-text("Create"), .q-btn:has-text("New")')
    await expect(createButton.first()).toBeVisible()
    await createButton.first().click()
    
    await waitForPageReady(page)
    
    // Should show backup form
    const backupForm = page.locator('[data-testid="backup-form"], .backup-form, .q-dialog')
    await expect(backupForm).toBeVisible()
    
    // Fill form fields
    const nameField = page.locator('[data-testid="backup-name"], input[name="name"]')
    if (await nameField.isVisible()) {
      await fillFormField(page, nameField.first().getAttribute('selector') || 'input[name="name"]', 'Test Backup')
    }
    
    // Select backup type
    const typeSelect = page.locator('[data-testid="backup-type"], .q-select, select[name="type"]')
    if (await typeSelect.isVisible()) {
      if (await typeSelect.locator('.q-select').isVisible()) {
        await typeSelect.click()
        await page.locator('.q-item:has-text("Full")').click()
      } else {
        await typeSelect.selectOption('full')
      }
    }
    
    // Submit form
    const saveButton = page.locator('[data-testid="create-backup-submit"], button:has-text("Create")')
    await saveButton.click()
    
    await waitForPageReady(page)
    
    // Should show success message and backup in progress
    const successMessage = page.locator('.q-notification--positive, .success')
    await expect(successMessage).toBeVisible({ timeout: 5000 })
  })

  test('shows backup creation progress', async ({ page }) => {
    // Mock backup in progress
    await mockApiResponse(page, 'backups', {
      backups: [
        {
          id: 'backup-1',
          name: 'In Progress Backup',
          type: 'full',
          status: 'in_progress',
          progress: 65,
          created_at: '2025-09-10T10:00:00Z'
        }
      ]
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should show progress indicator
    const progressBar = page.locator('[data-testid="backup-progress"], .q-linear-progress, .progress-bar')
    await expect(progressBar.first()).toBeVisible()
    
    // Should show progress percentage
    await expect(page.locator('text=/65%|progress/i')).toBeVisible()
    
    // Should show status
    await expect(page.locator('text=/in progress|creating/i')).toBeVisible()
  })

  test('can download backups', async ({ page }) => {
    // Mock backup data
    await mockApiResponse(page, 'backups', {
      backups: [
        {
          id: 'backup-1',
          name: 'Completed Backup',
          type: 'full',
          size: '2.5MB',
          status: 'completed',
          download_url: '/api/v1/backups/backup-1/download'
        }
      ]
    })
    
    // Mock download endpoint
    await page.route('**/api/v1/backups/backup-1/download', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/zip',
        body: 'mock-backup-content'
      })
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Wait for and listen for download
    const downloadPromise = page.waitForEvent('download')
    
    // Click download button
    const downloadButton = page.locator('[data-testid="download-backup"], button:has-text("Download"), .q-btn[title="Download"]')
    await expect(downloadButton.first()).toBeVisible()
    await downloadButton.first().click()
    
    // Wait for download to start
    const download = await downloadPromise
    expect(download.suggestedFilename()).toContain('backup')
  })

  test('can delete backups', async ({ page }) => {
    // Mock backup data
    await mockApiResponse(page, 'backups', {
      backups: [
        {
          id: 'backup-1',
          name: 'Old Backup',
          type: 'full',
          size: '2.5MB',
          status: 'completed'
        }
      ]
    })
    
    // Mock delete API
    await page.route('**/api/v1/backups/backup-1', route => {
      if (route.request().method() === 'DELETE') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: true,
            message: 'Backup deleted successfully'
          })
        })
      } else {
        route.continue()
      }
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Click delete button
    const deleteButton = page.locator('[data-testid="delete-backup"], button:has-text("Delete"), .q-btn[title="Delete"]')
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

  test('can restore from backup', async ({ page }) => {
    // Mock backup data
    await mockApiResponse(page, 'backups', {
      backups: [
        {
          id: 'backup-1',
          name: 'Restore Point',
          type: 'full',
          size: '2.5MB',
          status: 'completed',
          restorable: true
        }
      ]
    })
    
    // Mock restore API
    await page.route('**/api/v1/backups/backup-1/restore', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            restore_id: 'restore-1',
            status: 'in_progress'
          }
        })
      })
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Click restore button
    const restoreButton = page.locator('[data-testid="restore-backup"], button:has-text("Restore")')
    await expect(restoreButton.first()).toBeVisible()
    await restoreButton.first().click()
    
    await waitForPageReady(page)
    
    // Should show confirmation dialog
    const confirmDialog = page.locator('[data-testid="confirm-dialog"], .q-dialog')
    await expect(confirmDialog).toBeVisible()
    
    // Should warn about overwriting current configuration
    await expect(page.locator('text=/overwrite|replace|current/i')).toBeVisible()
    
    // Confirm restore
    const confirmButton = page.locator('[data-testid="confirm-restore"], button:has-text("Restore")')
    await confirmButton.click()
    
    await waitForPageReady(page)
    
    // Should show restore in progress
    const successMessage = page.locator('.q-notification--positive, .success')
    await expect(successMessage).toBeVisible({ timeout: 5000 })
  })

  test('displays backup statistics', async ({ page }) => {
    // Mock backup statistics
    await mockApiResponse(page, 'backups/stats', {
      stats: {
        total_backups: 12,
        total_size: '25.4MB',
        oldest_backup: '2025-08-01T10:00:00Z',
        newest_backup: '2025-09-10T10:00:00Z',
        backup_types: {
          full: 8,
          config: 4
        }
      }
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should show statistics
    const statsSection = page.locator('[data-testid="backup-stats"], .backup-statistics')
    
    if (await statsSection.isVisible()) {
      await expect(statsSection).toBeVisible()
      
      // Check for key statistics
      await expect(page.locator('text=/12.*backups|total.*12/i')).toBeVisible()
      await expect(page.locator('text=/25\.4MB|total.*size/i')).toBeVisible()
    }
  })

  test('can filter backups by type', async ({ page }) => {
    // Mock backup data with different types
    await mockApiResponse(page, 'backups', {
      backups: [
        {
          id: 'backup-1',
          name: 'Full Backup',
          type: 'full',
          status: 'completed'
        },
        {
          id: 'backup-2',
          name: 'Config Backup',
          type: 'config',
          status: 'completed'
        }
      ]
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Look for filter controls
    const filterButton = page.locator('[data-testid="filter-backups"], .filter-button, button:has-text("Filter")')
    const typeFilter = page.locator('[data-testid="backup-type-filter"], select[name="type_filter"]')
    
    if (await filterButton.isVisible()) {
      await filterButton.click()
      await waitForPageReady(page)
      
      // Should show filter options
      const filterOptions = page.locator('[data-testid="filter-options"], .filter-menu')
      await expect(filterOptions).toBeVisible()
    }
    
    if (await typeFilter.isVisible()) {
      await typeFilter.selectOption('full')
      await waitForPageReady(page)
      
      // Results should be filtered
      await expect(page.locator('text=Full Backup')).toBeVisible()
      // Config backup should be hidden or filtered out
    }
  })

  test('handles backup API errors gracefully', async ({ page }) => {
    // Mock API error
    await page.route('**/api/v1/backups**', route => {
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          error: 'Backup service unavailable'
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

  test('backup management is responsive', async ({ page }) => {
    const viewports = [
      { width: 1920, height: 1080 },
      { width: 768, height: 1024 },
      { width: 375, height: 667 }
    ]

    for (const viewport of viewports) {
      await page.setViewportSize(viewport)
      await page.reload()
      await waitForPageReady(page)
      
      // Backup content should be accessible at all sizes
      const backupContent = page.locator('[data-testid="backup-list"], main, .q-page')
      await expect(backupContent.first()).toBeVisible()
      
      // Create button should be accessible
      const createButton = page.locator('[data-testid="create-backup"], button:has-text("Create")')
      await expect(createButton.first()).toBeVisible()
    }
  })
})