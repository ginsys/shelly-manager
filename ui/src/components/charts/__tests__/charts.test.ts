import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import BarChart from '../BarChart.vue'
import LineChart from '../LineChart.vue'

// Mock the installed echarts package. init returns a fake instance so we can assert
// the lifecycle (setOption / resize / dispose) without a real canvas.
const { init, use, setOption, resize, dispose } = vi.hoisted(() => {
  const setOption = vi.fn()
  const resize = vi.fn()
  const dispose = vi.fn()
  const init = vi.fn(() => ({ setOption, resize, dispose }))
  const use = vi.fn()
  return { init, use, setOption, resize, dispose }
})

vi.mock('echarts/core', () => ({ init, use }))
vi.mock('echarts/charts', () => ({ BarChart: {}, LineChart: {} }))
vi.mock('echarts/components', () => ({
  TitleComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  GridComponent: {},
}))
vi.mock('echarts/renderers', () => ({ CanvasRenderer: {} }))

describe.each([
  ['BarChart', BarChart],
  ['LineChart', LineChart],
])('%s', (_name, Component) => {
  let wrapper: VueWrapper | null = null

  beforeEach(() => {
    init.mockClear()
    use.mockClear()
    setOption.mockClear()
    resize.mockClear()
    dispose.mockClear()
  })

  afterEach(() => {
    // Unmount so a component's window 'resize' listener can't leak into the next test.
    wrapper?.unmount()
    wrapper = null
  })

  it('initializes echarts and applies the initial options on mount', async () => {
    const options = { series: [{ type: 'bar', data: [1, 2, 3] }] }
    wrapper = mount(Component, { props: { options } })
    await flushPromises()

    expect(use).toHaveBeenCalledOnce()
    expect(init).toHaveBeenCalledOnce()
    expect(setOption).toHaveBeenCalledWith(options)
  })

  it('re-applies options with merge=true when the prop changes', async () => {
    wrapper = mount(Component, { props: { options: { a: 1 } } })
    await flushPromises()
    setOption.mockClear()

    const next = { a: 2 }
    await wrapper.setProps({ options: next })

    expect(setOption).toHaveBeenCalledWith(next, true)
  })

  it('resizes the chart on window resize', async () => {
    wrapper = mount(Component, { props: { options: {} } })
    await flushPromises()

    window.dispatchEvent(new Event('resize'))

    expect(resize).toHaveBeenCalledOnce()
  })

  it('does not initialize or leak a resize listener when unmounted before the imports resolve', async () => {
    const addSpy = vi.spyOn(window, 'addEventListener')
    // Mount and unmount synchronously: onMounted is still suspended on its first
    // `await import(...)`, so the continuation runs after the component is gone.
    const w = mount(Component, { props: { options: {} } })
    w.unmount()

    await flushPromises()

    expect(init).not.toHaveBeenCalled()
    expect(addSpy).not.toHaveBeenCalledWith('resize', expect.any(Function))
    // And nothing is left listening on the window.
    window.dispatchEvent(new Event('resize'))
    expect(resize).not.toHaveBeenCalled()
    addSpy.mockRestore()
  })

  it('disposes the chart and drops the resize listener on unmount', async () => {
    const removeSpy = vi.spyOn(window, 'removeEventListener')
    wrapper = mount(Component, { props: { options: {} } })
    await flushPromises()

    wrapper.unmount()
    wrapper = null // already unmounted; keep afterEach a no-op

    expect(dispose).toHaveBeenCalledOnce()
    expect(removeSpy).toHaveBeenCalledWith('resize', expect.any(Function))
    // No resize should reach the disposed instance afterwards.
    resize.mockClear()
    window.dispatchEvent(new Event('resize'))
    expect(resize).not.toHaveBeenCalled()
    removeSpy.mockRestore()
  })
})
