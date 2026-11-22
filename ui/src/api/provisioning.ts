import api from './client'
import type { APIResponse } from './types'

export interface ProvisioningTask {
  id: string
  deviceId: string
  deviceName: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  taskType: 'configure' | 'update' | 'restart'
  config: Record<string, unknown>
  result?: Record<string, unknown>
  error?: string
  createdAt: string
  updatedAt: string
}

// How an agent got in. A keyless agent is `legacy`; an agent that enrols by key waits as `pending` until the
// operator approves its fingerprint, or is `preapproved` before it first registers.
export type EnrolmentState = 'legacy' | 'pending' | 'preapproved' | 'approved' | 'rejected'

export interface ProvisioningAgent {
  id: string
  name: string
  // The manager stores the enrolment state in `status` until an enrolled agent registers (a pending row
  // has `pending`, a just approved one `approved`); only `online`, `busy` and `offline` say how it is doing.
  status: 'online' | 'offline' | 'busy' | EnrolmentState
  version: string
  capabilities: string[]
  lastSeen: string
  enrolmentState: EnrolmentState
  // Eight groups of four, for an agent enrolled by key; absent for a keyless one.
  fingerprint?: string
}

// A working agent is a keyless one or one the operator approved. A row that waits for approval, was
// pre-approved and has not registered, or was rejected is not an agent that is online or offline, so the
// counts leave it out.
export function isWorkingAgent(agent: ProvisioningAgent): boolean {
  return !['pending', 'preapproved', 'rejected'].includes(agent.enrolmentState)
}

// How a working agent is doing, for the page's Activity column: `online`, `busy` or `offline`, and null for
// everything else. The manager also stores an enrolment state name in `status` (a row just approved has
// `approved` until it registers), and a row that is not a working agent has no activity, however stale.
export function agentActivity(agent: ProvisioningAgent): string | null {
  return isWorkingAgent(agent) && ['online', 'busy', 'offline'].includes(agent.status) ? agent.status : null
}

export interface GetTasksParams {
  status?: string
  page?: number
  limit?: number
}

export interface BulkProvisionRequest {
  deviceIds: string[]
  config: Record<string, unknown>
}

// Tasks
export async function getTasks(params?: GetTasksParams): Promise<ProvisioningTask[]> {
  const res = await api.get<APIResponse<{ tasks: ProvisioningTask[] }>>('/provisioning/tasks', { params })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load provisioning tasks'
    throw new Error(msg)
  }
  return res.data.data?.tasks || []
}

export async function getTask(id: string): Promise<ProvisioningTask> {
  const res = await api.get<APIResponse<ProvisioningTask>>(`/provisioning/tasks/${id}`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Provisioning task not found'
    throw new Error(msg)
  }
  return res.data.data
}

export async function createTask(data: Partial<ProvisioningTask>): Promise<ProvisioningTask> {
  const res = await api.post<APIResponse<ProvisioningTask>>('/provisioning/tasks', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create provisioning task'
    throw new Error(msg)
  }
  return res.data.data
}

export async function cancelTask(id: string): Promise<void> {
  // Empty-body POST: pass {} so axios emits Content-Type for the backend's
  // ValidateContentTypeMiddleware.
  const res = await api.post<APIResponse<void>>(`/provisioning/tasks/${id}/cancel`, {})
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to cancel provisioning task'
    throw new Error(msg)
  }
}

// Bulk operations
export async function bulkProvision(request: BulkProvisionRequest): Promise<ProvisioningTask[]> {
  const res = await api.post<APIResponse<{ tasks: ProvisioningTask[] }>>('/provisioning/bulk', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create bulk provisioning tasks'
    throw new Error(msg)
  }
  return res.data.data.tasks || []
}

// Agents
export async function getAgents(): Promise<ProvisioningAgent[]> {
  const res = await api.get<APIResponse<{ agents: ProvisioningAgent[] }>>('/provisioning/agents')
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load provisioning agents'
    throw new Error(msg)
  }
  return res.data.data?.agents || []
}

export async function getAgentStatus(id: string): Promise<ProvisioningAgent> {
  const res = await api.get<APIResponse<ProvisioningAgent>>(`/provisioning/agents/${id}/status`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load agent status'
    throw new Error(msg)
  }
  return res.data.data
}

// The three enrolment acts (admin key, sent by the shared client). Each answers with the manager's own view
// of the row, which has other field names than the list, so they return nothing and the caller loads the
// list again. Each call names its URL where it is made, which the route contract test reads.
function assertActed(res: { data: APIResponse<unknown> }, failure: string): void {
  if (!res.data.success) {
    throw new Error(res.data.error?.message || failure)
  }
}

// An agent id can hold a space or a slash when it starts with `provisioner-`, so it is escaped in the path.
export async function approveAgent(id: string): Promise<void> {
  const agentId = encodeURIComponent(id)
  const res = await api.post<APIResponse<unknown>>(`/provisioner/agents/${agentId}/approve`, {})
  assertActed(res, 'Failed to approve the agent')
}

export async function rejectAgent(id: string): Promise<void> {
  const agentId = encodeURIComponent(id)
  const res = await api.post<APIResponse<unknown>>(`/provisioner/agents/${agentId}/reject`, {})
  assertActed(res, 'Failed to reject the agent')
}

// Creates a `preapproved` row for the fingerprint, or approves the `pending` row that has it.
export async function preapproveFingerprint(fingerprint: string): Promise<void> {
  const res = await api.post<APIResponse<unknown>>('/provisioner/enrolments', { fingerprint })
  assertActed(res, 'Failed to pre-approve the fingerprint')
}
