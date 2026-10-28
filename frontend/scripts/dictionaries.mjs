// Fetches the spellchecker's dictionaries (docs/EDITOR.md, "Spellchecker")
// from the LibreOffice dictionaries repository at a fixed revision into
// frontend/public/dict/: each .aff and .dic gzipped (the browser unpacks
// them, spell.worker.js), with the dictionary's own readme and licence
// files beside it as they are.
//
//   node frontend/scripts/dictionaries.mjs [--revision <commit>]
//
// After changing the revision, change DICT_REVISION in
// src/editor/spellWords.js (it keeps browsers from using an old copy) and
// the revision in NOTICE.md and docs/EDITOR.md.
import { mkdirSync, writeFileSync } from 'node:fs'
import { join, resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { gzipSync } from 'node:zlib'

const here = dirname(fileURLToPath(import.meta.url))
const out = resolve(here, '../public/dict')
const args = process.argv.slice(2)
const at = args.indexOf('--revision')
const REVISION = at >= 0 ? args[at + 1] : '4f1df64b2e8e14751aa7c2205d170ca016071315'
const BASE = `https://raw.githubusercontent.com/LibreOffice/dictionaries/${REVISION}`

// Our folder → [source folder, dictionary name, other files to keep].
const DICTS = {
  en_US: ['en', 'en_US', ['README_en_US.txt']],
  en_GB: ['en', 'en_GB', ['README_en_GB.txt']],
  pt_BR: ['pt_BR', 'pt_BR', ['README_pt_BR.txt', 'README_en.txt']],
  es_ES: ['es', 'es_ES', ['README_hunspell_es.txt', 'LICENSE.md', 'GPLv3.txt', 'LGPLv3.txt', 'MPL-1.1.txt']],
}

async function get(path) {
  const res = await fetch(`${BASE}/${path}`)
  if (!res.ok) throw new Error(`${path}: ${res.status}`)
  return Buffer.from(await res.arrayBuffer())
}

for (const [code, [folder, name, extra]] of Object.entries(DICTS)) {
  const dir = join(out, code)
  mkdirSync(dir, { recursive: true })
  for (const ext of ['aff', 'dic']) {
    const raw = await get(`${folder}/${name}.${ext}`)
    const gz = gzipSync(raw, { level: 9 })
    writeFileSync(join(dir, `${code}.${ext}.gz`), gz)
    console.log(`${code}.${ext}: ${raw.length} bytes, ${gz.length} gzipped`)
  }
  for (const f of extra) writeFileSync(join(dir, f), await get(`${folder}/${f}`))
}
