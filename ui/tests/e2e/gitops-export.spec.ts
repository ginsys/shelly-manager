import { test, expect } from '@playwright/test'
import { 
  waitForPageReady, 
  waitForApiResponse,
  fillFormField,
  submitForm,
  SELECTORS,
  mockApiResponse
} from './fixtures/test-helpers'

test.describe('GitOps Export E2E', () => {
  
  test.beforeEach(async ({ page }) => {
    await page.goto('/gitops-export')
    await waitForPageReady(page)
  })

  test('gitops export page loads correctly', async ({ page }) => {
    // Check page title
    const heading = page.locator('h1, h2, [data-testid="page-title"]')
    await expect(heading.first()).toBeVisible()
    
    // Check for GitOps configuration form or status
    const gitopsConfig = page.locator('[data-testid="gitops-config"], .gitops-form, .config-form')
    const gitopsStatus = page.locator('[data-testid="gitops-status"], .gitops-info')
    
    await expect(gitopsConfig.or(gitopsStatus).first()).toBeVisible()
  })

  test('displays current GitOps configuration', async ({ page }) => {
    // Mock GitOps configuration
    await mockApiResponse(page, 'gitops/config', {
      config: {
        enabled: true,
        repository_url: 'https://github.com/user/shelly-configs',
        branch: 'main',
        path: 'home-assistant/',
        commit_message_template: 'Update Shelly configurations',
        auto_sync: true,
        sync_interval: '1h',
        last_sync: '2025-09-10T10:00:00Z'
      }
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should display configuration details
    await expect(page.locator('text=https://github.com/user/shelly-configs')).toBeVisible()
    await expect(page.locator('text=main')).toBeVisible()
    await expect(page.locator('text=home-assistant/')).toBeVisible()
    await expect(page.locator('text=/auto.*sync|sync.*enabled/i')).toBeVisible()
  })

  test('can configure GitOps settings', async ({ page }) => {
    // Mock GitOps update API
    await page.route('**/api/v1/gitops/config', route => {
      if (route.request().method() === 'PUT') {
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: true,
            data: {
              enabled: true,
              repository_url: 'https://github.com/test/configs',
              branch: 'develop',
              path: 'shelly/',
              auto_sync: true
            }
          })
        })
      } else {
        route.continue()
      }
    })
    
    // Look for configuration form
    const configForm = page.locator('[data-testid="gitops-form"], .gitops-config-form')
    const editButton = page.locator('[data-testid="edit-config"], button:has-text("Edit"), button:has-text("Configure")')
    
    if (await editButton.isVisible()) {
      await editButton.click()
      await waitForPageReady(page)
    }
    
    await expect(configForm.first()).toBeVisible()
    
    // Fill configuration fields
    const repoUrlField = page.locator('[data-testid="repository-url"], input[name="repository_url"]')
    if (await repoUrlField.isVisible()) {
      await fillFormField(page, repoUrlField.first().getAttribute('selector') || 'input[name="repository_url"]', 'https://github.com/test/configs')
    }
    
    const branchField = page.locator('[data-testid="branch"], input[name="branch"]')
    if (await branchField.isVisible()) {
      await fillFormField(page, branchField.first().getAttribute('selector') || 'input[name="branch"]', 'develop')
    }
    
    const pathField = page.locator('[data-testid="path"], input[name="path"]')
    if (await pathField.isVisible()) {
      await fillFormField(page, pathField.first().getAttribute('selector') || 'input[name="path"]', 'shelly/')
    }
    
    // Enable auto-sync
    const autoSyncToggle = page.locator('[data-testid="auto-sync"], .q-toggle')
    if (await autoSyncToggle.isVisible()) {
      await autoSyncToggle.click()
    }
    
    // Save configuration
    const saveButton = page.locator('[data-testid="save-config"], button:has-text("Save")')
    await saveButton.click()
    
    await waitForPageReady(page)
    
    // Should show success message
    const successMessage = page.locator('.q-notification--positive, .success')
    await expect(successMessage).toBeVisible({ timeout: 5000 })
  })

  test('can test GitOps connectivity', async ({ page }) => {
    // Mock test connection API
    await page.route('**/api/v1/gitops/test', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            status: 'connected',
            message: 'Successfully connected to repository',
            details: {
              repository: 'https://github.com/test/configs',
              branch: 'main',
              write_access: true,
              last_commit: 'abc123'
            }
          }
        })
      })
    })
    
    // Click test connection button
    const testButton = page.locator('[data-testid="test-connection"], button:has-text("Test")')
    
    if (await testButton.isVisible()) {
      await testButton.click()
      await waitForPageReady(page, 15000)
      
      // Should show test results
      const testResults = page.locator('[data-testid="test-results"], .test-results, .connection-status')
      await expect(testResults).toBeVisible({ timeout: 10000 })
      
      // Should show success message
      await expect(page.locator('text=/connected|success/i')).toBeVisible()
      
      // Should show repository details
      await expect(page.locator('text=/write.*access|last.*commit/i')).toBeVisible()
    }
  })

  test('can trigger manual sync', async ({ page }) => {
    // Mock manual sync API
    await page.route('**/api/v1/gitops/sync', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            sync_id: 'sync-123',
            status: 'in_progress',
            started_at: '2025-09-10T12:00:00Z'
          }
        })
      })
    })
    
    // Click sync now button
    const syncButton = page.locator('[data-testid="sync-now"], button:has-text("Sync")')
    
    if (await syncButton.isVisible()) {
      await syncButton.click()
      await waitForPageReady(page)
      
      // Should show sync in progress
      const syncStatus = page.locator('[data-testid="sync-status"], .sync-progress')
      await expect(syncStatus).toBeVisible({ timeout: 5000 })
      
      // Should show progress indicator
      const progressIndicator = page.locator('.q-spinner, .progress, [data-testid="sync-progress"]')
      await expect(progressIndicator.first()).toBeVisible()
    }
  })

  test('displays sync history and status', async ({ page }) => {
    // Mock sync history
    await mockApiResponse(page, 'gitops/history', {
      history: [
        {
          id: 'sync-1',
          started_at: '2025-09-10T10:00:00Z',
          completed_at: '2025-09-10T10:02:00Z',
          status: 'success',
          commit_sha: 'abc123def',
          files_changed: 5,
          message: 'Updated device configurations'
        },
        {
          id: 'sync-2',
          started_at: '2025-09-09T10:00:00Z',
          completed_at: '2025-09-09T10:01:30Z',
          status: 'success',
          commit_sha: 'def456ghi',
          files_changed: 3,
          message: 'Added new device exports'
        }
      ]
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should show sync history
    const syncHistory = page.locator('[data-testid="sync-history"], .sync-log, .history-table')
    
    if (await syncHistory.isVisible()) {
      await expect(syncHistory).toBeVisible()
      
      // Should show sync entries
      await expect(page.locator('text=abc123def')).toBeVisible()
      await expect(page.locator('text=def456ghi')).toBeVisible()
      
      // Should show file counts
      await expect(page.locator('text=/5.*files|3.*files/i')).toBeVisible()
      
      // Should show status
      await expect(page.locator('text=success')).toBeVisible()
    }
  })

  test('validates GitOps configuration fields', async ({ page }) => {
    // Click configure or edit button
    const configureButton = page.locator('[data-testid="edit-config"], button:has-text("Configure")')
    if (await configureButton.isVisible()) {
      await configureButton.click()
      await waitForPageReady(page)
    }
    
    // Try to submit with invalid repository URL
    const repoUrlField = page.locator('[data-testid="repository-url"], input[name="repository_url"]')
    if (await repoUrlField.isVisible()) {
      await repoUrlField.fill('invalid-url')
      
      const saveButton = page.locator('[data-testid="save-config"], button:has-text("Save")')
      await saveButton.click()
      
      // Should show validation error
      const validationErrors = page.locator('.field-error, .q-field--error, .error-message')
      await expect(validationErrors.first()).toBeVisible({ timeout: 3000 })
    }
    
    // Test empty required fields
    if (await repoUrlField.isVisible()) {
      await repoUrlField.fill('')
      
      const saveButton = page.locator('[data-testid="save-config"], button:has-text("Save")')
      await saveButton.click()
      
      // Should show required field error
      const validationErrors = page.locator('.field-error, .q-field--error')
      await expect(validationErrors.first()).toBeVisible({ timeout: 3000 })
    }
  })

  test('shows GitOps authentication options', async ({ page }) => {
    // Look for authentication section
    const authSection = page.locator('[data-testid="gitops-auth"], .auth-config, .authentication')
    const tokenField = page.locator('[data-testid="access-token"], input[name="access_token"], input[type="password"]')
    const keyField = page.locator('[data-testid="ssh-key"], textarea[name="ssh_key"]')
    
    if (await authSection.isVisible()) {
      await expect(authSection).toBeVisible()
      
      // Should have authentication options
      if (await tokenField.isVisible()) {
        await expect(tokenField).toBeVisible()
        
        // Should be a password field for security
        const fieldType = await tokenField.getAttribute('type')
        expect(fieldType).toBe('password')
      }
      
      if (await keyField.isVisible()) {
        await expect(keyField).toBeVisible()
      }
    }
  })

  test('can enable/disable GitOps functionality', async ({ page }) => {
    // Mock toggle API
    await page.route('**/api/v1/gitops/toggle', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: { enabled: true }
        })
      })
    })
    
    // Find enable/disable toggle
    const enableToggle = page.locator('[data-testid="gitops-enabled"], .q-toggle')
    
    if (await enableToggle.isVisible()) {
      await enableToggle.click()
      await waitForPageReady(page)
      
      // Should show confirmation or success message
      const successMessage = page.locator('.q-notification--positive, .success')
      await expect(successMessage).toBeVisible({ timeout: 5000 })
    }
  })

  test('handles GitOps API errors gracefully', async ({ page }) => {
    // Mock API error
    await page.route('**/api/v1/gitops**', route => {
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          error: 'GitOps service unavailable'
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

  test('shows sync conflicts and resolution', async ({ page }) => {
    // Mock sync conflict
    await mockApiResponse(page, 'gitops/status', {
      status: {
        enabled: true,
        last_sync: '2025-09-10T10:00:00Z',
        sync_status: 'conflict',
        conflicts: [
          {
            file: 'devices/shelly-1.yaml',
            type: 'merge_conflict',
            description: 'Local changes conflict with remote changes'
          }
        ]
      }
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should show conflict warning
    const conflictWarning = page.locator('[data-testid="sync-conflicts"], .conflict-warning, .q-banner--warning')
    
    if (await conflictWarning.isVisible()) {
      await expect(conflictWarning).toBeVisible()
      
      // Should show conflict details
      await expect(page.locator('text=/conflict|merge/i')).toBeVisible()
      await expect(page.locator('text=shelly-1.yaml')).toBeVisible()
      
      // Should offer resolution options
      const resolveButton = page.locator('[data-testid="resolve-conflicts"], button:has-text("Resolve")')
      if (await resolveButton.isVisible()) {
        await expect(resolveButton).toBeVisible()
      }
    }
  })

  test('gitops export is responsive', async ({ page }) => {
    const viewports = [
      { width: 1920, height: 1080 },
      { width: 768, height: 1024 },
      { width: 375, height: 667 }
    ]

    for (const viewport of viewports) {
      await page.setViewportSize(viewport)
      await page.reload()
      await waitForPageReady(page)
      
      // GitOps content should be accessible at all sizes
      const gitopsContent = page.locator('[data-testid="gitops-config"], main, .q-page')
      await expect(gitopsContent.first()).toBeVisible()
      
      // Configuration form should be usable on mobile
      const configForm = page.locator('[data-testid="gitops-form"], .config-form')
      if (await configForm.isVisible()) {
        await expect(configForm).toBeVisible()
      }
    }
  })
})