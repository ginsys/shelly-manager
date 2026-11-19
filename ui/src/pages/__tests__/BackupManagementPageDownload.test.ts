import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

// The client is mocked and the export module is not, so the real download helper runs: a backup that
// cannot be downloaded must show the server's own message (ginsys/shelly-manager#697).
vi.mock('@/api/client', () => ({
  default: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))

import api from '@/api/client'
import BackupManagementPage from '../BackupManagementPage.vue'

const backup = {
  id: 1,
  backup_id: 'b1',
  name: 'nightly',
  format: 'sma',
  device_count: 3,
  encrypted: false,
  success: true,
  created_at: '2026-11-19T10:00:00Z',
}

const realCreateElement = Document.prototype.createElement.bind(document)

function mockApi(download: () => Promise<unknown>) {
  vi.mocked(api.get as any).mockImplementation((path: string) => {
    if (path === '/export/backups') {
      return Promise.resolve({ data: { success: true, data: { backups: [backup] }, meta: {} } })
    }
    if (path === '/export/backup/b1/download') return download()
    return Promise.resolve({ data: { success: true, data: {} } })
  })
}

async function clickDownload() {
  const wrapper = mount(BackupManagementPage)
  await flushPromises()
  await wrapper.get('.download-btn').trigger('click')
  await flushPromises()
  return wrapper
}

describe('BackupManagementPage download', () => {
  beforeEach(() => {
    vi.mocked(api.get as any).mockReset()
    vi.mocked(document.createElement).mockImplementation(
      ((tag: string, opts?: ElementCreationOptions) => realCreateElement(tag, opts)) as any,
    )
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows the message of the server when the download is refused', async () => {
    mockApi(() =>
      Promise.reject(
        Object.assign(new Error('Request failed with status code 401'), {
          response: {
            status: 401,
            data: new Blob([
              JSON.stringify({ success: false, error: { code: 'UNAUTHORIZED', message: 'Admin authorization required' } }),
            ]),
          },
        }),
      ),
    )

    const wrapper = await clickDownload()

    const message = wrapper.get('.message.error')
    expect(message.text()).toContain('Admin authorization required')
    expect(message.text()).not.toContain('status code')
    wrapper.unmount()
  })

  it('shows the plain error when the answer is not a server error envelope', async () => {
    mockApi(() => Promise.reject(new Error('Network Error')))

    const wrapper = await clickDownload()

    expect(wrapper.get('.message.error').text()).toContain('Network Error')
    wrapper.unmount()
  })
})
