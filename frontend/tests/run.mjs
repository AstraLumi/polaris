// Every frontend test in one go: `npm test` (or `node tests/run.mjs [name…]`
// to run only the tests whose file name contains one of the words).
// A test is any tests/*.test.mjs, plus the translation check; each is a
// plain script that exits with an error when something is wrong, so a new
// test file needs no registering anywhere.

import { spawn } from 'node:child_process'
import { readdirSync } from 'node:fs'
import { availableParallelism } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const dir = path.dirname(fileURLToPath(import.meta.url))
const only = process.argv.slice(2)
const files = ['i18n.check.mjs', ...readdirSync(dir).filter((f) => f.endsWith('.test.mjs')).sort()].filter(
  (f) => !only.length || only.some((w) => f.includes(w)),
)
if (!files.length) {
  console.error('no test matches ' + only.join(', '))
  process.exit(1)
}

const run = (file) =>
  new Promise((resolve) => {
    const started = Date.now()
    const child = spawn(process.execPath, [path.join(dir, file)], { cwd: path.dirname(dir) })
    let out = ''
    child.stdout.on('data', (d) => (out += d))
    child.stderr.on('data', (d) => (out += d))
    child.on('close', (code) => resolve({ file, ok: code === 0, out, ms: Date.now() - started }))
  })

// A few at a time: they are independent processes.
const queue = [...files]
const results = []
const worker = async () => {
  for (let f = queue.shift(); f; f = queue.shift()) {
    const r = await run(f)
    results.push(r)
    console.log(`${r.ok ? 'ok  ' : 'FAIL'} ${r.file} (${r.ms} ms)`)
  }
}
await Promise.all(Array.from({ length: Math.min(4, availableParallelism()) }, worker))

const failed = results.filter((r) => !r.ok)
for (const r of failed) console.error(`\n--- ${r.file} ---\n${r.out.trimEnd()}`)
console.log(`\n${results.length - failed.length} of ${results.length} passed`)
process.exit(failed.length ? 1 : 0)
