<template>
  <div class="page">
    <div class="toolbar">
      <router-link class="btn" to="/">← Back</router-link>
      <h1 class="title">Device Details</h1>
      <div class="spacer" />
    </div>

    <div class="card" v-if="loading">
      <div class="state">Loading...</div>
    </div>
    <div class="card" v-else-if="error">
      <div class="state error">{{ error }}</div>
    </div>

    <div v-else class="card details">
      <div class="row"><div class="k">Name</div><div class="v">{{ d.name || '-' }}</div></div>
      <div class="row"><div class="k">Type</div><div class="v">{{ d.type || '-' }}</div></div>
      <div class="row"><div class="k">IP</div><div class="v">{{ d.ip || '-' }}</div></div>
      <div class="row"><div class="k">MAC</div><div class="v mono">{{ d.mac }}</div></div>
      <div class="row"><div class="k">Status</div><div class="v">
        <span :class="['chip', d.status]">{{ d.status || 'unknown' }}</span>
      </div></div>
      <div class="row"><div class="k">Last Seen</div><div class="v">{{ formatDate(d.last_seen) }}</div></div>
      <div class="row"><div class="k">Firmware</div><div class="v mono small">{{ d.firmware || '-' }}</div></div>
      <div class="row"><div class="k">Created</div><div class="v">{{ formatDate(d.created_at) }}</div></div>
      <div class="row"><div class="k">Updated</div><div class="v">{{ formatDate(d.updated_at) }}</div></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getDevice } from '../api/devices'
import type { Device } from '../api/types'

const route = useRoute()
const loading = ref(false)
const error = ref<string | null>(null)
const d = ref<Device>({
  id: 0, ip: '', mac: '', type: '', name: '', firmware: '', status: '', last_seen: ''
})

function formatDate(iso?: string) {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    const id = route.params.id as string
    d.value = await getDevice(id)
  } catch (e: any) {
    error.value = e?.message || 'Failed to load device'
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>

<style scoped>
.page { display: flex; flex-direction: column; gap: 12px; }
.toolbar { display: flex; align-items: center; gap: 8px; }
.title { font-size: 20px; margin: 0; }
.spacer { flex: 1; }
.btn { padding: 6px 10px; border: 1px solid #cbd5e1; background: #fff; border-radius: 6px; cursor: pointer; text-decoration: none; color: inherit; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; overflow: hidden; }
.state { padding: 16px; text-align: center; color: #64748b; }
.state.error { color: #b91c1c; }
.details { display: grid; grid-template-columns: 240px 1fr; gap: 0; }
.row { display: contents; }
.k { padding: 10px 12px; background: #f8fafc; border-bottom: 1px solid #f1f5f9; font-weight: 600; }
.v { padding: 10px 12px; border-bottom: 1px solid #f1f5f9; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.small { font-size: 12px; color: #475569; }
.chip { padding: 2px 8px; border-radius: 999px; font-size: 12px; background: #e2e8f0; color: #334155; }
.chip.online { background: #dcfce7; color: #065f46; }
.chip.offline { background: #fee2e2; color: #991b1b; }
</style>

