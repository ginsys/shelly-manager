import { expect, test, type Page } from '@playwright/test'

// The Provisioning Agents page with the API mocked, so no device is contacted and nothing is written.
// It holds a pending, an approved and a keyless (legacy) agent, as the manager lists them, and a mock of
// the approve route that moves the pending agent to approved.

const now = new Date().toISOString()
const envelope = (data: unknown) => ({ success: true, data, timestamp: now })

const approved = {
  id: 'keyed-3f9a1c2b', name: 'garage-gateway', status: 'online', version: '0.9.2',
  capabilities: ['wifi', 'restart'], lastSeen: now, enrolmentState: 'approved',
  fingerprint: '3f9a-1c2b-7d8e-4f60-a1b2-c3d4-e5f6-0718',
}
const legacy = {
  id: 'office-pi', name: 'office-pi', status: 'online', version: '0.8.0',
  capabilities: ['wifi'], lastSeen: now, enrolmentState: 'legacy',
}
const pending = {
  id: 'keyed-9c01d7e2', name: 'workshop-pi', status: 'pending', version: '0.9.2',
  capabilities: ['wifi'], lastSeen: now, enrolmentState: 'pending',
  fingerprint: '9c01-d7e2-b4a6-5f38-c0d1-e2f3-a4b5-c6d7',
}

async function mockAgents(page: Page) {
  const agents = [approved, legacy, { ...pending }]
  await page.route('**/api/v1/provisioning/agents', route => route.fulfill({
    contentType: 'application/json',
    body: JSON.stringify(envelope({ agents, count: agents.length })),
  }))
  await page.route('**/api/v1/provisioner/agents/*/approve', route => {
    const id = decodeURIComponent(new URL(route.request().url()).pathname.split('/').at(-2) ?? '')
    const row = agents.find(a => a.id === id)
    if (row) Object.assign(row, { enrolmentState: 'approved', status: 'approved' })
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify(envelope({ id })) })
  })
}

test.describe('mocked provisioning agents page', () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await mockAgents(page)
  })

  test('shows the state and fingerprint of each agent, and the pending one first', async ({ page }) => {
    await page.goto('/provisioning/agents')
    await expect(page.getByRole('heading', { level: 1, name: 'Provisioning Agents' })).toBeVisible()
    const pendingSection = page.getByTestId('pending-agents')
    await expect(pendingSection).toContainText('workshop-pi')
    await expect(pendingSection).toContainText(pending.fingerprint)
    await expect(page.getByText(approved.fingerprint)).toBeVisible()
    await page.screenshot({
      path: '../docs/frontend/images/provisioning-agents-enrolment.png',
      fullPage: true,
    })
  })

  test('approving the pending agent moves it out of the waiting list', async ({ page }) => {
    await page.goto('/provisioning/agents')
    await page.getByTestId(`approve-${pending.id}`).click()
    await expect(page.getByTestId('pending-agents')).toHaveCount(0)
    // The mock stores `approved` in both fields, as the manager does; the page says it once, in State.
    const cells = page.getByRole('row', { name: /workshop-pi/ }).getByRole('cell')
    await expect(cells.nth(1)).toHaveText('approved')
    await expect(cells.nth(3)).toHaveText('—')
  })
})
