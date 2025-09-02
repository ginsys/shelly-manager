<template>
  <div ref="root" class="chart"></div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'

const props = defineProps<{ options: any }>()
const root = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null

onMounted(() => {
  if (root.value) {
    chart = echarts.init(root.value)
    chart.setOption(props.options || {})
  }
})

watch(() => props.options, (opt) => {
  chart?.setOption(opt || {}, true)
})
</script>

<style scoped>
.chart { width: 100%; height: 260px; }
</style>

