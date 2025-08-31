import axios from 'axios'

export const api = axios.create({
  baseURL: (import.meta as any)?.env?.VITE_API_BASE || '/api/v1',
  timeout: 10000,
})

// Inject bearer admin key if provided (interim auth)
api.interceptors.request.use((config) => {
  const adminKey = (window as any).__ADMIN_KEY__ as string | undefined
  if (adminKey) {
    config.headers = config.headers || {}
    ;(config.headers as any)['Authorization'] = `Bearer ${adminKey}`
  }
  return config
})

