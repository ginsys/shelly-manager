<template>
  <main style="padding:16px">
    <h1>Provisioning Agents</h1>
    <p style="color:#6b7280;margin-bottom:16px">Manage provisioning agents, their status and who may enrol</p>

    <div style="margin-bottom:16px">
      <button class="secondary-button" @click="store.fetchAgents()">Refresh</button>
    </div>

    <ErrorDisplay
      :error="actionError"
      :show-retry="false"
      data-testid="agent-action-error"
      @dismiss="clearActionError"
    />

    <section v-if="store.pendingAgents.length" class="pending" data-testid="pending-agents">
      <h2>Waiting for approval ({{ store.pendingAgents.length }})</h2>
      <p class="hint">
        Compare each fingerprint with the one in the agent's own log before you approve. The name and version
        are what the agent sent: they prove nothing.
      </p>
      <table>
        <thead>
          <tr><th>Name</th><th>Fingerprint</th><th>Last Seen</th><th>Actions</th></tr>
        </thead>
        <tbody>
          <tr v-for="row in store.pendingAgents" :key="row.id">
            <td>{{ row.name || row.id }}</td>
            <td><code class="fingerprint">{{ row.fingerprint }}</code></td>
            <td>{{ formatLastSeen(row.lastSeen) }}</td>
            <td>
              <button class="primary-button" :data-testid="`approve-${row.id}`" @click="startAct('approve', row)">Approve</button>
              <button class="danger-button" :data-testid="`reject-${row.id}`" @click="startAct('reject', row)">Reject</button>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <form class="preapprove" data-testid="preapprove-form" @submit.prevent="handlePreapprove">
      <label for="preapprove-input">Pre-approve a fingerprint</label>
      <input
        id="preapprove-input"
        v-model="fingerprintInput"
        data-testid="preapprove-input"
        placeholder="32 hexadecimal characters, or eight groups of four"
        autocomplete="off"
      />
      <button class="secondary-button" type="submit" :disabled="!fingerprintInput.trim()">Pre-approve</button>
    </form>

    <DataTable :rows="store.agents" :loading="store.agentsLoading" :error="store.agentsError" :cols="8" :rowKey="row => row.id">
      <template #header>
        <th>Name</th>
        <th>State</th>
        <th>Fingerprint</th>
        <th>Activity</th>
        <th>Version</th>
        <th>Capabilities</th>
        <th>Last Seen</th>
        <th>Actions</th>
      </template>
      <template #row="{ row }">
        <td>
          <span v-if="row.name">{{ row.name }}</span>
          <span v-else class="muted">Not registered yet</span>
        </td>
        <td><span :class="['status-badge', `state-${row.enrolmentState}`]">{{ row.enrolmentState }}</span></td>
        <td><code v-if="row.fingerprint" class="fingerprint">{{ row.fingerprint }}</code></td>
        <td>
          <span v-if="agentActivity(row)" :class="['status-badge', `status-${row.status}`]">{{ row.status }}</span>
          <span v-else class="muted">—</span>
        </td>
        <td><code>{{ row.version }}</code></td>
        <td>
          <div class="capabilities">
            <span v-for="cap in row.capabilities" :key="cap" class="capability-badge">{{ cap }}</span>
          </div>
        </td>
        <td>
          <span v-if="row.enrolmentState === 'preapproved'" class="muted">—</span>
          <span v-else>{{ formatLastSeen(row.lastSeen) }}</span>
        </td>
        <td>
          <button class="link-button" @click="handleRefreshAgent(row.id)">Refresh Status</button>
          <button
            v-if="row.enrolmentState === 'pending' || row.enrolmentState === 'rejected'"
            class="link-button"
            :data-testid="`row-approve-${row.id}`"
            @click="startAct('approve', row)"
          >Approve</button>
          <button
            v-if="['pending', 'preapproved', 'approved'].includes(row.enrolmentState)"
            class="link-button"
            :data-testid="`row-reject-${row.id}`"
            @click="startAct('reject', row)"
          >Reject</button>
        </td>
      </template>
    </DataTable>

    <section style="margin-top:24px">
      <h2>Summary</h2>
      <div class="stats">
        <div class="card">Total: {{ store.workingAgents.length }}</div>
        <div class="card">Online: {{ store.onlineAgents.length }}</div>
        <div class="card">Busy: {{ store.busyAgents.length }}</div>
        <div class="card">Offline: {{ store.offlineAgents.length }}</div>
        <div class="card">Not enrolled: {{ store.notEnrolledAgents.length }}</div>
      </div>
    </section>

    <div v-if="confirming" class="modal-overlay" data-testid="confirm-dialog" @click="confirming = null">
      <div class="modal-content" @click.stop>
        <h3>{{ confirming.kind === 'reject' ? 'Reject agent' : 'Approve a rejected agent' }}</h3>
        <p v-if="confirming.kind === 'reject'">
          Reject <strong>{{ confirming.name }}</strong>? Its token stops working at once and its assigned and
          in-progress tasks fail. Its pending tasks stay until you cancel them.
        </p>
        <p v-else>
          <strong>{{ confirming.name }}</strong> was rejected. Approving it lets the agent with this
          fingerprint back in.
        </p>
        <div class="modal-actions">
          <button class="secondary-button" data-testid="cancel-act" @click="confirming = null">Cancel</button>
          <button
            :class="confirming.kind === 'reject' ? 'danger-button' : 'primary-button'"
            data-testid="confirm-act"
            @click="confirmAct"
          >{{ confirming.kind === 'reject' ? 'Reject' : 'Approve' }}</button>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useProvisioningStore } from '@/stores/provisioning'
