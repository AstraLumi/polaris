// Writing the service worker at build time (docs/EDITOR.md, "Offline
// writing"): vite.config.js's plugin hands Rollup's bundle to these, and
// emits /sw.js from the template src/offline/sw.js. Pure (node tests).

import { createHash } from 'node:crypto'

// Files from public/ the installed app needs (the page itself is "/").
export const PUBLIC_FILES = ['/', '/site.webmanifest', '/favicon.svg', '/favicon-32.png', '/icon-192.png', '/icon-512.png', '/apple-touch-icon.png']

// What the service worker keeps at install: the entry's scripts and styles
// and everything they import, the writing page's chunk and what it imports,
// and the spellchecker's worker. Other pages' chunks are kept when they
// first load.
export function precacheList(bundle, { pages = [/\/views\/WritePage\.vue$/], workers = [/(^|\/)spell\.worker-[^/]*\.js$/] } = {}) {
  const files = Object.values(bundle)
  const chunks = new Map(files.filter((f) => f.type === 'chunk').map((c) => [c.fileName, c]))
  const out = new Set()
  const add = (c) => {
    if (!c || out.has(c.fileName)) return
    out.add(c.fileName)
    for (const css of c.viteMetadata?.importedCss || []) out.add(css)
    for (const i of c.imports || []) add(chunks.get(i))
  }
  for (const c of chunks.values()) if (c.isEntry || pages.some((re) => re.test((c.facadeModuleId || '').replace(/\\/g, '/')))) add(c)
  for (const f of files) if (workers.some((re) => re.test(f.fileName))) out.add(f.fileName)
  return [...out].sort().map((f) => '/' + f)
}

export const entryFile = (bundle) => Object.values(bundle).find((f) => f.type === 'chunk' && f.isEntry)?.fileName || ''

// The template with this build's values. BUILD changes whenever a built file
// or the template does (built files have their content's hash in their name).
export function serviceWorkerSource(template, bundle, extra = PUBLIC_FILES) {
  const precache = [...extra, ...precacheList(bundle)]
  const entry = entryFile(bundle)
  const build = createHash('sha256')
    .update(template)
    .update(JSON.stringify(Object.keys(bundle).sort()))
    .update(JSON.stringify(precache))
    .digest('hex')
    .slice(0, 12)
  const swaps = [
    ["const BUILD = 'dev'", `const BUILD = ${JSON.stringify(build)}`],
    ["const PRECACHE = ['/']", `const PRECACHE = ${JSON.stringify(precache)}`],
    ["const ENTRY = ''", `const ENTRY = ${JSON.stringify(entry)}`],
  ]
  let out = template
  for (const [from, to] of swaps) {
    if (!out.includes(from)) throw new Error(`service worker template: "${from}" not found`)
    out = out.replace(from, to)
  }
  return { source: out, build, precache, entry }
}
