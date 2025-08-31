import axios from 'axios'

const env: any = (import.meta as any)?.env || {}
const runtimeBase: string | undefined = (globalThis as any)?.window?.__API_BASE__

export const api = axios.create({
  baseURL: env.VITE_API_BASE || runtimeBase || '/api/v1',
  timeout: 10000,
})

// Inject bearer admin key if provided (interim auth)
api.interceptors.request.use((config) => {
  // Prefer Vite env at build-time; fallback to window global at runtime
  const adminKey = env.VITE_ADMIN_KEY || (window as any).__ADMIN_KEY__
  if (adminKey) {
    config.headers = config.headers || {}
    ;(config.headers as any)['Authorization'] = `Bearer ${adminKey}`
  }
  return config
})

export default api
