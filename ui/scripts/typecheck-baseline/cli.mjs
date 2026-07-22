#!/usr/bin/env node
/**
 * Thin CLI wrapper. Argv translation only — all orchestration lives in
 * runner.mjs, and nothing runs on import so the module can be loaded by tests
 * without invoking the compiler.
 *
 * Usage:
 *   node scripts/typecheck-baseline/cli.mjs                       # check (CI gate)
 *   node scripts/typecheck-baseline/cli.mjs --update              # accept reductions
 *   node scripts/typecheck-baseline/cli.mjs --update --move=a=b   # record a rename
 */

import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { runCli } from './runner.mjs'

export function isExecutedAsMain(moduleUrl, argv1) {
  if (!argv1) return false
  return path.resolve(fileURLToPath(moduleUrl)) === path.resolve(argv1)
}

if (isExecutedAsMain(import.meta.url, process.argv[1])) {
  process.exitCode = await runCli(process.argv.slice(2))
}
