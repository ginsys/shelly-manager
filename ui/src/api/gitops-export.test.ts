import { describe, it, expect, vi, beforeEach } from 'vitest'
import { 
  createGitOpsExport,
  downloadGitOpsExport,
  listGitOpsExports,
  getGitOpsExportStatistics,
  getGitOpsExportResult,
  deleteGitOpsExport,
  previewGitOpsExport,
  type GitOpsExportRequest
} from './export'
import api from './client'

// Mock the API client
vi.mock('./client', () => ({
  default: {
    post: vi.fn(),
    get: vi.fn(),
    delete: vi.fn()
  }
}))

const mockApi = api as any

describe('GitOps Export API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('createGitOpsExport', () => {
    it('should create a GitOps export successfully', async () => {
      const mockRequest: GitOpsExportRequest = {
        name: 'Test Export',
        format: 'terraform',
        repository_structure: 'hierarchical',
        devices: [1, 2, 3],
        include_secrets: false,
        generate_readme: true
      }

      const mockResponse = {
        data: {
          success: true,
          data: { export_id: 'test-export-123' }
        }
      }

      mockApi.post.mockResolvedValue(mockResponse)

      const result = await createGitOpsExport(mockRequest)

      expect(mockApi.post).toHaveBeenCalledWith('/export/gitops', mockRequest)
      expect(result).toEqual({ export_id: 'test-export-123' })
    })

    it('should throw error on failed creation', async () => {
      const mockRequest: GitOpsExportRequest = {
        name: 'Test Export',
        format: 'terraform',
        repository_structure: 'hierarchical'
      }

      const mockResponse = {
        data: {
          success: false,
          error: { message: 'Invalid configuration' }
        }
      }

      mockApi.post.mockResolvedValue(mockResponse)

      await expect(createGitOpsExport(mockRequest)).rejects.toThrow('Invalid configuration')
    })
  })

  describe('downloadGitOpsExport', () => {
    it('should download export as blob', async () => {
      const mockBlob = new Blob(['test content'])
      const mockResponse = { data: mockBlob }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await downloadGitOpsExport('test-export-123')

      expect(mockApi.get).toHaveBeenCalledWith('/export/gitops/test-export-123/download', {
        responseType: 'blob'
      })
      expect(result).toBe(mockBlob)
    })
  })

  describe('listGitOpsExports', () => {
    it('should list exports with pagination', async () => {
      const mockExports = [
        {
          id: 1,
          export_id: 'export-1',
          name: 'Test Export 1',
          format: 'terraform',
          repository_structure: 'hierarchical',
          device_count: 3,
          file_count: 5,
          total_size: 1024,
          success: true,
          created_at: '2023-01-01T00:00:00Z'
        }
      ]

      const mockResponse = {
        data: {
          success: true,
          data: { exports: mockExports },
          meta: { pagination: { page: 1, total_pages: 1 } }
        }
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await listGitOpsExports({ page: 1, pageSize: 20 })

      expect(mockApi.get).toHaveBeenCalledWith('/export/gitops', {
        params: { page: 1, page_size: 20, format: undefined, success: undefined }
      })
      expect(result.items).toEqual(mockExports)
      expect(result.meta).toBeDefined()
    })

    it('should handle filters', async () => {
      const mockResponse = {
        data: {
          success: true,
          data: { exports: [] }
        }
      }

      mockApi.get.mockResolvedValue(mockResponse)

      await listGitOpsExports({ format: 'terraform', success: true })

      expect(mockApi.get).toHaveBeenCalledWith('/export/gitops', {
        params: { page: 1, page_size: 20, format: 'terraform', success: true }
      })
    })
  })

  describe('getGitOpsExportStatistics', () => {
    it('should fetch statistics successfully', async () => {
      const mockStats = {
        total: 10,
        success: 8,
        failure: 2,
        by_format: { terraform: 5, kubernetes: 3, ansible: 2 },
        by_structure: { hierarchical: 6, monorepo: 4 },
        total_files: 150,
        total_size: 1024000,
        last_export: '2023-01-01T00:00:00Z'
      }

      const mockResponse = {
        data: {
          success: true,
          data: mockStats
        }
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await getGitOpsExportStatistics()

      expect(mockApi.get).toHaveBeenCalledWith('/export/gitops-statistics')
      expect(result).toEqual(mockStats)
    })
  })

  describe('getGitOpsExportResult', () => {
    it('should fetch export result successfully', async () => {
      const mockResult = {
        export_id: 'test-export-123',
        name: 'Test Export',
        format: 'terraform',
        repository_structure: 'hierarchical',
        device_count: 3,
        file_count: 5,
        total_size: 1024,
        files: [
          {
            path: 'main.tf',
            name: 'main.tf',
            size: 512,
            type: 'config' as const,
            description: 'Main Terraform configuration'
          }
        ],
        duration: '2s'
      }

      const mockResponse = {
        data: {
          success: true,
          data: mockResult
        }
      }

      mockApi.get.mockResolvedValue(mockResponse)

      const result = await getGitOpsExportResult('test-export-123')

      expect(mockApi.get).toHaveBeenCalledWith('/export/gitops/test-export-123')
      expect(result).toEqual(mockResult)
    })
  })

  describe('deleteGitOpsExport', () => {
    it('should delete export successfully', async () => {
      const mockResponse = {
        data: { success: true }
      }

      mockApi.delete.mockResolvedValue(mockResponse)

      await deleteGitOpsExport('test-export-123')

      expect(mockApi.delete).toHaveBeenCalledWith('/export/gitops/test-export-123')
    })

    it('should throw error on failed deletion', async () => {
      const mockResponse = {
        data: {
          success: false,
          error: { message: 'Export not found' }
        }
      }

      mockApi.delete.mockResolvedValue(mockResponse)

      await expect(deleteGitOpsExport('test-export-123')).rejects.toThrow('Export not found')
    })
  })

  describe('previewGitOpsExport', () => {
    it('should preview export successfully', async () => {
      const mockRequest: GitOpsExportRequest = {
        name: 'Test Export',
        format: 'terraform',
        repository_structure: 'hierarchical'
      }

      const mockPreviewResult = {
        preview: {
          success: true,
          file_count: 5,
          estimated_size: 1024,
          structure_preview: ['main.tf', 'variables.tf', 'outputs.tf'],
          template_validation: {
            valid: true,
            terraform: {
              syntax_valid: true,
              provider_compatible: true,
              warnings: []
            }
          },
          warnings: []
        },
        summary: {
          devices: 3,
          estimated_files: 5
        }
      }

      const mockResponse = {
        data: {
          success: true,
          data: mockPreviewResult
        }
      }

      mockApi.post.mockResolvedValue(mockResponse)

      const result = await previewGitOpsExport(mockRequest)

      expect(mockApi.post).toHaveBeenCalledWith('/export/gitops-preview', mockRequest)
      expect(result).toEqual(mockPreviewResult)
    })

    it('should handle preview with validation errors', async () => {
      const mockRequest: GitOpsExportRequest = {
        name: 'Test Export',
        format: 'terraform',
        repository_structure: 'hierarchical'
      }

      const mockPreviewResult = {
        preview: {
          success: false,
          file_count: 0,
          estimated_size: 0,
          structure_preview: [],
          template_validation: {
            valid: false,
            terraform: {
              syntax_valid: false,
              provider_compatible: false,
              warnings: ['Invalid provider configuration']
            }
          },
          warnings: ['Configuration has validation errors']
        },
        summary: {}
      }

      const mockResponse = {
        data: {
          success: true,
          data: mockPreviewResult
        }
      }

      mockApi.post.mockResolvedValue(mockResponse)

      const result = await previewGitOpsExport(mockRequest)

      expect(result.preview.success).toBe(false)
      expect(result.preview.template_validation.valid).toBe(false)
      expect(result.preview.warnings).toContain('Configuration has validation errors')
    })
  })

  describe('Format-specific template validation', () => {
    it('should validate Terraform configuration', async () => {
      const mockRequest: GitOpsExportRequest = {
        name: 'Terraform Test',
        format: 'terraform',
        repository_structure: 'monorepo',
        template_options: {
          terraform: {
            provider_version: '>= 1.0',
            module_structure: 'per-device',
            include_data_sources: true,
            variable_files: true
          }
        }
      }

      const mockResponse = {
        data: {
          success: true,
          data: {
            preview: {
              success: true,
              template_validation: {
                valid: true,
                terraform: {
                  syntax_valid: true,
                  provider_compatible: true,
                  warnings: []
                }
              }
            }
          }
        }
      }

      mockApi.post.mockResolvedValue(mockResponse)

      const result = await previewGitOpsExport(mockRequest)

      expect(result.preview.template_validation.terraform?.syntax_valid).toBe(true)
    })

    it('should validate Kubernetes configuration', async () => {
      const mockRequest: GitOpsExportRequest = {
        name: 'Kubernetes Test',
        format: 'kubernetes',
        repository_structure: 'hierarchical',
        template_options: {
          kubernetes: {
            namespace: 'shelly-manager',
            api_version: 'apps/v1',
            use_kustomize: true,
            include_rbac: true
          }
        }
      }

      const mockResponse = {
        data: {
          success: true,
          data: {
            preview: {
              success: true,
              template_validation: {
                valid: true,
                kubernetes: {
                  api_valid: true,
                  rbac_valid: true,
                  warnings: []
                }
              }
            }
          }
        }
      }

      mockApi.post.mockResolvedValue(mockResponse)

      const result = await previewGitOpsExport(mockRequest)

      expect(result.preview.template_validation.kubernetes?.api_valid).toBe(true)
      expect(result.preview.template_validation.kubernetes?.rbac_valid).toBe(true)
    })
  })
})