import { agentActivity, type ProvisioningAgent } from '@/api/provisioning'
import DataTable from '@/components/DataTable.vue'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'
import { usePageError } from '@/composables/useNotice'

type Act = 'approve' | 'reject'

const store = useProvisioningStore()
const confirming = ref<{ kind: Act; id: string; name: string } | null>(null)
const fingerprintInput = ref('')
// The failure of an action, above the table. The store keeps no action failure (ginsys/shelly-manager#701).
// A dialog of this page can hide the panel, so a failure that arrives then goes to the app notice.
const { error: actionError, setError: setActionError, clearError: clearActionError } = usePageError(() => confirming.value !== null)

onMounted(() => {
  store.fetchAgents()
})

function formatLastSeen(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)

  if (diffMins < 1) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`

  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}h ago`

  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ago`
}

async function handleRefreshAgent(id: string) {
  clearActionError()
  try {
    await store.refreshAgentStatus(id)
  } catch (e) {
    setActionError(e, { action: 'Refresh agent status', resource: 'Provisioning agent', resourceId: id })
  }
}

// Reject always asks, and so does approving a rejected row, which lets a refused key back in. Approving a
// pending row does not: the operator compared its fingerprint to decide.
function startAct(kind: Act, agent: ProvisioningAgent) {
  const needsConfirm = kind === 'reject' || agent.enrolmentState === 'rejected'
  if (needsConfirm) {
    confirming.value = { kind, id: agent.id, name: agent.name || agent.id }
    return
  }
  void runAct(kind, agent.id)
}

function confirmAct() {
  const act = confirming.value
  confirming.value = null
  if (act) void runAct(act.kind, act.id)
}

async function runAct(kind: Act, id: string) {
  clearActionError()
  try {
    if (kind === 'approve') await store.approveAgent(id)
    else await store.rejectAgent(id)
  } catch (e) {
    setActionError(e, { action: kind === 'approve' ? 'Approve agent' : 'Reject agent', resource: 'Provisioning agent', resourceId: id })
  }
}

async function handlePreapprove() {
  const fingerprint = fingerprintInput.value.trim()
  if (!fingerprint) return
  clearActionError()
  try {
    await store.preapproveFingerprint(fingerprint)
    fingerprintInput.value = ''
  } catch (e) {
    setActionError(e, { action: 'Pre-approve fingerprint', resource: 'Provisioning agent' })
  }
}
</script>

<style scoped>
.badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; background: #e5e7eb; }
.status-badge { padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: 500; }
.status-online { background: #d1fae5; color: #065f46; }
.status-busy { background: #fef3c7; color: #92400e; }
.status-offline { background: #f3f4f6; color: #4b5563; }
.state-approved { background: #d1fae5; color: #065f46; }
.state-pending { background: #fef3c7; color: #92400e; }
.state-preapproved { background: #dbeafe; color: #1e40af; }
.state-rejected { background: #fee2e2; color: #991b1b; }
.state-legacy { background: #f3f4f6; color: #4b5563; }
.capabilities { display: flex; gap: 4px; flex-wrap: wrap; }
.capability-badge { padding: 2px 6px; border-radius: 3px; font-size: 11px; background: #dbeafe; color: #1e40af; }
.stats { display: flex; gap: 12px; }
.card { border: 1px solid #e5e7eb; padding: 8px 12px; border-radius: 6px; }
.link-button { color: #2563eb; cursor: pointer; background: none; border: none; text-decoration: underline; }
.secondary-button { padding: 8px 16px; background: #e5e7eb; border: none; border-radius: 6px; cursor: pointer; }
.primary-button { padding: 6px 12px; background: #2563eb; color: #fff; border: none; border-radius: 6px; cursor: pointer; margin-right: 6px; }
.danger-button { padding: 6px 12px; background: #b91c1c; color: #fff; border: none; border-radius: 6px; cursor: pointer; }
.pending { border: 1px solid #fcd34d; background: #fffbeb; border-radius: 6px; padding: 12px 16px; margin-bottom: 16px; }
.pending table { width: 100%; border-collapse: collapse; }
.pending th, .pending td { text-align: left; padding: 6px 8px; }
.hint { color: #6b7280; margin: 4px 0 8px; }
.muted { color: #6b7280; }
.fingerprint { font-size: 12px; white-space: nowrap; }
.preapprove { display: flex; gap: 8px; align-items: center; margin-bottom: 16px; }
.preapprove input { flex: 1; max-width: 420px; padding: 6px 8px; border: 1px solid #d1d5db; border-radius: 6px; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.4); display: flex; align-items: center; justify-content: center; }
.modal-content { background: #fff; border-radius: 8px; padding: 20px 24px; max-width: 460px; }
.modal-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px; }
</style>
