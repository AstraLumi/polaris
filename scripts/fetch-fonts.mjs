// Bundles Google Fonts families with the app (offline, served by Polaris itself).
//   node scripts/fetch-fonts.mjs [--only "Merriweather,Cinzel"]
// Writes: frontend/public/fonts/<slug>/ (woff2 slices, <slug>.css, license),
// frontend/public/fonts/index.json and README.md, and backend/bundledfonts.json
// (license + line metrics; embedded by backend/bundledfonts.go). Only OFL-1.1
// and Apache-2.0 families are kept; anything else is dropped with a message.
// Plain node, no dependencies. With --only it fetches those families and
// leaves the index, README and registry alone.
import { mkdir, writeFile, rm } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const outDir = join(root, 'frontend', 'public', 'fonts')
const UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36'
const RAW = 'https://raw.githubusercontent.com/google/fonts/main'

// [family, group, weights: 'book' = 400/700 with italics, else 400/700]
const FAMILIES = [
  ...['Merriweather', 'Literata', 'Crimson Pro', 'Cinzel', 'Cormorant Garamond', 'Playfair Display'].map((f) => [f, 'Book and headings', 'book']),
  ...['Orbitron', 'Turret Road', 'Exo 2', 'Rajdhani', 'Share Tech Mono', 'VT323', 'Press Start 2P'].map((f) => [f, 'System windows', 'plain']),
  ...['Caveat', 'Patrick Hand', 'Kalam', 'Bangers', 'Permanent Marker'].map((f) => [f, 'Handwriting and sound effects', 'plain']),
  ...['Noto Serif JP', 'Noto Sans JP', 'Zen Old Mincho', 'Shippori Mincho'].map((f) => [f, 'Japanese', 'plain']),
  ...['Noto Sans Symbols', 'Noto Sans Symbols 2', 'Noto Sans Math', 'Noto Emoji'].map((f) => [f, 'Symbols', 'plain']),
]
const LICENSES = {
  ofl: { id: 'OFL-1.1', url: 'https://openfontlicense.org', file: 'OFL.txt' },
  apache: { id: 'Apache-2.0', url: 'https://www.apache.org/licenses/LICENSE-2.0', file: 'LICENSE.txt' },
}
const STATED = { ofl: 'OFL', apache: 'APACHE2' } // METADATA.pb's license field

const only = process.argv.includes('--only')
  ? process.argv[process.argv.indexOf('--only') + 1].split(',').map((s) => s.trim().toLowerCase())
  : null
const slugOf = (f) => f.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
const dirOf = (f) => f.toLowerCase().replace(/[^a-z0-9]/g, '')
const plus = (f) => encodeURIComponent(f).replace(/%20/g, '+')

async function get(url, ua) {
  const res = await fetch(url, ua ? { headers: { 'User-Agent': ua } } : {})
  if (!res.ok) throw Object.assign(new Error(`${res.status} ${url}`), { status: res.status })
  return res
}
const text = async (url, ua) => (await get(url, ua)).text()
const bytes = async (url) => Buffer.from(await (await get(url)).arrayBuffer())

// The stylesheet, asking for the most weights the family has.
async function cssFor(family, kind) {
  const asks =
    kind === 'book'
      ? ['ital,wght@0,400;0,700;1,400;1,700', 'ital,wght@0,400;1,400', 'wght@400;700', 'wght@400', '']
      : ['wght@400;700', 'wght@400', '']
  for (const ask of asks) {
    try {
      return await text(`https://fonts.googleapis.com/css2?family=${plus(family)}${ask ? ':' + ask : ''}&display=swap`, UA)
    } catch (e) {
      if (e.status !== 400) throw e
    }
  }
  throw new Error('no stylesheet')
}

function metaOf(pb) {
  const one = (k) => (pb.match(new RegExp(`^${k}:\\s*"([^"]*)"`, 'm')) || [])[1] || ''
  return { license: one('license'), designer: one('designer'), copyright: (pb.match(/copyright:\s*"([^"]*)"/) || [])[1] || '' }
}

// hhea / OS/2 line heights of a TTF (the maths of backend parseLineMetrics).
function lineMetrics(buf) {
  const n = buf.readUInt16BE(4)
  const t = {}
  for (let i = 0; i < n; i++) t[buf.toString('latin1', 12 + i * 16, 16 + i * 16)] = buf.readUInt32BE(20 + i * 16)
  const upm = buf.readUInt16BE(t.head + 18)
  const h = t.hhea
  const asc = buf.readInt16BE(h + 4)
  const desc = buf.readInt16BE(h + 6)
  const gap = buf.readInt16BE(h + 8)
  const hhea = (asc - desc + gap) / upm
  let win = hhea
  if (t['OS/2'] !== undefined) {
    const wa = buf.readUInt16BE(t['OS/2'] + 74)
    const wd = buf.readUInt16BE(t['OS/2'] + 76)
    if (wa + wd > 0) win = (wa + wd + Math.max(0, gap - (wa + wd - (asc - desc)))) / upm
  }
  const r = (v) => Math.round(v * 1000) / 1000
  return { win: r(win), hhea: r(hhea) }
}

// No User-Agent: Google answers with TTF files.
async function metricsOf(family) {
  const css = await text(`https://fonts.googleapis.com/css?family=${plus(family)}`)
  const m = css.match(/url\((https:[^)]+)\)/)
  return m ? lineMetrics(await bytes(m[1])) : null
}

