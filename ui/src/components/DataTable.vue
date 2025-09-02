<template>
  <div class="data-table">
    <table>
      <thead>
        <tr>
          <slot name="header" />
        </tr>
      </thead>
      <tbody>
        <tr v-if="loading">
          <td :colspan="cols">Loading…</td>
        </tr>
        <tr v-else-if="error">
          <td :colspan="cols" class="error">{{ error }}</td>
        </tr>
        <tr v-else-if="rows.length === 0">
          <td :colspan="cols">No data</td>
        </tr>
        <tr v-else v-for="row in rows" :key="rowKey(row)">
          <slot name="row" :row="row" />
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
defineProps<{ rows: any[]; loading?: boolean; error?: string; cols: number; rowKey: (r:any)=>string }>()
</script>

<style scoped>
.data-table table { width: 100%; border-collapse: collapse; }
.data-table th, .data-table td { text-align: left; padding: 8px; border-bottom: 1px solid #e5e7eb; }
.error { color: #b91c1c; }
</style>

