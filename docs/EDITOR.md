# The writing editor

Polaris's place to write the book itself: formatted text per chapter, laid
out for one page type per book, read from and written to Word (.docx).
Planned in stages; this file is the shared contract every part builds on.
Change it together with the code.

## Scope

Feature parity with Word in what matters to novelists; nothing else.
Out: cloud features, macros and automation, track changes, equations, mail
merge, SmartArt, charts, WordArt.

| Stage | What |
|---|---|
| 1 | The editor (ProseMirror): paragraph and text formatting, named styles, breaks, find and replace, word counts, focus mode, autosave with a copy in the browser, saved versions with restore. A book's page type. |
| 2 | .docx upload: everything stage 1 supports plus pictures, lists, links and bookmarks; split into chapters at Heading 1; a report of what was left out. |
| 3 | Book layout and export: page setup, headers and footers (odd/even/first page), page numbers (formats, restarts), a generated table of contents, page view (paged.js), PDF, .docx download, EPUB, Online Post export (formatted copy, HTML for AO3, text/Markdown). |
| 4 | Polaris features: Tab-to-link names, "appears in" lists, Script Mode and each character's Lines. |
| 5 | Extras: fonts library with a license tracker, a spellchecker that knows the story's names (LibreOffice dictionaries), comments, footnotes, tables, pictures with wrapping, boxes (borders and shading), drop caps, read aloud, offline writing (service worker, IndexedDB, version-checked sync). |

The MVP is stage 1 with a first cut of stage 2 (upload of text and
formatting, split into chapters), so a real manuscript can be brought in
and written on.

## Books and page types

A volume is a book. Chapters without a volume belong to the story's
"loose chapters" book, addressed as book `0`. Each book has one page type,
picked when it is set up; the whole book is formatted for it:

| Key | Page | Default body text |
|---|---|---|
| `a4` | 210 × 297 mm | 12 pt |
| `a5` | 148 × 210 mm | 10.5 pt |
| `a6` | 105 × 148 mm | 8 pt |
| `custom` | width × height in mm (5×8 in, 6×9 in…) | 10.5 pt |
| `online` | no pages: left-aligned, no first-line indent, a blank line between paragraphs, one chapter per post | 12 pt |

Switching a book's page type warns that formatting may not carry over,
saves every chapter as a version labelled "Before switching from X to Y",
then applies the new type's defaults.

Book settings (JSON, stored per book):

    {
      "page_type": "a6",
      "page": { "width_mm": 105, "height_mm": 148,
                "margin_top_mm": 12, "margin_bottom_mm": 12,
                "margin_inner_mm": 12, "margin_outer_mm": 12,
                "gutter_mm": 5, "mirror": true },
      "styles": [ <style>... ]
    }

`page` is ignored for `online`. Stage 3 adds headers, footers and page
numbering here.

## The document (ProseMirror JSON)

Each chapter's text is one ProseMirror document, stored as JSON. The
schema is defined once in `frontend/src/editor/schema.js`; the Go side
(`backend/docx*.go`, word counts, links) reads and writes the same JSON.
Unknown attributes must be kept, not dropped. Lengths are in points (pt),
colours `#rrggbb`, line height a multiple of the font size (1 = single).

Blocks:

- `paragraph`, attrs:
  - `style`: a style id or `null` (the book's Normal);
  - `align`: `left` | `center` | `right` | `justify` | `null` (from the style);
  - `indentFirst`, `indentLeft`, `indentRight`: pt or `null`; a hanging
    indent is a negative `indentFirst`;
  - `spaceBefore`, `spaceAfter`: pt or `null`;
  - `lineHeight`: multiple or `null`;
  - `keepNext`: bool (`false`);
  - `lineExact`: pt or absent: Word's "exactly" line spacing, kept by the
    .docx import for a later export (`lineHeight` then holds pt / (1.2 ×
    font size)); the editor may ignore it;
  - `bookmark`: a name or absent: a bookmark starting in the paragraph
    (the target of `link` `#name`), from the .docx import.
  Content: inline nodes.
- `heading`: attrs `level` (1–6) plus every paragraph attr. Heading 1 is a
  chapter title.
- `scene_break`: an atom; attrs `glyph` (text, default `* * *`) or `image`
  (an uploaded picture used as the ornament), one of the two.
- `page_break`: an atom; a forced new page (ignored for `online`).
- `image`: an atom block; attrs `src` (an `/uploads/…` path), `alt`,
  `width` (pt or `null` for natural), `align` (`left` | `center` | `right`),
  `fullPage` (bool: alone on its own page, like a cover).
- `bullet_list`, `ordered_list` (attr `start`), `list_item` (content:
  paragraph+).

Inline:

- `text`, with marks;
- `hard_break` (a line break inside a paragraph);
- `tab`.

Marks:

- `bold`, `italic`, `underline`, `strike`, `superscript`, `subscript`,
  `smallcaps`;
- `font` {`family`}; `size` {`pt`}; `color` {`color`}; `highlight` {`color`};
- `link` {`href`} (web or `#bookmark`);
- `story_link` {`type`, `id`}: a link to a story item, the same type keys
  and ids as the wiki's `[[type#id]]` links (`backend/wikilinks.go`,
  `frontend/src/wikiLinks.js`); it shows the text it covers.

