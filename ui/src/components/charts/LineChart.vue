<template>
  <div ref="root" class="chart"></div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch, shallowRef } from 'vue'

const props = defineProps<{ options: any }>()
const root = ref<HTMLDivElement>()

// Use shallowRef for large objects like echarts instances
const chartRef = shallowRef(null)

// Lazy load echarts only when component is mounted
let echarts: any = null

onMounted(async () => {
  if (process.env.NODE_ENV === 'development') {
    // Use CDN in development for faster rebuilds
    echarts = await import('https://cdn.jsdelivr.net/npm/echarts@5.5.0/dist/echarts.esm.js')
  } else {
    // Bundle in production - lazy load the entire echarts module
    const { init } = await import('echarts/core')
    const { LineChart } = await import('echarts/charts')
    const {
      TitleComponent,
      TooltipComponent,
      LegendComponent,
      GridComponent
    } = await import('echarts/components')
    const { CanvasRenderer } = await import('echarts/renderers')
    const { use } = await import('echarts/core')

    // Register components
    use([
      TitleComponent,
      TooltipComponent,
      LegendComponent,
      GridComponent,
      LineChart,
      CanvasRenderer
    ])

    echarts = { init }
  }

  initChart()
})

const initChart = () => {
  if (root.value && echarts) {
    chartRef.value = echarts.init(root.value)
    chartRef.value.setOption(props.options || {})
  }
}

watch(() => props.options, (opt) => {
  chartRef.value?.setOption(opt || {}, true)
})
</script>

<style scoped>
.chart { width: 100%; height: 260px; }
</style>

