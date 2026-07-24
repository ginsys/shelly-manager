import { expect, it } from 'vitest'
import { generateSMAFile } from '../sma-generator'
import { parseSMAFile } from '../sma-parser'
import { validDevice } from './sma-fixture'

it('round-trips non-ASCII data with the same normalized size', async () => {
  const generated = await generateSMAFile({
    devices: [{ ...validDevice, name: 'Cuisine — 温度' }],
  })
  const parsed = await parseSMAFile(await generated.blob!.arrayBuffer())
  expect(parsed.success, parsed.error).toBe(true)
  expect(parsed.archive?.devices[0].name).toBe('Cuisine — 温度')
  expect(parsed.parseInfo.originalSize).toBe(generated.metadata.originalSize)
})