### Styles

A book's styles are named sets of paragraph attrs and marks:

    { "id": "normal", "name": "Normal", "kind": "paragraph",
      "based_on": null, "builtin": true,
      "para": { "align": "justify", "indentFirst": 14.2, "spaceAfter": 2,
                "lineHeight": 1 },
      "text": { "font": "Merriweather", "size": 8, "color": null } }

`kind` is `paragraph` or `character`. Built-ins (always present, renamable
but not deletable): `normal`, `heading1`–`heading3`, `title`, `subtitle`,
`quote`. A paragraph's own attrs and its text's marks override its style,
which overrides `based_on`, which ends at `normal`. Imported Word styles
keep their Word names.

## The server

All under the story prefix (`/api/…`, rewritten to the story's
database like every other route).

- `GET /api/books/{volume}`: `{ "settings": {...}, "chapters": [ { "id",
  "title", "position", "words", "version" } ] }`, chapters in reading
  order. A book never set up gets the `a5` defaults (with the built-in
  styles) until it is saved; `"is_set_up": false` tells the client to
  offer the page type choice. Also in the answer: `volume`, `title` (the
  volume's, `""` for book 0), `words` (the whole book), `page_types` (the
  keys in order) and `defaults` (every page type's default settings, what
  a switch applies). Book 0 always exists; an unknown volume is `404`.
- `PUT /api/books/{volume}`: `{ "settings": {...}, "switch_label": "…" }`;
  with `switch_label` every chapter of the book is saved as a version with
  that label first, in the same transaction. The server checks the page
  type and the page (each side 50–1000 mm, margins 0–200 mm, gutter 0–100
  mm, at least 20 mm left for the text; `a4`/`a5`/`a6` always get their
  own paper; `online` ignores a bad page), keeps the styles as sent (each
  needs an `id`, a `name` and a `kind`, ids unique) and puts back any
  built-in style that is missing. Keys it doesn't know are kept. Answers
  like GET.
- `GET /api/chapters/{id}/text`: `{ "doc", "version", "words",
  "updated_at" }`; a chapter never written gives an empty doc and version 0.
- `PUT /api/chapters/{id}/text`: `{ "doc", "base_version" }`. If
  `base_version` isn't the stored version the answer is `409` with the
  stored `{ doc, version, words, updated_at }`, so the client can show both
  and let the writer choose. Otherwise it stores the doc, version + 1, and
  answers like GET. The server counts the words. Documents are limited in
  size (5 MB of JSON; `413` past that) and must be valid JSON with a `doc`
  root (`400` otherwise). Saving the stored doc unchanged keeps the
  version. Picture addresses are stored as `/uploads/…`: the
  `/uploads/s/<story>/…` form the browser is given is turned back on save.
