// The writing editor's server calls (docs/EDITOR.md, "The server"), under
// the open story's prefix like every other call.
import { t, tr } from '../i18n'
import { STORY_API } from '../stories'
import { reportReach } from '../offline/connection.js'

// Messages these calls can give (backend editor.go, chaptertext.go,
// editorimport.go, docx.go); listed so they get translated.
void [
  tr('failed to load the book'), tr('failed to save the book'), tr("failed to load the chapter's text"),
  tr("failed to save the chapter's text"), tr('failed to load the versions'), tr('failed to save the version'),
  tr('failed to restore the version'), tr('failed to import the file'), tr("failed to save the file's pictures"),
  tr('book not found'), tr('chapter not found'), tr('version not found'), tr('invalid request'), tr('invalid form data'),
  tr('that text is too long'), tr('the chapter is too long to save in one piece; split it into two chapters'),
  tr('nothing is written in this chapter yet'), tr('choose a .docx file'), tr('unknown import mode'),
  tr('this .docx file could not be turned into chapters'), tr('this .docx file has no text to import'),
  tr("this .docx file is damaged and can't be read"), tr('this is not a .docx file (Word 2007 or newer)'),
  tr('this .docx file has no document in it'), tr('this .docx file is too large to import'),
  tr('a page must be between 50 and 1000 mm on each side'), tr('the gutter must be between 0 and 100 mm'),
  tr('the margins leave no room for the text'), tr('the margins must be between 0 and 200 mm'), tr('the page size is not valid'),
  tr('a style is either a paragraph style or a character style'), tr('every style needs a name'), tr('pick a page type'),
  tr("the book's settings are not valid"), tr("the book's settings are too large"), tr('the styles are not valid'),
  tr('two styles have the same id'), tr('unknown page type'),
  tr("the book's details are not valid"), tr("the book's layout is not valid"), tr('the chapter options are not valid'),
  tr('a header or footer text is too long'), tr("a header or footer's size must be between 4 and 72 pt"),
  tr('a chapter starts on the next page or on a right-hand page'), tr('unknown page number format'),
  tr('a page number must be between 0 and 99999'),
  tr('failed to save the picture'), tr('choose a picture'), tr('that picture is too large (over 32 MB)'),
  tr('this is not a picture the editor can use (PNG, JPEG, GIF or WebP)'),
]

// A failed call: `status` is the HTTP status (0 when the server couldn't be
// reached); a 409 carries the stored text in `stored`.
export class ApiError extends Error {
  constructor(message, status, stored) {
    super(message)
    this.status = status
    this.stored = stored
  }
  get offline() {
    return this.status === 0
  }
}

async function call(path, options, fallback) {
  let res
  try {
    res = await fetch(`${STORY_API}${path}`, options)
  } catch (e) {
    reportReach(false)
    throw new ApiError(t("can't reach the server"), 0)
  }
  // A proxy in front of Polaris answers these while Polaris is down.
  const gone = res.status === 502 || res.status === 503 || res.status === 504
  reportReach(!gone)
  if (gone) throw new ApiError(t("can't reach the server"), 0)
  if (res.status === 409) {
    let stored = null
    try {
      stored = await res.json()
    } catch (e) {
      /* no body */
    }
    throw new ApiError(t('This chapter was changed somewhere else.'), 409, stored)
  }
  if (!res.ok) {
    const text = (await res.text().catch(() => '')).trim()
    // "chapter 3 of this file is too long…": the number is kept, the rest translated.
    const m = /^chapter (\d+) of this file is too long for one chapter; split it with a Heading 1$/.exec(text)
    if (m) throw new ApiError(t('chapter {n} of this file is too long for one chapter; split it with a Heading 1', { n: m[1] }), res.status)
    throw new ApiError(t(text.length < 200 && text ? text : fallback), res.status)
  }
  return res.status === 204 ? null : res.json()
}

const json = (method, body) => ({ method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })

export const booksApi = {
  // { settings, chapters: [{ id, title, position, words, version }], is_set_up }
  // …plus volume, title, words, page_types and defaults (each type's settings)
  get: (volume) => call(`/books/${volume}`, undefined, 'failed to load the book'),
  save: (volume, settings, switchLabel) =>
    call(`/books/${volume}`, json('PUT', switchLabel ? { settings, switch_label: switchLabel } : { settings }), 'failed to save the book'),
  // { chapters: [{ id, title, words }], page, page_type_guess, styles, fonts, left_out }
  // Every chapter's text, in reading order: [{ id, title, doc }] (the page view).
  docs: (volume) => call(`/books/${volume}/docs`, undefined, 'failed to load the book'),
  importDocx: (volume, file) => {
    const fd = new FormData()
    fd.append('file', file)
    fd.append('mode', 'append')
    return call(`/books/${volume}/import-docx`, { method: 'POST', body: fd }, 'failed to import the file')
  },
  // A picture for an `image` node (PNG, JPEG, GIF or WebP, up to 32 MB): { src }.
  uploadPicture: (volume, file) => {
    const fd = new FormData()
    fd.append('file', file)
    return call(`/books/${volume}/pictures`, { method: 'POST', body: fd }, 'failed to save the picture')
  },
}

export const textApi = {
  // { doc, version, words, updated_at }
  get: (id) => call(`/chapters/${id}/text`, undefined, "failed to load the chapter's text"),
  save: (id, doc, baseVersion) => call(`/chapters/${id}/text`, json('PUT', { doc, base_version: baseVersion }), "failed to save the chapter's text"),
  versions: (id) => call(`/chapters/${id}/text/versions`, undefined, 'failed to load the versions'),
  version: (id, vid) => call(`/chapters/${id}/text/versions/${vid}`, undefined, 'failed to load the versions'),
  // The current text as a version; with doc, that doc instead (the text
  // stays as it is).
  saveVersion: (id, label, doc) => call(`/chapters/${id}/text/versions`, json('POST', doc ? { label, doc } : { label }), 'failed to save the version'),
  restore: (id, vid) => call(`/chapters/${id}/text/versions/${vid}/restore`, { method: 'POST' }, 'failed to restore the version'),
}

// Comments on a chapter's text (stage 5, backend comments.go): rows
// { id, chapter_id, parent_id, text, resolved, resolved_at, created_at, updated_at }.
void [
  tr('failed to load the comments'), tr('failed to save the comment'), tr('failed to delete the comment'), tr('comment not found'),
  tr("the comment's text is not valid"), tr('that comment is too long'), tr('write the reply first'), tr('a reply is resolved with its thread'),
]
export const commentsApi = {
  list: (chapterId) => call(`/chapters/${chapterId}/comments`, undefined, 'failed to load the comments'),
  // { text, parent_id? }: a reply joins its comment's thread.
  create: (chapterId, body) => call(`/chapters/${chapterId}/comments`, json('POST', body), 'failed to save the comment'),
  // { text?, resolved? }
  update: (id, patch) => call(`/comments/${id}`, json('PUT', patch), 'failed to save the comment'),
  remove: (id) => call(`/comments/${id}`, { method: 'DELETE' }, 'failed to delete the comment'),
}

// The story's own words for the spellchecker ("Add to dictionary";
// backend spelling.go): { words: [...] }.
void [tr('failed to load the dictionary'), tr('failed to save the dictionary'), tr('a word is 1 to 64 characters, with a letter and no spaces'), tr("the story's dictionary holds at most 5000 words")]
export const spellingApi = {
  words: () => call('/spelling/words', undefined, 'failed to load the dictionary'),
  change: ({ add = [], remove = [] }) => call('/spelling/words', json('POST', { add, remove }), 'failed to save the dictionary'),
}