const registry = {}
const index = []
const dropped = []
let total = 0

for (const [family, group, kind] of FAMILIES) {
  if (only && !only.includes(family.toLowerCase())) continue
  const slug = slugOf(family)
  try {
    let lic = null
    let pb = ''
    for (const k of ['ofl', 'apache']) {
      try {
        pb = await text(`${RAW}/${k}/${dirOf(family)}/METADATA.pb`)
        lic = { ...LICENSES[k], kind: k }
        break
      } catch (e) {
        if (e.status !== 404) throw e
      }
    }
    if (!lic) throw new Error('not in ofl/ or apache/ of google/fonts')
    const meta = metaOf(pb)
    if (meta.license !== STATED[lic.kind]) throw new Error(`license ${meta.license || '?'} is not OFL or Apache`)
    const licenseText = await text(`${RAW}/${lic.kind}/${dirOf(family)}/${lic.file}`)

    const dir = join(outDir, slug)
    await rm(dir, { recursive: true, force: true })
    await mkdir(dir, { recursive: true })
    const css = await cssFor(family, kind)
    const names = new Map() // remote url -> local file
    const used = new Set()
    const blocks = []
    let size = 0
    for (const m of css.matchAll(/(?:\/\*\s*([^*]*?)\s*\*\/\s*)?@font-face\s*\{([^}]*)\}/g)) {
      const subset = (m[1] || 'all').replace(/[^\w-]+/g, '') || 'all'
      const body = m[2]
      const url = body.match(/url\((https:[^)]+)\)/)[1]
      const style = (body.match(/font-style:\s*(\w+)/) || [])[1] || 'normal'
      const weight = (body.match(/font-weight:\s*([\d ]+)/) || [])[1].trim().replace(/\s+/g, '-')
      const range = (body.match(/unicode-range:\s*([^;]+)/) || [])[1]
      if (!names.has(url)) {
        let file = `${weight}${style === 'italic' ? 'i' : ''}-${subset}`
        while (used.has(file)) file += '_'
        used.add(file)
        file += '.woff2'
        const data = await bytes(url)
        await writeFile(join(dir, file), data)
        size += data.length
        names.set(url, file)
      }
      const w = weight.replace('-', ' ')
      blocks.push(
        `/* ${subset} */\n@font-face {\n  font-family: '${family}';\n  font-style: ${style};\n  font-weight: ${w};\n  font-display: swap;\n  src: url(/fonts/${slug}/${names.get(url)}) format('woff2');${range ? `\n  unicode-range: ${range};` : ''}\n}\n`,
      )
    }
    if (!blocks.length) throw new Error('no @font-face in the stylesheet')
    await writeFile(join(dir, `${slug}.css`), blocks.join(''))
    await writeFile(join(dir, lic.file), licenseText)
    const metrics = await metricsOf(family).catch(() => null)
    registry[family.toLowerCase()] = {
      family,
      license: lic.id,
      copyright: meta.copyright,
      license_url: lic.url,
      designer: meta.designer,
      ...(metrics ? { metrics } : {}),
    }
    index.push({ family, slug, group, license: lic.id, designer: meta.designer, copyright: meta.copyright })
    total += size
    console.log(`${family}: ${names.size} files, ${(size / 1048576).toFixed(2)} MB`)
  } catch (e) {
    dropped.push([family, e.message])
    console.log(`DROPPED ${family}: ${e.message}`)
  }
}

if (!only) {
  const sorted = (o) => Object.fromEntries(Object.entries(o).sort())
  await writeFile(join(root, 'backend', 'bundledfonts.json'), JSON.stringify(sorted(registry), null, 2) + '\n')
  await writeFile(join(outDir, 'index.json'), JSON.stringify(index.map(({ family, slug, group }) => ({ family, slug, group })), null, 2) + '\n')
  const js = [
    '// Generated by scripts/fetch-fonts.mjs; do not edit.',
    '// Families bundled with the app: public/fonts/<slug>/<slug>.css, served at /fonts/.',
    'export const BUNDLED_FONTS = [',
    ...index.map((f) => `  { family: ${JSON.stringify(f.family)}, slug: ${JSON.stringify(f.slug)}, group: ${JSON.stringify(f.group)} },`),
    ']',
    '',
  ]
  await writeFile(join(root, 'frontend', 'src', 'editor', 'bundledFonts.js'), js.join('\n'))
  const lines = [
    '# Bundled fonts',
    '',
    'Generated by `scripts/fetch-fonts.mjs`; do not edit. Each family is a Google Fonts family under the SIL Open Font License 1.1 or the Apache License 2.0; the license text sits in its folder. Sources: https://fonts.google.com and https://github.com/google/fonts.',
    '',
    '| Family | License | Designer | Folder |',
    '| --- | --- | --- | --- |',
    ...index.map((f) => `| ${f.family} | ${f.license} | ${f.designer} | ${f.slug}/ |`),
    '',
    ...index.map((f) => `- ${f.family}: ${f.copyright}`),
    '',
  ]
  await writeFile(join(outDir, 'README.md'), lines.join('\n'))
}
console.log(`Total ${(total / 1048576).toFixed(1)} MB; dropped: ${dropped.map((d) => d[0]).join(', ') || 'none'}`)
