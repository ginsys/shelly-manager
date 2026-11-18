import axios, { type InternalAxiosRequestConfig } from 'axios'
import { getAdminKey, requestAdminKey, rejectAdminKey } from './adminKey'

const env: any = (import.meta as any)?.env || {}
const runtimeBase: string | undefined = (globalThis as any)?.window?.__API_BASE__

export const api = axios.create({
  baseURL: env.VITE_API_BASE || runtimeBase || '/api/v1',
  timeout: 10000,
  headers: {
    'User-Agent': 'Shelly-Manager-UI/1.0 (axios)',
  },
})

// Inject bearer admin key if known (interim auth): the key the operator entered for this session,
// else the development sources. See ./adminKey.
api.interceptors.request.use((config) => {
  const adminKey = getAdminKey()
  if (adminKey) {
    config.headers = config.headers || {}
    ;(config.headers as any)['Authorization'] = `Bearer ${adminKey}`
  }
  return config
})

type RetryConfig = InternalAxiosRequestConfig & { _adminKeyRetried?: boolean }

// An admin 401 asks the operator for the key and retries the request once. Only the server's own
// admin guard counts: its envelope carries error.code UNAUTHORIZED in the body (axios sets its own
// error.code, so that field says nothing). A 401 from anything else never opens the dialog.
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const config = error?.config as RetryConfig | undefined
    const adminGuard =
      error?.response?.status === 401 && error.response.data?.error?.code === 'UNAUTHORIZED'
    if (!adminGuard || !config) throw error
    // The route contract test allows one use of the instance here: api.request(<ident>.config).

    if (config._adminKeyRetried) {
      // The key we just sent was refused: no loop, the next key is the operator's act.
      rejectAdminKey(String((config.headers as any)?.Authorization ?? '').replace(/^Bearer /, ''))
      throw error
    }

    const key = await requestAdminKey()
    if (!key) throw error
    config._adminKeyRetried = true
    return api.request(error.config)
  },
)

export default api
