<template>
  <div ref="root" class="chart"></div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch, shallowRef } from 'vue'
import type { ECharts } from 'echarts/core'

const props = defineProps<{ options: any }>()
const root = ref<HTMLDivElement>()

// shallowRef: echarts instances are large and must not be made deeply reactive
const chartRef = shallowRef<ECharts | null>(null)

// echarts.init, resolved lazily on mount so the library stays out of the initial bundle
let echartsInit: typeof import('echarts/core').init | null = null
// Set on unmount so a still-pending dynamic import can't initialize a dead component
let unmounted = false

onMounted(async () => {
  // Lazy-load the installed echarts package (tree-shaken core + only what we render)
  const core = await import('echarts/core')
  const { BarChart } = await import('echarts/charts')
  const {
    TitleComponent,
    TooltipComponent,
    LegendComponent,
    GridComponent
  } = await import('echarts/components')
  const { CanvasRenderer } = await import('echarts/renderers')

  // The imports are async: onUnmounted may already have run (e.g. navigating away
  // on a cold cache). Bail out, or we'd init a detached node and leak the resize
  // listener that unmount can no longer remove.
  if (unmounted) return

  core.use([
    TitleComponent,
    TooltipComponent,
    LegendComponent,
    GridComponent,
    BarChart,
    CanvasRenderer
  ])
  echartsInit = core.init

  initChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  unmounted = true
  window.removeEventListener('resize', handleResize)
  chartRef.value?.dispose()
  chartRef.value = null
})

const initChart = () => {
  if (root.value && echartsInit) {
    const chart = echartsInit(root.value)
    chartRef.value = chart
    chart.setOption(props.options || {})
  }
}

const handleResize = () => {
  chartRef.value?.resize()
}

watch(() => props.options, (opt) => {
  chartRef.value?.setOption(opt || {}, true)
})
</script>

<style scoped>
.chart { width: 100%; height: 240px; }
</style>

