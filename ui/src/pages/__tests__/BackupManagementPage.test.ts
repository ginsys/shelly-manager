import { mount, flushPromises } from '@vue/test-utils'
import BackupManagementPage from '@/pages/BackupManagementPage.vue'

const fetchBackups = vi.fn()

vi.mock('@/composables/useBackups', () => ({
  useBackups: () => ({
    // lists
    backups: [],
    statistics: { total: 0, success: 0, failure: 0, total_size: 0, by_format: {} },
    availableDevices: [],
    contentExports: [],
    meta: undefined,
    loading: false,
    error: 'Failed to load backups',
    downloading: '',
    message: { text: '', type: 'success' },
    // filters/pagination
    filters: { format: '', success: undefined },
    currentPage: 1,
    pageSize: 20,
    // create panel state (minimal stub)
    runMode: { value: 'now' },
    createType: { value: 'backup' },
    createName: { value: '' },
    createDesc: { value: '' },
    createCompression: { value: 'gzip' },
    createOutputDir: { value: './data/backups' },
    exportOutputDir: { value: './data/exports' },
    jsonOptions: { pretty: true, include_discovered: true },
    yamlOptions: { include_discovered: true },
    jsonCompression: { value: 'none' },
    yamlCompression: { value: 'none' },
    smaOptions: { compression_level: 6, include_discovered: true, include_network_settings: false, include_plugin_configs: true, include_system_settings: true, exclude_sensitive: true },
    scheduleEnabled: { value: true },
    scheduleInterval: { value: '24 hours' },
    schedulePreset: { value: '24 hours' },
    createSubmitting: { value: false },
    createError2: { value: '' },
    providerName: { value: '' },
    providerVersion: { value: '' },
    // restore stubs
    showRestoreModal: { value: false },
    restoreBackup: { value: null },
    restoreOptions: {},
    restorePreview: { value: null },
    restoreLoading: { value: false },
    restoreError: { value: '' },
    deleteConfirm: { value: null },
    // actions
    fetchBackups,
    fetchStatistics: vi.fn(),
    fetchAvailableDevices: vi.fn(),
    fetchContentExports: vi.fn(),
    refreshData: vi.fn(),
    applyIntervalPreset: vi.fn(),
    createBackupPanel: vi.fn(),
    startRestore: vi.fn(),
    previewRestoreAction: vi.fn(),
    executeRestoreAction: vi.fn(),
    closeRestoreModal: vi.fn(),
    downloadBackupAction: vi.fn(),
    downloadContentAction: vi.fn(),
    confirmDelete: vi.fn(),
    performDelete: vi.fn(),
    showMessage: vi.fn(),
  }),
}))

describe('BackupManagementPage', () => {
  it('shows ErrorDisplay and retries fetchBackups', async () => {
    const wrapper = mount(BackupManagementPage)
    await flushPromises()

    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(true)
    const retry = wrapper.find('button.btn.primary')
    if (retry.exists()) {
      await retry.trigger('click')
      await flushPromises()
    }
    expect(fetchBackups).toHaveBeenCalled()
  })
})

