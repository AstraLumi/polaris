import { t } from './i18n'

// "just now", "5m ago", "3h ago", "2d ago", then a date. SQLite's
// datetime('now') is UTC without a zone marker.
export function timeAgo(stamp) {
  const ts = new Date(String(stamp).replace(' ', 'T') + 'Z').getTime()
  if (Number.isNaN(ts)) return ''
  const mins = Math.max(0, Math.round((Date.now() - ts) / 60000))
  if (mins < 1) return t('just now')
  if (mins < 60) return t('{n}m ago', { n: mins })
  const hrs = Math.round(mins / 60)
  if (hrs < 24) return t('{n}h ago', { n: hrs })
  const days = Math.round(hrs / 24)
  return days < 30 ? t('{n}d ago', { n: days }) : new Date(ts).toLocaleDateString()
}
