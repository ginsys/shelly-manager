// Prints the API coverage of the UI, one line per `ui/src/api` file: name, functions, referenced and
// the unreferenced names, then a TOTAL line. Usage: node ui/scripts/api-coverage/cli.mjs [repo-root]
// The definition is in measure.mjs and in section 1 of docs/frontend/frontend-review.md.
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { measureApiCoverage } from './measure.mjs'

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(process.argv[2] ?? path.join(here, '..', '..', '..'))
const result = measureApiCoverage(repoRoot)

for (const m of result.modules) {
  console.log([m.file, m.functions, m.referenced, m.unreferenced.join(' ')].join('\t'))
}
console.log(['TOTAL', result.total, result.referenced].join('\t'))