- Versions: `GET /api/chapters/{id}/text/versions` (list, newest first:
  id, label, words, automatic, created_at), `GET …/versions/{vid}` (with
  its doc), `POST /api/chapters/{id}/text/versions` `{ "label", "doc"? }`
  (save the current text as a version; with `doc`, save that doc instead
  and leave the text alone: the copy the writer didn't keep after a `409`),
  `POST …/versions/{vid}/restore` (answers like GET text; save the current text as a
  version "Before restoring", then put that version's doc back, version + 1).
  The server also keeps a version on its own at most every 30 minutes of
  writing, labelled "Autosave", and prunes old automatic ones (keep the last
  20 automatic ones per chapter; labelled ones are kept). It is the text
  as it was before the save that crossed the 30 minutes (so after a pause,
  the text as it was left). The labels the server writes ("Autosave",
  "Before restoring") are English keys the client translates.
- `POST /api/books/{volume}/import-docx` (multipart `file`, optional
  `mode`: `append` (default) adds the file's chapters at the end of the
  book): splits the file at Heading 1 (text before the first one becomes a
  chapter titled "Front matter"), creates the
  chapters with their docs, takes pictures into uploads, and answers
  `{ "chapters": [ {id, title, words} ], "page": {width_mm, height_mm,
  margins…}, "page_type_guess": "a6", "styles": [...], "fonts": ["Merriweather", …],
  "left_out": { "headers_footers": 11, "toc": 1, "fields": 9, … } }`. The
  book's settings are not changed by an import; the client offers to apply
  the guessed page type, page and styles.
  Files up to 64 MB (`413` past that or past `parseDocx`'s limits; `400`
  for a file that isn't a .docx or can't be read, and `mode` other than
  `append`). Pictures are saved under `uploads/editor/`; nothing is
  created when anything fails.

  What an import gives (the mapping in detail is at the top of
  `backend/docx*.go`): each chapter's doc starts with its Heading 1, and
  an empty Heading 1 doesn't start one. Formatting is resolved as Word
  does (defaults < style chain < direct; theme fonts). A style takes the
  formatting most of its text actually has (hand-formatted Normal text
  makes `normal` Merriweather 8 pt justified), so paragraphs and text
  carry only what differs from their style. Character styles are listed
  but the text carries their formatting as marks (there is no mark that
  names a character style). All caps is written as capitals; hidden text,
  deleted text, tables of contents, page numbers and page references are
  dropped; other fields keep their shown text; tables and text boxes keep
  their text as paragraphs; nested list levels are items of the same list;
  a numbered heading gets its number in its text. A paragraph of separator
  marks (`* * *`, `***`, `◇◇◇`, `☆ ☆ ☆`, a lone `#` or `⁂`…) is a
  `scene_break`; a picture never is. An anchored picture about the size of
  the page and placed on the page is `fullPage`; one placed wholly off the
  page is left out. Section breaks to a new page become page breaks; the
  page setup is the section with the most words. `left_out` keys:
  `headers_footers`, `toc`, `fields`, `fields_as_text`, `footnotes`,
  `endnotes`, `comments`, `tables`, `text_boxes`, `equations`,
  `tracked_changes`, `drawings`, `objects`, `pictures`, `hidden_text`,
  `drop_caps`, `borders_shading`, `columns`, `sections` (only those above
  zero).

The words of a doc: text of every block, split on whitespace, counting
runs that hold a letter or digit (Unicode letters and decimal digits, as
Go's `unicode.IsLetter`/`IsDigit`); a hard break or tab inside a paragraph
separates words like a space.

## The client

- `/write/:volume` (and `?chapter=ID`), lazily loaded; ProseMirror stays
  out of the main bundle. Entry points: "Write" on a chapter's page and on
  each volume in Chapters.
- Left: the book's chapters (word counts, add, reorder stays in Chapters).
  Middle: the page, as wide as the page type's text block, in the book's
  fonts. Top: the toolbar (style picker, font, size, bold/italic/underline/
  strike, super/sub, small caps, colour, alignment, indents, spacing, line
  height, lists, scene break, page break, picture, link, find and replace,
  undo/redo, focus mode). Bottom: words in the chapter, the book, the
  selection; save state ("Saved", "Saving…", "Not saved yet: kept in this
  browser").
- Word's keyboard shortcuts where they make sense: Ctrl+B/I/U, Ctrl+E/L/R/J
  (alignment), Ctrl+Enter (page break), Ctrl+1/2/5 (line height), Ctrl+F /
  Ctrl+H, Ctrl+Z/Y, Ctrl+Shift+> / < (size), Ctrl+= / Ctrl+Shift+= (sub /
  superscript), Ctrl+Shift+K (small caps), Shift+F3 (change case), Ctrl+S
  (save now).
- Typing helpers: smart quotes, `--` to an em dash, `...` to an ellipsis;
  each undone by Ctrl+Z right after.
- Autosave a second or two after typing stops; every change is first kept
  in IndexedDB (per story and chapter, with its base version) so a closed
  tab or lost connection loses nothing; on opening, a newer local copy is
  offered back. A 409 shows the two texts and lets the writer keep one
  (the other is saved as a version).
- Pasting from Word, Google Docs and web pages keeps the formatting the
  schema knows and drops the rest.
  Black text (how most sources write "automatic") and pictures that aren't
  uploads yet are dropped; pasting into an empty paragraph takes the
  pasted paragraph's formatting.
- "Upload .docx" in the chapter list (and on a book's first visit) calls
  the import route, shows what was made and left out, and offers to take
  the file's page type, page and styles for the book (a page type change
  goes with `switch_label`).

### Client details (stage 1)

- Stored JSON leaves out attrs that are `null` or `false`; any reader takes a
  missing attr as its empty value. Other defaults (`glyph`, `level`,
  `start`…) are always written.
- A line height of 1 is drawn as CSS `line-height: 1.2` (Word's single
  spacing follows the font's line gap); every multiple is scaled the same.
- The page is drawn at the page type's text width in real units (mm, pt)
  and zoomed as a whole (CSS `zoom`, "Fit" by default: the body text near
  16 px as far as the sheet fits). Online Post is 34 em of the body text.
- Indents are shown in mm, spacing and sizes in pt; both are stored in pt.
- A 409's "keep mine" saves the stored text as a version ("Not kept after
  a conflict") then saves mine on its version; "keep the saved text" posts
  mine as a version with `doc`. A copy kept in the browser that was based
  on an older version is put back after saving the current text as a
  version ("Before using the copy kept in this browser").
- The built-in styles and page defaults are also in
  `frontend/src/editor/styles.js` and `pageTypes.js`, used only when the
  server's `defaults` are missing.
- Not yet: inserting pictures from the toolbar (no upload route for the
  editor; imported pictures are drawn), character styles (kept, not
  applied: the schema has no mark for them).
