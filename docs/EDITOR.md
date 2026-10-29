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

The default body font is Merriweather (`defaultBookFont` in
`backend/editor.go`, `DEFAULT_FONT` in `frontend/src/editor/styles.js`; keep
them equal). Books that already stored settings keep the font they have.

Adding a volume (`POST /api/volumes`) may carry `copy_settings_from: <volume
id>` (400 if it doesn't exist): in the same transaction the new volume gets
that book's settings and `is_set_up` (`copyBookSettings`), except
`chapter_options` (keyed by chapter ids) and `meta.title`, `meta.subtitle`,
`meta.isbn` and `meta.description`. A source never set up copies nothing. The
chapters list tells each volume's `book_set_up`; its "New volume" form offers
"Copy the book settings from {name}" (on by default) for the last volume by
position that has settings.

Book settings (JSON, stored per book):

    {
      "page_type": "a6",
      "page": { "width_mm": 105, "height_mm": 148,
                "margin_top_mm": 12, "margin_bottom_mm": 12,
                "margin_inner_mm": 12, "margin_outer_mm": 12,
                "gutter_mm": 5, "mirror": true },
      "styles": [ <style>... ]
    }

`page` is ignored for `online`. Stage 3 adds `meta`, `layout` and
`chapter_options` (see "Stage 3 and 4 additions" below).

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
  - `boxWidth` (pt, 0 to 20), `boxColor` (`#rrggbb`), `shade` (`#rrggbb`),
    `dropCap` (2, 3 or 4 lines): stage 5, see "Boxes and drop caps" below;
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
  `fullPage` (bool: alone on its own page, like a cover), `wrap`
  (`inline`, the default and what a doc without it means, | `left` |
  `right`: the picture floats and text flows around it; stage 5, see
  "Pictures with text wrapping" below).
- `bullet_list`, `ordered_list` (attr `start`), `list_item` (content: a
  paragraph, then any paragraphs and nested lists: `paragraph block*`).
- `toc` (stage 3): an atom block where a table of contents goes; attrs
  `levels` (1–3, default 1: Heading 1 only) and `title` (default
  "Contents").
- `dialogue` (stage 4, Script Mode): attrs `character` (a character id or
  `null`), `speaker` (the name as shown, kept even when `character` is
  set, so a deleted character still reads), `manner` (text or `""`);
  content: inline (the spoken line, without quotes). The server checks
  `character` is a positive integer or `null` and `speaker` and `manner`
  are text of at most 200 characters. A dialogue is a block anywhere a
  paragraph can be; the editor only makes them at the top level.
- `table` (stage 5): only at the doc's top level (not in a list, not in a
  cell); attrs `borders` (`all`, the default and what a missing attr
  means, `outer` or `none`), `align` (`null` = left, `center`, `right`),
  `width` (`null`: the columns' total, or the text's width when no column
  has one; `"full"`: the text's width; a number of pt, 10–5000).
  Content: `table_row`s, which have no attrs of their own and hold
  `table_cell`s and `table_header`s (prosemirror-tables' conventions; a
  row may be empty when cells above span all of it). Cell attrs:
  `colspan`, `rowspan` (1 when missing, and left out when 1),
  `colwidth` (`null` or one width in pt per column the cell spans, 0 =
  not known), `background` (`#rrggbb` or `null`), `valign` (`null` =
  top, `middle`, `bottom`). Cell content: paragraphs and lists, at least
  one block (no headings: Heading 1 starts chapters; no tables). A
  table's leading rows of header cells are its header rows, repeated on
  each page it runs onto. More in "Tables (stage 5)" below.

Inline:

- `text`, with marks;
- `hard_break` (a line break inside a paragraph);
- `tab`;
- `footnote` (stage 5, an atom): attr `kind` (`footnote`, the default, or
  `endnote`); content one or more paragraphs (plain text and marks, no
  pictures, no comments, no other note). Never stores its number. More in
  "Footnotes and endnotes (stage 5)" below.

Marks:

- `bold`, `italic`, `underline`, `strike`, `superscript`, `subscript`,
  `smallcaps`;
- `font` {`family`}; `size` {`pt`}; `color` {`color`}; `highlight` {`color`};
- `link` {`href`} (web or `#bookmark`);
- `story_link` {`type`, `id`}: a link to a story item, the same type keys
  and ids as the wiki's `[[type#id]]` links (`backend/wikilinks.go`,
  `frontend/src/wikiLinks.js`); it shows the text it covers.
- `char_style` {`id`}: the text uses a character style of the book. Order:
  the text's own marks > its character style (and that style's
  `based_on`) > its paragraph's style.
- `comment` {`id`} (stage 5): the text is commented on; `id` is the row of
  the chapter's comment (a positive integer). A text may carry several
  `comment` marks with different ids (overlapping comments; never the same
  id twice). Not inclusive: typing at its end doesn't extend it. It never
  shows outside the writing page (see "Comments (stage 5)").
- `speaker` {`character`, `speaker`} (stage 5, "Tagging speakers in
  prose"): who said the text. `character` is a character id (positive
  integer) or null, `speaker` the name as shown (up to 200 characters,
  kept so a deleted character still reads). At most one per text, not
  inclusive. Paragraphs and headings may carry it (the editor tags
  paragraphs). It never shows outside the writing page.
- `spell` {`spell`, `name`} (stage 5, "Tag spell"): a spell cast in the
  text. `spell` is a spell id (positive integer, required), `name` the
  spell's name as shown (up to 200 characters, kept so a deleted spell
  still reads). At most one per text, not inclusive, and it can sit on the
  same words as a `speaker`. It never shows outside the writing page.

The doc root may carry attrs: `mode`: `prose` (default) or `script`
(stage 4: the chapter is written in Script Mode). The editor writes
`script` or leaves the attr out (prose); the server refuses any other
value. Other root attrs are kept like any unknown attr.

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
  (id `word-<styleId>`) and text in one carries a `char_style` mark naming
  it, plus as marks only the direct formatting beyond its paragraph style
  and its character style. All caps is written as capitals; hidden text,
  deleted text, tables of contents, page numbers and page references are
  dropped; other fields keep their shown text; tables are tables (see
  "Tables (stage 5)"; one nested in a cell keeps its text as paragraphs in
  that cell); text boxes keep their text as paragraphs; a nested list level (Word's `ilvl`, up to nine
  levels) is a list inside the item before it; a numbered heading gets its number in its text. A paragraph of separator
  marks (`* * *`, `***`, `◇◇◇`, `☆ ☆ ☆`, a lone `#` or `⁂`…) is a
  `scene_break`; a picture never is. An anchored picture about the size of
  the page and placed on the page is `fullPage`; one placed wholly off the
  page is left out. Section breaks to a new page become page breaks; the
  page setup is the section with the most words. `left_out` keys:
  `headers_footers`, `toc`, `fields`, `fields_as_text`, `footnotes`,
  `endnotes`, `comments`, `tables`, `text_boxes`, `equations`,
  `tracked_changes`, `drawings`, `objects`, `pictures`, `hidden_text`,
  `columns`, `sections` (only those above
  zero).

The words of a doc: text of every block, split on whitespace, counting
runs that hold a letter or digit (Unicode letters and decimal digits, as
Go's `unicode.IsLetter`/`IsDigit`); a hard break or tab inside a paragraph
separates words like a space. A `dialogue` counts its spoken line only,
not its speaker or manner (they are attrs, as a script's cue isn't text).
The words in table cells count like any other.

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
- A line height of 1 ("single") is the font's Windows line metrics, as Word
  on Windows computes them: (usWinAscent + usWinDescent + external leading) /
  unitsPerEm, the leading being max(0, hhea.lineGap - ((winAscent +
  winDescent) - (hheaAscender - hheaDescender))); Merriweather's is 1.338,
  its hhea 1.257. The server reads them from the font file (`GET
  /api/fonts/line-metrics?family=X` → `{family, win, hhea}`, 404 when
  unknown; a library family from its stored file, a Google family from its
  TTF, kept in memory and in `font-line-metrics.json` in the data dir;
  `backend/fontmetrics.go`). The client (`fontMetrics.js`) asks it, keeps
  the answer per family (also in localStorage, for offline books), and
  falls back to the browser's `line-height: normal` measured on a hidden
  span. The stylesheet puts the ratio in `--lh` next to the font's `font-family`;
  a paragraph's `line-height` is `calc(multiple * var(--lh, 1.2))`, 1.2
  (`LINE_FACTOR`) while a font is unmeasured. The styles are drawn again
  when a measurement arrives (the editor) or before the layout (the page
  view). `lineExact` (kept in the paragraph's `extra`) is drawn as an exact
  `line-height: Npt`. The exports' own CSS (EPUB, HTML) keeps 1.2.
- Paragraphs get `widows: 2; orphans: 2` (Word's Normal). The page types made
  of pages (all but Online Post) set `font-kerning: none` and
  `font-variant-ligatures: none`, as Word does by default, so lines wrap the
  same. Not read from the .docx: `w:widowControl` off, run-level `w:kern`.
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
- Character styles: the toolbar has a second picker (shown once the book has
  character styles) that puts one on the selected text or takes it off;
  Styles lists them apart from the paragraph styles, makes new ones and
  edits them (text formatting only: font, size, colour, bold, italic,
  underline, strike, small caps; an empty box follows the style it is based
  on, else the text around it). The book's stylesheet draws a style as a rule
  on the mark's `span[data-cstyle]`, so the text's own marks, inside it, win.
  Clear formatting (Ctrl+Space) takes the style off like any other mark.
- Lists: Tab and Shift+Tab move an item one level in or out (under the item
  before it; nine levels at most, the server's nesting limit), Enter on an
  empty item takes it out one level (out of the list from the first).
- Pictures: the toolbar's picture button uploads a file
  (`POST /api/books/{volume}/pictures`) and puts an `image` at the cursor,
  selected; a picture file dropped on the page (where it is dropped) or pasted
  (a screenshot, "copy image"; not when text comes with it) goes the same way.
  A selected picture shows a small box with its width in pt (empty = natural),
  alignment, full page, alt text and Remove (the file stays in the uploads).
  Pasted HTML pictures that are not uploads are still dropped.

## Stage 3 and 4 additions

### Book settings

    "meta": { "title": "", "subtitle": "", "author": "", "language": "en",
              "publisher": "", "isbn": "", "rights": "", "description": "" },
    "layout": {
      "different_first": true,        // a chapter's first page has its own header/footer
      "different_odd_even": true,
      "header": { "odd": <band>, "even": <band>, "first": <band> },
      "footer": { "odd": <band>, "even": <band>, "first": <band> },
      "chapter_start": "next",        // "next" page or "odd" (a right-hand page)
      "numbering": { "format": "arabic", "start": 1 },
      "toc_dots": true
    },
    "chapter_options": { "<chapter id>": { "front_matter": false,
                         "number_restart": null, "no_header": false } }

A band is `{ "left": "", "center": "{chapter}", "right": "",
"font": null, "size": null, "italic": false, "smallcaps": false,
"uppercase": false }`; its texts hold fields: `{page}`, `{chapter}` (the
chapter's title), `{chapter_number}`, `{book}` (meta title or the volume's),
`{author}`. `numbering.format`: `arabic`, `roman` (i, ii), `upper_roman`
(I, II), `alpha`, `upper_alpha`. Front-matter chapters are numbered in
lower roman from i and get no header; numbering restarts at 1 (or
`numbering.start`) at the first chapter that isn't front matter, unless a
chapter sets `number_restart`. Missing keys take these defaults; the
server keeps what it doesn't know. `layout` is ignored for `online`.

The defaults (`defaultBookLayout` in `backend/booklayout.go`, `defaultLayout`
in `frontend/src/editor/bookLayout.js`; keep them equal): header odd
`{chapter}`, even `{book}`, first empty; footer `{page}` centred on every
page; `meta.language` `en`. A missing key inside a band takes the default
band's value. The server checks types and limits (band texts up to 300
characters, sizes 4–72 pt, `chapter_start` `next`/`odd`, the formats
above, page numbers 0–99999, meta fields up to 300 characters and the
description 5000, chapter option keys positive ids) and answers `400`
otherwise. Fields are only the five above; any other `{word}` in a band
is literal text. "Odd" pages are right-hand pages (recto) and "even" ones
left-hand pages, by where they lie in the book, not by their number.
`bookLayoutOf` (Go) reads the three with defaults for the exports.

### Server

- `GET /api/books/{volume}/docs`: every chapter's `{ id, title, doc }` in
  reading order, for the page view and exports.
- What the texts name and who speaks (stage 4, `backend/chapterindex.go`,
  schema 45): every write of a chapter's text (a save, a restore, a .docx
  import; the migration did the texts already written) rebuilds the
  chapter's rows in `chapter_links` (`chapter_id`, `type`, `item_id`,
  `count`) and `chapter_lines` (`chapter_id`, `position`, `character_id`,
  `speaker`, `manner`, `text`). A mention is a run of text with one
  `story_link` (text next to text with the same link, bold or not, is one
  mention) or a dialogue spoken by a character (type `character`).
  `position` numbers a chapter's dialogues from 0 in reading order. Both
  go with their chapter; a deleted item's rows stay until the chapter is
  saved again, and the readers only list chapters that exist.
  - `GET /api/appears/{type}/{id}`: `[{ chapter_id, title, number,
    volume_id, volume_title, count }]` in reading order (`volume_id` 0:
    no volume).
  - `GET /api/characters/{id}/lines`: the same chapter fields plus
    `position`, `speaker`, `manner`, `text` for each line, in reading
    order.
  - `GET /api/chapters/{id}/speakers`: `[{ character_id, name, lines,
    words, first }]`, most lines first (`name` is the character's current
    name, else the speaker as written; `first` the position of their first
    line; `words` counted like the doc's).
- `GET /api/wiki` also lists each character's `nicknames` (every
  version's, the current one's first) for Tab-to-link; they are not link
  targets.
- `POST /api/books/{volume}/pictures` (multipart `file`, PNG/JPEG/GIF/WebP,
  sniffed, up to 32 MB): saves the picture under `uploads/editor/` and
  answers `{ "src": "/uploads/…" }` for an `image` node.
- Exports (stage 3), all `GET` (`?chapter=ID` for one chapter; `404` for
  an unknown book or a chapter not in it), as downloads named after the
  book (`meta.title`, else the volume's; " - <chapter>" for one chapter;
  an ASCII `filename` and an RFC 5987 `filename*`). Pictures are read from
  the story's uploads; a missing one is left out. The code is
  `backend/export*.go`; the mapping in detail is at the top of each file.
  - `/api/books/{volume}/export.docx`: a Word file Word lays out like the
    page view: a section per chapter (next or odd page), page size and
    margins (mirrored, gutter), headers and footers from the bands with
    PAGE fields and the chapter's title written out (front matter and
    `no_header` chapters get none), numbering format and restarts (front
    matter lower roman from i), styles as Word styles (headings with
    outline levels, character styles too), lists (nested ones as deeper
    levels), pictures (`fullPage`: the whole page, over the header and
    footer), scene breaks as centred paragraphs, `toc` as a TOC field
    Word updates when the file is opened (`updateFields`; until then it
    lists the headings without page numbers), dialogue as paragraphs
    "Speaker, manner: “line”" with the speaker in small caps. Exported and
    uploaded again, a book comes back the same but for: the `toc`,
    dialogue (a paragraph), picture ornaments (pictures), page breaks
    around a full-page picture, a null picture width (its natural width),
    story links (plain text), a list item's later paragraphs (plain
    indented ones), ids of styles made in Polaris (`word-<id>`), a renamed
    built-in style's name, styles nothing uses, and sizes off the half
    point. Word's TOC also lists a paragraph (not a heading) set in a
    heading style.
  - `/api/books/{volume}/export.epub`: EPUB 3 (XHTML per chapter, CSS
    from the styles in em of the text, nav from the headings and an NCX
    for older readers, the first `fullPage` picture of the first chapter
    as the cover, `meta` as metadata, the ISBN as the identifier or a
    stable `urn:uuid`); the fonts library's families the book uses go in
    when their license and file allow it (see "Fonts library (stage 5)").
  - `/api/books/{volume}/export.html`, `.md`, `.txt`: Online Post exports.
    HTML is clean for AO3: p, em, strong, u, s, sup, sub, h1–h3, br, ul/
    ol/li, blockquote for paragraphs in the Quote style, a for web links,
    table/thead/tbody/tr/th/td (colspan and rowspan only);
    no classes or styles but `text-align: center` on centred paragraphs;
    scene breaks a centred paragraph with the glyph (hr for a picture
    ornament); fonts, sizes, colours, small caps and pictures left out.
    Markdown (CommonMark, ~~strike~~) and text likewise.
- The .docx import also answers `layout` and `meta` guesses (headers and
  footers mapped onto bands: PAGE fields to `{page}`, a header holding the
  chapter's title to `{chapter}`; the numbering format; the title from
  the core properties), offered to the book like the page type. In
  detail (`backend/docxlayout.go`): the layout comes from the section the
  page comes from, with Word's inheritance of headers between sections;
  default/even/first headers and footers are the odd/even/first bands;
  STYLEREF fields are `{chapter}` too, NUMPAGES and the like are dropped;
  a text that is a chapter's Heading 1 title (ignoring case, spacing and a
  leading "1." number) or that changes from one chapter's section to the
  next ("CHAPTER ONE", "CHAPTER TWO"…) becomes `{chapter}`, with the text
  around it kept; where text goes comes from positional tabs, tab stops or
  the paragraph's alignment; font, size, italics, small caps and capitals
  from the first run (the font left out when it is the body's);
  `different_first` from the section's titlePg, `different_odd_even` from
  evenAndOddHeaders, the format from pgNumType, `start` from the first
  section holding a Heading 1, `chapter_start` `odd` when most chapter
  sections start on an odd page. `layout` is absent when the file has no
  headers, footers or page numbering. `meta` holds only what the core
  properties fill (`title`, `subtitle` from the subject, `author` from the
  creator, `description`, `language`). Each new chapter in the answer
  carries `front_matter: true` when it is the text before the first
  Heading 1, and the import marks those chapters as front matter in the
  book's `chapter_options` itself (in its transaction; a book never set
  up stays so), whether or not the file's layout is applied. The import
  report tags them and doesn't count them as chapters.
  Starts that go up from one chapter's section to the next are Word
  carrying the count on; a start lower than the previous one is a restart,
  given as the chapter's `number_restart` in the answer (applied with the
  layout). A `fullPage` picture goes at the start of the page its anchor
  paragraph is on (after the page break before it, after its chapter's
  Heading 1, or first), as Word draws it over that page.

### Client

- Page view (stage 3): the whole book laid out in pages with paged.js
  (CSS paged media: `@page` size and mirrored margins, running headers
  and footers from the bands, page counters with the format and restarts,
  chapters starting on the next or a right-hand page, the `toc` with page
  numbers and dot leaders), read-only, with "Print / save as PDF". A click
  on a page opens that chapter in the editor near that spot. Loaded lazily
  only with the page view.
  Built (stage 3): `/write/:volume/pages` (`views/BookPagesPage.vue`;
  "Page view" beside the breadcrumbs of the writing page), `pagedjs`
  0.4.3 loaded by `editor/pageView.js` only there. The chapters are drawn
  with the schema's `toDOM` and the styles' stylesheet, each chapter a
  `section` whose top-level blocks carry `data-ch` / `data-blk` (the
  editor opens at `?chapter=ID&at=N`, block N of the chapter's doc, put in
  sight). The CSS comes from `pagedCSS` in `editor/bookLayout.js` (pure,
  `tests/bookLayout.test.mjs`): named pages `pvmain` (with header),
  `pvnohead` (`no_header`), `pvfront` (front matter: lower roman, no
  header) and `pvfull`; `:right` takes the odd band, `:left` the even one,
  `:first` (each chapter's first page) the first one; `{chapter}` and
  `{chapter_number}` are named strings set from each chapter's section;
  restarts are `counter-reset: page N` on the chapter. A `fullPage`
  picture is alone on a `pvfull` page with no margins and no header or
  footer (at the start of a chapter it goes before the chapter, so a cover
  comes first). `page_break` is a break after. The table of contents
  lists every heading up to its `levels` (a chapter without a Heading 1,
  other than front matter, by its title) with `target-counter` page
  numbers in the target chapter's format and a dotted leader when
  `toc_dots`. The book lays out hidden with a progress bar (pages so far
  and how much of the text is done) and can be stopped; "Print / save as
  PDF" prints the pages alone at their size. Online Post books get a
  reading view (one column, no pages) instead.
- In the editor (stage 3) the `toc` node is drawn as a box with its title
  and the chapters that aren't front matter (no numbers); it is inserted
  from the toolbar (folds behind "⋯" on phones; disabled for Online Post).
  Each chapter in the chapter list has a "…" menu: front matter, no
  running header, and "Page numbers and layout…" (the book panel at that
  chapter); a front-matter chapter's number shows as "·", and the others
  are numbered among the chapters that aren't front matter, there and
  wherever the story shows a chapter's number (`GET /api/chapters` gives
  front matter `number` 0 and `front_matter: true`, from the book's
  chapter options).
- Export menu: .docx, EPUB, PDF (page view's print), and for Online Post
  "Copy as formatted text" (clipboard HTML of the chapter), HTML, Markdown
  and text.
  (Built: `components/write/ExportMenu.vue`, an "Export ▾" menu in the
  page view and an "Export" section of the book panel, which knows the
  open chapter from the writing page's `?chapter=`. Downloads are links
  with `download`; PDF prints the page view (from the book panel it opens
  `/write/:volume/pages?print=1`, which prints once laid out). "Copy …
  as formatted text" puts export.html (text/html) and export.txt
  (text/plain) on the clipboard, falling back to copying a selection;
  the chapter's when one is open, else the book's. Word and EPUB and PDF
  are left out of the menu for Online Post.)
- Book panel (opened from the toolbar or the status bar's "Page: A6" button, which names the book's page type): the meta fields, the header and footer bands with a field
  picker, numbering, chapter start, front-matter flags per chapter.
  (Built: sections "Book details", "Headers and footers" (header/footer ×
  right/left/first pages, each with left/centre/right texts, a field
  picker that inserts at the cursor, a preview, font, size, italic, small
  caps, capitals), "Page numbers and chapters" (format, first number,
  chapter start, contents' dots) and "Chapters" (front matter, no header,
  restart at). The import report offers the file's layout and details
  beside its page type and styles.)
- Script Mode (stage 4): a chapter's mode switch; in Script Mode Enter
  after a dialogue starts a line for the next speaker, `Name, manner:` at a line's
  start becomes a dialogue (Tab completes the speaker from the story's
  characters), lines without that shape stay paragraphs (action). Drawn
  like a script: the speaker in small caps in the character's colour, the
  manner in italics, the line indented below.
  Built (`editor/script.js`, the toolbar's speech-bubble button, folded
  behind "⋯" on phones; switching only sets the root's `mode`):
  `Name:` or `Name, manner:` (a speaker of up to six words and 60
  characters, a manner up to 200; `parseSpeakerLine` in
  `editor/nameMatch.js`) typed at the start of a top-level paragraph
  becomes a dialogue when a space follows; the speaker is kept as typed
  and `character` is the character whose name, nickname or former name
  it is exactly (ignoring case and accents), else `null`; text after the
  caret becomes the line; Ctrl+Z right after undoes it. Enter at the end
  of a dialogue starts an empty paragraph (not a new dialogue: the next
  line may be action or another speaker); Enter inside one splits it into
  two lines of the same speaker. Backspace at a dialogue's start turns it
  back into the paragraph "Speaker, manner: line" (in both modes). A
  click on the speaker opens a small box: the character (a picker), the
  speaker as shown and the manner, and "Back to plain text"; picking a
  character names the speaker after them unless the speaker is already
  one of their names. Characters have no colour, so the speaker is drawn
  in the accent (darkened toward the ink on the paper). Prose Mode draws
  dialogues the same, so nothing is hidden. The page view draws them
  with the same classes (`.pm-dialogue`, `.pm-who`, `.pm-line`).
- Tab-to-link (stage 4): typing three or more letters matching the start
  of a story item's name, nickname or former name shows the rest in faint
  text; Tab accepts it as a `story_link` (the typed text kept when it was a
  nickname), Esc or typing on dismisses; with several matches a small list
  opens (↑/↓). Tab does nothing special without a suggestion.
  Built (`editor/nameSuggest.js`, matching in `editor/nameMatch.js`, pure,
  `tests/nameMatch.test.mjs`): the items are `GET /api/wiki`'s (every
  type), loaded with the page and again when the window gets the focus
  back. The typed start begins a word (not after a letter, digit or
  apostrophe) and the caret ends one; it may hold several words ("Mira V"
  for "Mira Vey", the longest start that matches wins) but never crosses
  a link, an inline atom or a hard break, and nothing is offered inside a
  link. Accepting replaces what was typed with the name (or nickname, or
  former name) as the story writes it, keeping the typed text's
  formatting. A whole name already typed shows a small "⇥": Tab links it.
  Order: characters, then names starting with exactly what was typed,
  then shorter, then by name; eight at most. With a list, Enter also
  accepts; a click (or a tap on the faint text, for phones) accepts.
  Pasted, dropped or undone text never opens a suggestion. In Script
  Mode, a name typed at the start of a top-level paragraph is a speaker:
  only characters are offered and the name goes in as plain text, ready
  for ":". A `story_link` is drawn with a dotted underline; its `href` is
  the item's wiki page, so hovering shows the wiki's preview card and
  Ctrl/Cmd+click opens it.
- "Appears in" (stage 4): the chapters naming the item, with counts, each
  opening the chapter in the editor: in a character's Story tab and as a
  section after "What links here" on every wiki article. A character's
  "Lines" tab lists what they say in reading order (search, a chapter
  filter with counts; a line opens the editor at `?chapter=ID&line=N`,
  the chapter's dialogue N selected). A chapter's page shows "Who speaks"
  when its text has dialogue. The wiki graph doesn't use the texts:
  chapters aren't nodes there, and items named in the same chapter would
  join almost everything.

## Where things stand, and stage 5

Status (2026-10-29): stages 1–4 are built and pushed (`126fdc3` the MVP,
`12df0da` stages 3–4; schemas 42–45). Stage 5 items 1 (fonts library),
2 (spellchecker), 3 (comments, schema 46), 4 (footnotes and endnotes),
5 (tables), 6 (boxes and drop caps), 7 (pictures with text wrapping),
8 (read aloud) and 9 (offline writing) are built and pushed: all of
stage 5's items are built.
The writer's decisions behind this plan: parity with Word in what matters
to novelists, nothing else (no cloud, macros, track changes, equations);
page types A4, A5, A6, custom and Online Post, one per book; EPUB yes;
a spellchecker that knows the story's names yes; offline writing yes;
the writer's own fonts with a license tracker yes. Build each part so it
can be tested on its own, with node/Go tests and headless screenshots
(the scripts under `frontend/tests/visual/`; throwaway data only, servers
on 127.0.0.1). A real manuscript to test with is never committed.

Known gaps in what exists:

- Tab at the end of a sublist's first item types a tab; uploaded pictures
  are never cleaned up when no text or version uses them.
- The editor's `toc` box lists chapter titles only (levels 2–3 show only
  in the page view); a page that continues after a mid-chapter full-page
  picture may be treated as a chapter's first page by paged.js; headings'
  `break-after: avoid` isn't honoured by paged.js; printing was checked
  headless only, not from a real browser's print dialog.
- Characters have no colour field, so Script Mode speakers use the accent.
- The suggestion list for Tab-to-link scales with the page zoom.
- AO3 may strip `style="text-align:center"` in the HTML export (fallback:
  `align="center"`); WebP pictures go into .docx as they are.
- Stage 5 parts were checked with node/Go tests and headless screenshots
  only: not heard (read aloud), not opened in Word or LibreOffice (comments,
  tables), not tried on a phone. Each part's subsection lists its own gaps.
- In `writeShots.mjs` (tables step), Ctrl+End pressed right after Shift+Tab
  in a table sometimes leaves the caret in the table in the merged build
  (never on the tables branch alone; not the spellchecker: it happens with
  it off). The script now settles first; the cause isn't known.

Stage 5, suggested order (each its own piece of work):

1. **Fonts library with a license tracker.** (Built: see "Fonts library
   (stage 5)" below; .docx embedding not done.) One library for all stories
   (not per story database): upload .ttf/.otf/.woff/.woff2; read the
   file's name table (family, styles, copyright, license text and URL)
   and the OS/2 `fsType` embedding flags in Go. Per font: license kind
   (OFL, Apache, commercial, personal use only, other), source, purchase
   date, an attached receipt or license file, allowed uses (print, ebook,
   web, commercial). "Used in": which books and styles use it. Warnings
   when it matters: a personal-use font in a book's styles, exporting PDF
   or .docx/EPUB with a font that forbids embedding. Then embed allowed
   fonts in EPUB and (optionally) .docx, and serve them to the editor and
   page view with `@font-face`. Google Fonts stay pickable as today.
2. **Spellchecker that knows the story.** (Built: see "Spellchecker
   (stage 5)" below.) LibreOffice/Hunspell
   dictionaries (en, pt-BR, es to match the interface), checked in a
   worker; every character, place, faction, creature, asset name,
   nickname and former name counts as a word; a per-story "add to
   dictionary". Underlines through a ProseMirror decoration plugin, with
   suggestions on right click. The browser's own spellcheck is turned off
   in the editor once this is on.
3. **Comments.** Built (schema 46): see "Comments (stage 5)" below.
4. **Footnotes and endnotes.** Built: see "Footnotes and endnotes
   (stage 5)" below.
5. **Tables.** prosemirror-tables; for stat blocks in LitRPG; import,
   export, page view. Built: see "Tables (stage 5)" below.
6. **Boxes and drop caps.** Built: see "Boxes and drop caps (stage 5)"
   below.
7. **Pictures with text wrapping.** Built: see "Pictures with text
   wrapping (stage 5)" below.
8. **Read aloud** (built; see "Read aloud (stage 5)" below). The browser's speech synthesis reading the selection or
   from the caret, with the current sentence highlighted.
9. **Offline writing.** Built: see "Offline writing (stage 5)" below.

### Read aloud (stage 5)

The toolbar's speaker button (folded behind "⋯" on phones; disabled with a
tooltip when the browser has no `speechSynthesis`) or Ctrl+Alt+Space
reads the selection, or from the sentence holding the caret to the end of
the chapter. The same shortcut pauses and resumes. Code:
`src/editor/readAloud.js` (pure, `tests/readAloud.test.mjs`),
`readAloudPlugin.js` (the decoration plugin and the reader),
`components/write/ReadAloudBar.vue` (the floating control).

- **One utterance per sentence**, not boundary events: many voices give no
  `boundary`, and Chrome stops long utterances after about 15 s. The
  highlight (`.read-now`, a decoration, theme variables `--read`) therefore
  follows exactly and is scrolled gently into view when it leaves the
  middle of the page. A sentence over 220 characters is spoken in pieces
  cut at a comma or space (`chunkText`), still one highlight.
- `splitSentences` ends a sentence at `.` `!` `?` `…` (runs of them), with
  closing quotes and brackets kept; it does not end it after an
  abbreviation (English: Mr., Mrs., Dr., St., e.g., i.e. ...; pt/es: Sr.,
  Sra., Dr., ...), a lone initial, inside a number (3.5), or when the next
  word starts in lower case (`"Stop!" she said.`, `wait... and then`).
  `collectSentences` maps each sentence back to document positions
  (`from`/`to`) and gives the silence after it: headings 900 ms,
  paragraphs 350 ms, a scene break 1100 ms. Pictures, page breaks and the
  contents box are skipped; a selection cuts sentences at its edges.
- A dialogue line is spoken as "Speaker: line" (its first sentence only;
  the `speakers` option, on). Nothing in the UI turns it off yet.
- Pause cancels the sentence and says it again from its start on resume
  (`speechSynthesis.pause()` is unreliable). Previous/next sentence work
  while playing or paused. Speed 0.5-2x; the voice picker lists the voices
  of the book's `meta.language` first, then the rest (`voiceschanged`
  fills it in); both are remembered per browser in localStorage
  (`polaris.readAloud`, wrapped in try/catch). Changing either restarts
  the current sentence.
- Stops: Esc, the stop button, a click in the text, any change to the
  document (typing, a restored version), opening another chapter,
  leaving the page. Clicking elsewhere (toolbar, panels) does not.
- Checked headless only with a stand-in `speechSynthesis` that records what
  would be said (`writeShots.mjs`, step 5b); real voices and the browsers'
  quirks have not been heard.

### Fonts library (stage 5)

One library for every story (`backend/fontlib.go`, the files read by
`backend/fontparse.go`; client `frontend/src/fontLibrary.js`, pure rules
in `fontRules.js`, `tests/fontLibrary.test.mjs`).

Storage: two tables in the registry (`<DATA_DIR>/stories.db`, made and
upgraded in place beside `stories`, not a story migration):
`font_families` (`name` unique ignoring case, `license_kind` `""` |
`ofl` | `apache` | `commercial` | `personal` | `other`, `source`,
`purchase_date` `YYYY-MM-DD` or `""`, the attached `license_file` and its
original name, `use_print` / `use_ebook` / `use_web` / `use_commercial`,
`notes`) and `font_files` (family, stored name, original name, `format`
`ttf` | `otf` | `woff` | `woff2`, size, `weight` 100–900, `italic`, and
what the file says: style, full name, copyright, license text and URL,
designer, vendor, `fs_type` (NULL when unread), `details_read`). Files in
`<UPLOADS_DIR>/_fonts/` (never a story's folder: story ids are 16 hex
digits), named by content (the first 32 hex digits of the SHA-256, then
the format; the same file twice is refused); receipts and license files in
`_fonts/licenses/` under random names. Story backups and transfers don't
carry fonts.

Reading a file: sniffed from its first bytes (TTF `00010000`/`true`, OTF
`OTTO`, WOFF, WOFF2; `.ttc` collections refused; 40 MB each, 20 per
upload). The `name` table (Windows English first, then any Windows or
Unicode record, then Mac Roman; the typographic family and style, ids 16
and 17, win over 1 and 2), OS/2 `usWeightClass` (put on the 100 steps;
1–9 read as 100–900), `fsSelection` (italic, oblique, bold) else `head`'s
`macStyle`, and `fsType`: `installable` (0), else the least restrictive
bit set: `editable` (8), `preview` (4), `restricted` (2); bits 8 (no
subsetting) and 9 (bitmap only) apart. A file forbids embedding when it
is restricted or bitmap only. WOFF tables are inflated with zlib. WOFF2
needs Brotli, which the standard library lacks and which isn't worth a
dependency for a few names: a WOFF2 is kept with `details_read` false,
its family, weight and slant guessed from the file name
("Foo-SemiBoldItalic.woff2"), its embedding `unknown`; the UI says so and
the writer corrects them. A new family's license is guessed from the
file's license text, URL and copyright: the SIL Open Font License or the
Apache License → that kind with every use ticked; "personal use" or
"non-commercial" → `personal` with none.

Routes (global, beside `/api/stories`):

- `GET /api/fonts`: `[{ id, name, license_kind, source, purchase_date,
  license_file (its name or ""), uses: { print, ebook, web, commercial },
  notes, created_at, files: [{ id, url, format, size, original_name,
  weight, italic, style, full_name, copyright, license, license_url,
  designer, vendor, details_read, embedding, no_subsetting, bitmap_only
  }] }]`, by name.
- `POST /api/fonts` (multipart `file`, one or more; `family_id` puts them
  all in that family, else each goes in the family its name gives, made
  when new): `{ families, added, problems: [{ file, error }] }`; `400`
  (or `413`) with the first problem when nothing was added.
- `PUT /api/fonts/{id}`: any of `name`, `license_kind`, `source`,
  `purchase_date`, `uses`, `notes`, and `files: [{ id, weight, italic }]`;
  answers the family. `DELETE /api/fonts/{id}` removes the family, its
  files and its license file.
- `POST|GET|DELETE /api/fonts/{id}/license-file`: a PDF, picture or text
  (sniffed, 20 MB), downloaded as an attachment, `nosniff`, sandboxed.
- `DELETE /api/font-files/{fid}`: one file (answers its family).
- `GET /api/fonts/used`: `{ "<family id>": [{ story_id, story, volume,
  book, family, styles: [names], bands, chapters }] }`, scanning every
  story on demand (books set up or with chapters: their styles'
  `text.font`, header/footer bands' `font`, and `font` marks of chapter
  texts that name a font).
- `GET /uploads/fonts/{name}`: the file, its font type,
  `Cache-Control: immutable` (the name is the content), `nosniff`.
- Under each story: `GET /api/books/{volume}/fonts`, the same per-book
  list for one book (`[{ volume, book, family, styles, bands, chapters }]`),
  each row also carrying its source, license and verdict (see "Fonts and
  licenses").

EPUB: the export handler gets the library from the request's context (the
story manager adds it) and embeds the files of the families the e-book's
CSS names (styles and text marks, not bands) whose license has `ebook`
and whose file doesn't forbid embedding (an unread WOFF2 is left to the
license): `OEBPS/fonts/fNN.<format>`, an `@font-face` each at the top of
`style.css`, manifest items, obfuscated the IDPF way (first 1040 bytes
XORed with the SHA-1 of the unique identifier) and listed in
`META-INF/encryption.xml`. .docx embedding (Word's obfuscated ODTTF) is
not done: Word falls back to an installed font of that name.

Client: `/fonts` (`views/FontsLibrary.vue`, lazily loaded; linked from
Settings → Fonts, the Styles panel's "Manage fonts…" and the warnings):
upload by `FilePick` (several files), each family as a card with a line
in its own font (editable sample text), its faces, license kind, uses,
warnings and "Used in" (story › book: styles, headers and footers,
chapters); "License…" opens `FontLicenseModal.vue` (name, kind, purchase
date, where from, uses, receipt, notes, each file's weight and italic, and
what each file says; `useModalGuard`); the "…" menu adds styles to the
family, opens the receipt, deletes (warning when books use it). Loading
the library puts an `@font-face` for every file in the page
(`#polaris-font-library`), so the editor, page view and library draw
library families; `editor/fonts.js` lists library families first in the
pickers (toolbar, Styles panel, bands share the `write-fonts` list),
loads them with `document.fonts.load` and never asks Google for them.
Warnings (`fontRules.js`): a library family a book style uses that is
personal use only or not ticked for commercial use shows in the Styles
panel and the book panel; Word, EPUB and PDF in the Export menu first ask
`GET /api/books/{volume}/fonts` and confirm (never block) when a library
family lacks the use (print for PDF and Word, ebook for EPUB, where
families only bands use don't count) or its file forbids embedding. The
library page warns about an unrecorded license, embedding, and a license
problem when a book's styles use the family.

Not verified: the page in a real browser (only the build and the node and
Go tests), and readers opening the embedded fonts (the obfuscation
round-trip is tested).

### Bundled fonts

The app ships 26 Google Fonts families (OFL-1.1 or Apache-2.0 only: book and
heading serifs, system-window and handwriting display faces, Japanese, symbols)
in `frontend/public/fonts/<slug>/` (woff2 split by unicode-range, one
`<slug>.css`, the license text), served at `/fonts/`. `scripts/fetch-fonts.mjs`
downloads and regenerates all of it, plus `backend/bundledfonts.json` (license,
copyright, designer, line metrics read from the TTF, so `/api/fonts/line-metrics`
needs no Google for them), `src/editor/bundledFonts.js` (the family list) and
`public/fonts/README.md`. `loadFont` adds the family's local stylesheet (the page
view and PDF use the same document fonts); the service worker keeps `/fonts/`
for offline writing. A library family of the same name wins. The EPUB export
does not embed bundled fonts (only library fonts); readers use their own.

### Fonts and licenses

The Book and page panel's "Fonts and licenses" section tells a writer every
font their book uses and whether they may publish with it. It reads
`GET /api/books/{volume}/fonts` (volume 0 = chapters without a volume;
`backend/fontlicense.go`). One row per family: the use counts above (`styles`
by name, header/footer `bands`, `chapters` whose text has a `font` mark, so
the fonts a .docx import brought count too) plus:

- `source`, in this order: `library` (the fonts library: `license_kind`,
  `source`, `allowed` = the family's print/ebook/web/commercial flags, and
  copyright, license and designer from its files), `bundled` (the app's own
  fonts: `bundledFonts` in `backend/bundledfonts.go`, read from the generated
  `bundledfonts.json`, family in lower case → license, copyright, license URL,
  designer, line metrics), `system` (the proprietary fonts
  most computers have: `systemFonts`, mirrored from `SYSTEM_FONTS` in
  `frontend/src/editor/fonts.js`; never asked of Google), `google` (read
  from the family's TTF, name IDs 0, 13 and 14, through the same fetch as the
  line metrics, `googleTTF`; kept in `<data dir>/font-licenses.json`), else
  `unknown` (Google has no such family).
- `license_id` (`OFL-1.1`, `Apache-2.0`, `UFL-1.0` or empty), `license` (the
  words), `license_url` (http(s) only), `copyright`, `designer`, `source_url`.
- `verdict` with a `reason`: `ok` (open license, or a library font allowed for
  print and e-book; reasons `open`, `uses`), `check` (a library font with some
  uses not allowed, or whose file forbids embedding, or a license that is
  unknown or couldn't be read; reasons `uses`, `embedding`, `unknown`), `warn`
  (a system font: it may not be embedded in an e-book or PDF without a license
  from its owner; reason `proprietary`). Offline, a Google family comes back
  with source `google` and verdict `check`.

- `places`: the chapters whose text has the family's `font` mark, in book
  order, each as `{chapter, runs}` (`docFontRuns`; a run is neighbouring
  inline nodes in that family, any case). A flagged row (verdict not `ok`)
  with places shows "Show in the text" with ‹ › to step through them: the
  writing page gets `?chapter=…&font=<family>&run=<n>`, opens that chapter
  and selects its run n (`fontRuns` in `frontend/src/editor/fontPlaces.js`,
  which counts the same way; `tests/fontPlaces.test.mjs` and
  `TestDocFontRuns` share one doc).

- `spaces` and `symbols`: how much of the family's text is whitespace
  (`isFontSpace`), and its symbols (`isFontSymbol`: Unicode symbols and
  private-use characters, which old Symbol/Wingdings text uses) as
  `[{char, count}]`, most used first, at most 40.

A flagged row with places can also change the font in the text:
"Its N spaces to the body font", each symbol shown in the fonts here that
draw it (the body font, the book's fonts with an open license, the bundled
symbol fonts; `drawsGlyph` in `frontend/src/editor/glyphs.js`), a click
moving that symbol to that font, and "All of its text to the body font"
(a second click confirms). They call
`POST /api/books/{volume}/fonts/swap` with `{family, what, chars, to,
label}` (`backend/fontswap.go`): `what` is `spaces`, `chars` or `all`;
`to` empty removes the font mark (the paragraph's style decides, the body
font for Normal text), else names a family. Every chapter it changes keeps
its text first as a version named `label` ("Before changing the font
Cambria Math"), gets a new text version and is re-indexed; the answer is
`{chapters, pieces}`. The writing page saves what was typed first and reads
the open chapter again after.

"Copy credits" copies one plain-text line for the copyright page, built by
`frontend/src/editor/fontCredits.js` (pure, `tests/fontCredits.test.mjs`):
"Set in Merriweather (Copyright 2016 The Merriweather Project Authors; SIL
Open Font License 1.1) and Turret Road (…)." System fonts are left out.
Text that comes from font files is shown as plain text, never as HTML.
Screenshot: `writeShots.mjs --only fonts`.

### Font picker

The toolbar's font box is `components/write/FontPicker.vue`, not a
datalist: a datalist hides every font that doesn't match the text in the
box, and picking the font already shown changes nothing. The picker lists
every font (`fontChoices`) in sections (`fontSections` in
`editor/fontSearch.js`, pure, `tests/fontSearch.test.mjs`): while typing,
the best matches first (`fontScore`: exact, start of the name, start of a
word, every typed word, anywhere, letters in order), then the favourites
(★ on a row), the last 6 fonts used, then all fonts. A first row, "The
style's font", takes the font mark off. Enter or a click puts the font on
the whole selection (`setMark`), even the font shown, so a line in several
fonts becomes one; Esc or a click away changes nothing. With two or more
fonts selected (`fontsInSelection` in `editor/commands.js`: the font marks,
else the paragraph style's font) the box is blank, saying "Several fonts".
The bundled and library fonts are loaded when the list first opens, so
they show in their own faces; Google fonts show in theirs only once used.
Favourites and recent fonts are kept in `localStorage`
(`polaris-write-font-favourites`, `polaris-write-font-recent`). Screenshot:
`writeShots.mjs --only fontpicker`.

### Symbols

The toolbar's Ω opens the Symbols panel (`components/write/SymbolsPanel.vue`):
a font (the body font, the book's style fonts, the bundled fonts by group,
the fonts library) and a kind (`SYMBOL_BLOCKS` in `editor/glyphs.js`:
symbols, dingbats, arrows, shapes, math, runes, alchemy, letters, Greek,
Cyrillic, emoji, private use…), or "Everything in this font", the default.
The panel goes through every kind once per font (kept for the visit),
showing what it finds as it goes, each kind under its heading, with its
count in the kind list; a kind the font has nothing of shows everything
the font has instead. Only the characters the font itself draws are shown:
the browser can't list a font's characters, so `drawsGlyph` draws each on
a canvas in `"Font", <generic>` and in `<generic>` alone, on the alphabetic
baseline, and the same pixels mean a fallback drew it. A bundled font's
`unicode-range` (its stylesheet) skips what it has no face for. A click
inserts the character at the cursor (`insertSymbol` in `editor/commands.js`)
with a `font` mark for that font, or none for the body font. The preview
shows the character pointed at, in its font, and is cleared when the font or
kind changes. The font, the kind and the last 24 characters used are kept
in `localStorage` (`polaris-write-symbol-panel`). Tests:
`tests/glyphs.test.mjs`; screenshot: `writeShots.mjs --only symbols`.

### Comments (stage 5)

Notes the writer leaves on their own manuscript, like Word's comments (one
writer: no authors). Built: `backend/comments.go`, `docxcomments.go`,
`frontend/src/editor/comments.js` (pure, `tests/comments.test.mjs`),
`editor/commentPlugin.js`, `components/write/CommentsPanel.vue`.

Storage (schema 46): `chapter_comments` (`id` AUTOINCREMENT, so an id is
never reused by a newer comment; `chapter_id` cascading with the chapter;
`parent_id` for a reply, cascading with its comment; `text`; `resolved`,
`resolved_at`; `anchored`, `detached_at`; `created_at`, `updated_at`).
Story-only (`storyOnlyTables`). A comment's text is plain text (new lines
and tabs kept, at most 10 000 characters): no wiki links, no Markdown.
Replies are one level deep: a reply to a reply joins the first comment's
thread. Resolving is a thread's (a reply can't be resolved on its own).

Routes (story prefix):

- `GET /api/chapters/{id}/comments`: every comment of the chapter, by id:
  `[{ id, chapter_id, parent_id, text, resolved, resolved_at, created_at,
  updated_at }]` (`404` for an unknown chapter).
- `POST /api/chapters/{id}/comments` `{ text, parent_id? }`: the new row. A
  comment may be empty (the editor makes it, then the writer types); a
  reply may not (`400`); a parent in another chapter is `404`.
- `PUT /api/comments/{id}` `{ text?, resolved? }`: the row.
- `DELETE /api/comments/{id}`: `204`; its replies go with it.

Lifecycle. The row is made first, and only then is the mark put on the
text (the chapter's next save carries it), so a mark never points at a
comment that doesn't exist. Every write of a chapter's text (save, restore,
import: `indexChapterText` → `syncChapterComments`, in the same
transaction) compares the chapter's comments with the marks:

- in the text: `anchored`, `detached_at` cleared;
- not in the text but in one of the chapter's saved versions: kept as is,
  for a restore to bring back;
- anchored once and now in neither: `detached_at` set, and deleted at the
  first write of the text a day (`commentGrace`) later, so an undo or a
  paste back in the meantime finds it;
- never anchored (its mark was never saved, e.g. the tab closed first):
  deleted at the first write of the text a day after it was made.

A mark whose comment is gone (deleted, or a restored version's mark of a
deleted comment) draws as plain text and is dropped by the exports.
Deleting a comment in the panel deletes the row and takes its marks off
the text. Pasted text keeps only the comments of the open chapter that are
no longer in its text (a cut and paste moves a comment; a copy, or text
from another chapter, comes without). Clear formatting keeps comments.
Comments are not words and don't touch `chapter_links` / `chapter_lines`.

Editor: "New comment" in the toolbar and Ctrl+Alt+M (Word's) comment the
selection; with nothing selected, the word at the cursor (or just before
it); a selection with no text (a picture alone, an empty paragraph) gets a
message instead. While the row is being made the range is drawn as pending
and follows edits (a plugin); if its text is gone by then, the row is
deleted again. The new comment opens for writing in the "Comments" panel
(Ctrl+Enter saves, Esc on a comment never written deletes it). Each
comment's text is saved on its own call, apart from the chapter's
autosave. The panel lists the threads whose text is in the chapter, in the
order of the text, each with a quote of what it covers, its replies, Reply,
Resolve / Reopen, and a "…" menu (Edit, Resolve or Reopen, Delete);
"Show resolved" lists resolved ones too; a line counts the comments whose
text was deleted. Clicking a thread selects its text and scrolls to it;
the cursor in commented text makes the innermost comment there the
active one (scrolled into view in the panel). Highlights: a soft wash and
underline on every open comment the chapter knows, stronger for the active
one, for light and dark paper (CSS rules by id from `highlightCSS`;
resolved comments are not highlighted). The page view, PDF, EPUB and the
HTML / Markdown / text exports never show comments (the HTTP exports take
the marks off and join the text they split; the page view draws the mark
as an unstyled span).

.docx: the import reads `word/comments.xml` (text, date; author ignored)
and `commentsExtended.xml` (`done` = resolved, `paraIdParent` = a reply)
and the body's `commentRangeStart` / `commentRangeEnd` (between or inside
paragraphs, across paragraphs too); a reply's own range is ignored; a
comment with no text in its range (a point) takes the word before it in
its paragraph, else the next word. A range crossing a Heading 1 belongs to
the chapter where its text starts; its marks in later chapters are
dropped. The rows are made in the import's transaction (`anchored`).
`left_out.comments` now counts only comments that mark no text (with
their replies) and references to comments the file doesn't have. The
export writes `comments.xml` (author: the book's `meta.author`, else
"Polaris"; the date made; one paragraph per line) and
`commentsExtended.xml` (`done`, `paraIdParent`), with their relationships
and content types; `commentRangeStart` before a comment's first text and
`commentRangeEnd` plus a `commentReference` run after its last (a reply's
beside its comment's); only comments whose text is in the exported
chapters are written. Exported and imported again, comments come back with
their text, dates, replies, resolved state and ranges (a comment whose
marks had a gap comes back as one stretch).

### Spellchecker (stage 5)

Built. Everything runs in the browser; the server only keeps the story's
added words.

- Dictionaries: LibreOffice/Hunspell `en_US`, `en_GB`, `pt_BR` and
  `es_ES` from <https://github.com/LibreOffice/dictionaries> at revision
  `4f1df64b2e8e14751aa7c2205d170ca016071315`, unmodified but gzipped, in
  `frontend/public/dict/<code>/<code>.{aff,dic}.gz` with each one's readme
  and licence files (licences in `NOTICE.md`). Fetched again by
  `node frontend/scripts/dictionaries.mjs`; `DICT_REVISION` in
  `editor/spellWords.js` goes in the address (`?v=`) so a browser never
  keeps an old copy, and the files are fetched with `cache:
  'force-cache'`. Sizes (raw / gzipped): en_US 0.55 / 0.19 MB, en_GB
  1.27 / 0.45 MB, pt_BR 5.46 / 1.38 MB, es_ES 0.89 / 0.24 MB; 2.4 MB in
  the build. Only the book's dictionary is downloaded, when the writing
  page opens.
- Checker: `editor/hunspell.js`, our own reader of the Hunspell format
  (pure, no dependency): the stems stay in a Map and a word is checked
  by taking affixes off it, as Hunspell does (prefixes, suffixes, cross
  products, two suffixes, conditions, FLAG char/UTF-8/long/num, AF,
  FORBIDDENWORD, NEEDAFFIX, NOSUGGEST, KEEPCASE, ONLYINCOMPOUND, BREAK,
  ICONV; Hunspell's case rules: "the" also as "The"/"THE", "Paris" not as
  "paris"). Not read: compounding (only used for ordinals like "21st";
  words with digits aren't checked), morphology, PHONE/n-gram
  suggestions. Suggestions: the case, REP, MAP (accents), one edit
  (swap, missing, extra, wrong letter from TRY), two words, then two
  edits within 250 ms; ranked by an edit distance where accents count
  half; at most six. nspell was measured and dropped: it expands every
  form at load (en_US 133 ms / 13 MB, en_GB 0.75 s / 48 MB, es_ES 0.8 s
  / 52 MB, pt_BR did not finish in 280 s); this reader loads en_US in
  ~40 ms (+4 MB heap), en_GB ~80 ms (+11 MB), es_ES ~50 ms (+9 MB), pt_BR
  ~380 ms (+45 MB), a check takes well under a millisecond.
- It runs in a worker, `editor/spell.worker.js` (one dictionary at a
  time; `load`, `check` a batch → the wrong ones, `suggest` a word), from
  `editor/spellClient.js`, which remembers every answer per dictionary.
- Language: the book's `meta.language` picks it (`en*` → en_US, but
  en-GB/AU/NZ/IE/ZA/IN → en_GB; `pt*` → pt_BR; `es*` → es_ES); an empty
  one falls back to the interface's; any other language has no
  dictionary and the checker is off with a note. Picking a dictionary in
  the spelling panel sets the book's `meta.language` (`en-US`, `en-GB`,
  `pt-BR`, `es`).
- What is a word (`tokenize` in `editor/spellWords.js`): letters of any
  script with accents, joined by an apostrophe or hyphen between two
  letters ("don't", "Mira's", "fazê-lo"); not words: anything with a
  digit, one letter alone, web and mail addresses, words joined to
  others by a dot ("e.g.", "file.txt") or a slash/underscore, and (an
  option, on by default) words in capitals. A word followed by a full
  stop also passes with it (abbreviations).
- The story's words: every name, nickname and former name `GET
  /api/wiki` lists (`useStoryNames`), word by word (hyphenated ones also
  in parts), the story's added words and the words ignored this session.
  A word with a capital in the list passes written with capitals in any
  case ("Mira", "MIRA", not "mira"); a lower-case one in any case; a
  possessive ("Mira's", "Mira’s") passes when its word does.
- Server (`backend/spelling.go`; per story, `app_settings` key
  `spelling_words`, a JSON list; no schema change):
  - `GET /api/spelling/words`: `{ "words": [...] }`, sorted ignoring
    case.
  - `POST /api/spelling/words` `{ "add": [...], "remove": [...] }`:
    answers like GET. A word is kept as written (NFC from the browser),
    1–64 characters, a letter, no spaces or control characters; at most
    5000 words (`400` otherwise).
- Underlines (`editor/spellPlugin.js`): inline decorations
  (`span.spell-wrong`, a wavy line in `--spell-line`, lighter on dark
  paper). Only textblocks on screen (and a screen above and below) are
  checked, after a 350 ms pause and on scroll; what a textblock gave is
  remembered by its node, so typing rechecks one paragraph. The word the
  caret is in, while typing, isn't underlined until the caret leaves it.
  Linked text (`story_link`, `link`) isn't checked; a dialogue's line is,
  its speaker and manner (attrs) aren't.
- Menu (`components/write/SpellMenu.vue`, in `<body>`): right click on an
  underlined word (on a touch screen, a tap on the word the caret is
  already in): up to six suggestions (one replaces the word, keeping its
  formatting), "Add to dictionary" (the story's), "Ignore" (every
  occurrence until the page is left; not stored), "Spelling options…".
- Switch and panel: the toolbar's spelling button turns it on or off
  (remembered in this browser, `polaris-write-spell`; folded behind "⋯" on
  phones); while on with a dictionary the editor has `spellcheck="false"`,
  otherwise the browser's own is back. The status bar shows "Spelling:
  English (US)" (loading, off, no dictionary) and opens the spelling side
  panel: on/off, the dictionary, "Ignore words in capitals"
  (`polaris-write-spell-caps`), the story's dictionary (add, list,
  remove) and the words ignored for now.
- Tests: `tests/spellWords.test.mjs` (words, names, language),
  `tests/spellcheck.test.mjs` (the reader on a small dictionary written
  in the test, then the four bundled ones), `tests/editorSpell.test.mjs`
  (what text is checked, the caret rule, replacing a word),
  `TestSpellingWords*` in Go; screenshots with `node
  frontend/tests/visual/writeShots.mjs --only spell`.
- Not done: suggestions for words the dictionary only knows through
  compounding; no Portuguese (Portugal) or regional Spanish
  dictionaries (pt-PT uses pt_BR, every Spanish uses es_ES); a word
  added to the story's dictionary doesn't take affixes (add "Zarathul's"
  too, or rely on the possessive rule); the tap-to-open on phones was not
  tried on a real phone.

### Tables (stage 5)

Built (no schema change: tables live in the doc JSON). For LitRPG stat
blocks, status screens and item lists; Word's tables in what matters for
that.

- The nodes (above) are defined twice: `frontend/src/editor/tableSchema.js`
  and `backend/pmtable.go` (validation: at most 63 columns and 1000 rows,
  every row as wide as the grid, no overlapping cells, a span never past
  the last row, sizes in range). Inside the editor widths are CSS px (what
  prosemirror-tables' column resizing writes); `docFromJSON` turns the
  stored pt into px and `docToJSON` back (the HTML's `data-*` widths are
  pt).
- Drawn the same in the editor, the page view, the EPUB and the .docx
  export: single 0.5 pt lines (`all`: every line; `outer`: the edges;
  `none`), Word's 5.4 pt cell margins left and right, fixed layout, each
  column its share of the columns' total (a column without a width counts
  as the others' average) and the table as wide as its `width` but never
  wider than the text, so a table made for a wider page fits.
  Normal paragraphs in cells (no `style`) have no first-line indent and no
  space before or after unless they set their own (`styleSheet`'s
  `:where(td, th) > p:not([data-style])`, as for list items; the .docx
  export writes those 0s out, the import leaves 0s out). Header cells look
  like other cells (bold is the text's own); the editor tints them and
  shows the lines a table doesn't draw as faint dashes.
- Editor (`editor/tables.js`, `components/write/TableOptions.vue`): the
  toolbar's table button opens an 8 × 8 grid to pick rows × columns (folds
  behind "⋯" on phones; disabled in a table: tables don't nest); the table
  goes where the cursor is (the paragraph split there, after the list the
  cursor is in, in place of an empty paragraph) with the cursor in its
  first cell. Tab and Shift+Tab go to the next and previous cell
  (selecting its text, as Word does); Tab in the last cell adds a row; at
  the start of a list item in a cell Tab and Shift+Tab indent and outdent
  the item instead. prosemirror-tables gives cell selection (drag or
  Shift+arrows), the arrows between cells, column resizing (drag a cell's
  right edge) and pasting cells into a table. While the cursor is in a
  table a small button sits at its top right corner (in the page's margin
  when there is room, kept in view); it opens the table box: rows (above,
  below, delete), columns (left, right, delete), merge and split cells,
  header row on/off, cell shading (light swatches, any colour, none),
  vertical alignment, the table's alignment, width (fit the columns / full
  width), borders and Delete table. The box closes on a click elsewhere or
  Esc. Tab-to-link in a cell: a name still to complete takes Tab as
  everywhere; a name typed in full (the "⇥" hint) isn't offered, so Tab
  moves on. Script Mode is unaffected (dialogue only at the top level).
- Pasting: tables from Word, Google Docs and web pages keep their cells,
  colspan/rowspan, widths in pt or px (style or `width`), shading (white
  is none; a cell's shading isn't also read as a highlight on its text),
  vertical alignment, alignment and a 95–100% width as `full`; borders:
  cells drawing a line mean `all`, cells saying none mean `none`, nothing
  said means `all`. prosemirror-tables fixes ragged tables.
- Page view (`editor/pageView.js`): the header rows go in a `thead`;
  paged.js 0.4.3 doesn't repeat it, nor keep a split table's `colgroup`,
  so a small paged.js handler (`repeatingTables`) puts both back on each
  page a table continues on. Rows don't break across pages
  (`break-inside: avoid`); a row taller than a page is still cut by
  paged.js. Header rows the page broke inside aren't repeated, and header
  rows can be left alone at a page's foot (no keep-with-next). The Online
  Post reading view draws tables the same, without pages.
- .docx import (`backend/docxtables.go`): grid widths, gridSpan, vMerge,
  gridBefore/gridAfter and short rows (filled with empty cells),
  tblHeader rows at the top, jc, a 100% tblW as `full`, borders from
  tblBorders set directly or by the table style chain (inside lines: all,
  edges only: outer, else none; a table whose cells mostly draw their own
  tcBorders, as Google Docs writes them, is all), shd fill, vAlign; the
  table style's paragraph and run properties apply to its text between the
  document defaults and the paragraph's style, as in Word. Headings in a
  cell are paragraphs; pictures in a cell go right after the table; page
  breaks in cells are dropped; a table nested in a cell keeps its text as
  paragraphs in that cell (`left_out.tables`); a table over 1000 rows goes
  on as a second table.
- .docx export: a w:tbl with the same, `cantSplit` rows, fixed layout,
  `tblHeader` on the header rows. Round trip (`TestExportDocxRoundTrip`):
  a table comes back the same but for: a table without column widths
  (comes back with equal ones over the text's width, which draws the
  same), widths wider than the text (scaled to it), header cells outside
  the header rows (plain cells), explicit 0 indents and spacing on Normal
  cell paragraphs (left out, the same look), story links (plain text).
- EPUB: an XHTML table (classes `tb tb-<borders>`, colgroup in %, width a
  share of the text's, thead for the header rows, shading and vertical
  alignment inline). HTML for AO3: table, thead, tbody, tr, th, td with
  colspan/rowspan only, a one-paragraph cell as its text. Markdown: a
  GitHub pipe table when no cell spans and every cell is one paragraph
  (its first row is the header row; hard breaks as `<br>`; `|` escaped),
  else the HTML table. Text: columns lined up with spaces, two between
  columns, a cell's lines one under another, a line of dashes under the
  header rows.
- Word counts (Go and JS) count the cells' words; "Appears in" counts
  story links in cells (`chapterindex.go` walks the whole doc).
- Screenshots: `node frontend/tests/visual/writeShots.mjs --only tables`
  (a stat block and a long inventory, the table box, Tab, a paste from
  Word, dark paper, the page view with the repeated header row).
- Not done: per-cell borders (the import reads them only to choose the
  table's setting), row heights, cell margins other than Word's default,
  text direction, header columns in exports (they are plain cells there),
  tables inside lists, and a table's caption.

### Footnotes and endnotes (stage 5)

Built (no schema change: notes live in the doc JSON).

- Decisions: numbers are per chapter and never stored (footnotes 1, 2, 3;
  endnotes i, ii, iii, in reading order); notes count as words and feed
  "Appears in" (`chapterindex.go` walks the whole doc); find and read
  aloud skip notes; a note has the browser's own spellcheck (the story
  spellchecker doesn't reach into it); no comments and no pictures inside
  a note, and no note inside a note; note text is 5/6 of the body size on
  the half point (`noteSize` in `css.js`, `noteSizeOf` in
  `exportnotes.go`).
- The node: inline atom `footnote`, attr `kind` (`footnote` or
  `endnote`), content paragraphs. Defined twice, `editor/schema.js` and
  `backend/pmdoc.go` (`footnote` validation rejects a nested note and
  anything but paragraphs). `docToJSON` used to drop marks on non-text
  nodes; it keeps them now.
- Editor: toolbar buttons and Ctrl+Alt+F (footnote) / Ctrl+Alt+D
  (endnote) (`footnotes.js`, `noteView.js`). The note shows as its number;
  clicking it (or inserting one) opens a small pop-up with a nested
  editor, which can turn the note into the other kind, delete it, and
  return to the text with Esc.
- Page view: footnotes are floated to the page's foot by paged.js (a short
  rule above, the number in the text and in the note); endnotes are
  gathered at the chapter's end. An online post gathers every note at the
  end (`pageView.js`, `bookLayout.js` `notesCSS`).
- Exports (`exportnotes.go`): .docx writes real Word footnotes and
  endnotes (`word/footnotes.xml`, `endnotes.xml`, numbers restarting in
  each chapter's section); EPUB writes noteref links, footnote asides and
  an endnotes section; HTML, Markdown and text put the numbers in the text
  and the notes at the chapter's end.
- Import (`docxnotes.go`): Word's notes come back as `footnote` nodes with
  their paragraphs; `left_out.footnotes` / `endnotes` now count only
  references to notes the file lacks and notes inside notes. A picture or
  comment in a note is left out. `TestExportDocxRoundTripNotes` keeps the
  round trip.
- Screenshots: `node frontend/tests/visual/writeShots.mjs --only footnotes`
  (a note open in the editor, the page view with the footnote at the
  page's foot).
- Not done / not checked: not opened in Word or LibreOffice; a note longer
  than a page isn't split across pages (paged.js's limit); custom number
  formats or restarting per page; cross-references to a note; notes in
  tables' cells and in headings are not handled specially.

### Boxes and drop caps (stage 5)

Built (no schema change: paragraph attrs in the doc JSON).

- Attrs, on paragraphs and headings: `boxWidth` (pt, a border when > 0),
  `boxColor` (`#rrggbb`, black when missing), `shade` (`#rrggbb`, a
  background, with or without a border), `dropCap` (lines, 2 to 4; none
  when missing). They are the paragraph's own, not a style's: a style
  cannot carry a box (left out on purpose; "Clear formatting" and applying
  a style take them off). The checks are in `pmdoc.go`, the copies in
  `schema.js`.
- Editor: the "Box, shading and drop cap" button in the paragraph controls
  (border width, colour, shading, drop cap lines). The box is inline CSS
  (`border`, `background-color`, 4pt 6pt padding); `data-box` and
  `data-dropcap` on the paragraph let `editor.css` do the rest. Consecutive
  boxed paragraphs draw as one box (`[data-box]:has(+ [data-box])`: no
  border or gap between). They join whatever their colours (known gap).
  The drop cap is `::first-letter` with `initial-letter` and a float
  fallback; Enter does not copy a drop cap to the new paragraph.
- Page view: the same stylesheet (the schema's DOM), nothing extra.
- Import (.docx): `w:pBdr` (any side counts: one box, the widest side's
  width and colour), `w:shd` fill (white is none), `w:framePr` with
  `w:dropCap` (Word's drop cap is its own paragraph holding the letter: it
  is joined to the next paragraph's text and sets `dropCap`; `w:lines`,
  3 when missing). Direct formatting only: a style's border or shading is
  not imported. `left_out.borders_shading` and `drop_caps` are gone.
- Export: .docx writes `w:pBdr` (four sides, `w:sz` in eighths of a point)
  and `w:shd`, and a drop cap as Word does: a framed paragraph with the
  letter (`w:framePr w:dropCap="drop"`) before the paragraph without it
  (only when the paragraph starts with text; the letter loses its marks).
  EPUB writes the border and shading as inline CSS and classes `boxed`,
  `dropcapN` (`initial-letter` with a float fallback); there the first of
  two boxed paragraphs keeps its bottom border (no `:has`). The Online
  Post HTML writes border and shading inline. Markdown and text ignore both.
- Tests: `frontend/tests/editorBoxes.test.mjs`, `boxes_test.go` (validation,
  .docx round trip), `writeShots.mjs --only boxes`.
- Known gaps: not opened in Word or LibreOffice; joined boxes of different
  colours; rounded corners, per-side borders and a drop cap of more than
  one letter are not supported; a box that runs over a page break is cut.

### Tagging speakers in prose (stage 5)

A line of dialogue in a normal chapter can say who spoke it without
Script Mode. Select the text (a quote inside a paragraph, or the whole
paragraph), right-click and choose "Tag character…" (or press
Ctrl/Cmd+Alt+K), then pick a character: type to filter, ↑/↓ and Enter pick,
or use a typed name that is not a character. The selection is trimmed of
surrounding spaces and gets the `speaker` mark. Right-clicking inside a
tagged text offers "Change character…" and "Remove tag". A selection's
menu wins over the spelling menu; a misspelled word with nothing selected
keeps the spelling menu. (Deviation: the menus are not merged into one.)

Tagged text is underlined with a thin dotted accent line, in the writing
page only. Hovering it (after about a third of a second) shows a small
card (`components/write/TagCard.vue`, never takes the focus or the mouse):
the character's wiki card (picture, name, a few facts, the start of its
text: the same `ArticleCard` the wiki's link hover uses), or just the name
for a speaker who isn't a character or whose article can't be loaded. The chapter index
(`indexChapterText`) records each tagged run as a `chapter_lines` row like a
Script Mode dialogue (consecutive text nodes with the same tag are one
line, in reading order among the chapter's dialogues), and the character
counts as named in the chapter, so a character's Lines and "Who speaks" show
them. The page view, .docx, EPUB, HTML, Markdown and text exports and read
aloud, find, spellcheck and word count see plain text: exports and the .docx
emitter look marks up by name and ignore this one; a Word import never makes
it. Code: `editor/speaker.js`, `components/write/SpeakerTag.vue`,
`markedLines` in `backend/chapterindex.go`.

**Tag spell.** The same menu (right-click on selected text, or inside an
existing tag) also offers "Tag spell…" / "Change spell…" and "Remove spell
tag": pick one of the story's spells (type to filter) and the text gets the
`spell` mark (`tagSpell`, `spellAt`, `removeSpell` in `editor/speaker.js`;
one contextmenu handler serves both tags, and `onMenu` gets `tag` and
`spell`). Drawn with a thin dashed teal underline, so it reads apart from
the speaker's dots. Its hover card shows the spell's name and icon, its
cost ("12 MP", "5 HP", both, or "Free"), its class ("Class: Mage", also
subclass or specialization) and its level (`editor/spellTag.js`, from
`GET /api/spells`, fetched once per page and, for an offline book, kept
with it). The index counts each run of a spell mark as one mention of that
spell (`docLinkCounts`), so the spell's "Appears in" lists the chapter. Exports,
read aloud, find and word count see plain text; a Word import never makes
the mark.

**Show tags.** A toolbar button (tag icon, "Show tags") hides or shows the
underlines of both kinds of tag; hidden, there is no hover card either. The
tags themselves stay in the text and the right-click menu still works. The
choice is remembered per browser (`localStorage` key `polaris-write-tags`).

### Pictures with text wrapping (stage 5)

Built (no schema change: an image attr in the doc JSON).

- Attr: `wrap` on `image`: `inline` (default; a doc without it is
  unchanged), `left` or `right` (the picture floats to that side, text
  flows around it). The width attr is kept; a wrapped picture without a
  width takes 40% of the text, and never more than 60%. `align` is not
  used while it floats (the import sets both alike). `fullPage` wins over
  `wrap`. The gap to the text is fixed: 8pt. Checks in `pmdoc.go`, the
  copy in `schema.js` (`data-wrap` on the figure).
- Editor: "Text wrapping" buttons (In line, Text on the right, Text on the
  left) in the picture's options box, under the width. A floated picture
  is a block placed before the paragraph that should wrap it; the text of
  the next paragraphs flows beside it. Floats are cleared before headings,
  before a picture that is not wrapped and at the end of the text
  (`editor.css`).
- Page view: the same CSS (the chapter is drawn with `write-doc`), and the
  chapter clears its floats at its end. A floated picture near the foot of
  a page may be pushed whole to the next page by paged.js (accepted).
- Export: .docx writes a `wp:anchor` with `wp:wrapSquare
  wrapText="bothSides"`, `wp:positionH relativeFrom="column"` aligned
  left or right, 8pt of distance on the text's side, in a hair-thin
  (exact 1pt) paragraph of its own so no empty line shows; EPUB and HTML
  a `figure.image` with `float`, its width in % and a margin as inline CSS
  (plus a class `wrap-left` / `wrap-right`); Markdown and text as before.
- Import (.docx): a `wp:anchor` picture with `wrapSquare`, `wrapTight` or
  `wrapThrough` and a left or right place (Word's `wp:align`, or by its
  offset) becomes `wrap` left or right; `wrapTopAndBottom`, no wrapping,
  or a centred picture stays a picture on its own line (as before). None
  of this is counted in `left_out`. The picture goes right after the
  paragraph it was anchored in, so the text that wraps it is the next
  paragraph, not the anchor paragraph's own.
- Tests: `frontend/tests/editorWrap.test.mjs`, `backend/wrap_test.go` (the
  validation, the .docx XML and a round trip with a left and a right
  picture); screenshots:
  `node frontend/tests/visual/writeShots.mjs --only wrap`.
- Known gaps: not opened in Word or LibreOffice (only our own round trip);
  no wrapping inside lists or tables; tight and through wrapping are read
  as square; a picture floated at a chapter's end leaves its own space
  below it; it cannot be dragged to a side, only set in its options.

### Offline writing (stage 5)

A book marked "Available offline" (`OfflinePanel.vue`, `editor/useOfflineBook.js`)
can be opened and written with no connection; the writing page shows
`OfflineBanner.vue` while it is offline and what is waiting to upload.

- Service worker (`frontend/src/offline/sw.js`, filled in at build by a Vite
  plugin, `swBuild.js`): the app's files are kept in a versioned cache
  (`polaris-app-<BUILD>`, filled at install, older ones deleted on
  activate). Pages are network-first with the kept page as fallback. `/api/`
  is never cached or touched: what offline books need is stored by the page.
  Uploads are network-first; copies the writing page kept for offline books
  are served from their own caches with their original headers. Spelling
  dictionaries and Google Fonts (CSS and files) are kept once fetched.
- IndexedDB (`polaris-offline`, `offline/kv.js`): per story, the stories,
  books (settings, styles, page types) and chapters (doc, version, words,
  comments, a `dirty` flag for text written offline).
- Sync (`offline/sync.js`): on reconnect each dirty chapter is saved with the
  version it was based on. On a `409`, if the two sides edited different
  top-level blocks they are merged automatically (`offline/merge.js`, a
  three-way merge) and your text is saved as a version first; if the same
  block changed, the usual conflict view shows both texts.
- The page asks the browser for persistent storage when a book is made
  available offline, and shows whether it was granted.
- Needs HTTPS or localhost (a secure context); otherwise the option is hidden.
- Tests: `offlineMerge`, `offlineStore`, `serviceWorker`;
  screenshots: `node frontend/tests/visual/offlineShots.mjs`.
- Known gaps: the page view needs the connection; comments are read-only
  offline; pictures cannot be added offline; two tabs writing the same book
  offline means the last one wins; not tried on a real phone or a real HTTPS
  install.

## Format Painter

A brush button in the toolbar (`src/editor/formatPainter.js`, wired in
`WritePage.vue`). Click it: it copies the formatting at the caret or the
selection's start, and the page shows a copy cursor. The next selection made
with the mouse gets it, then the painter turns off. Double-click the button
to keep it on; Esc or the button again turns it off.

- Copied: the formatting marks (`FORMAT_MARKS`: character style, font, size,
  colour, highlight, bold, italic, underline, strike, super/subscript, small
  caps) and the paragraph's attrs (`PARA_ATTRS`, with a heading's level).
  Links, story links, comments and speaker tags are never copied nor
  removed.
- Painted: on a selection, the formatting marks of the range are replaced by
  the copied ones. Paragraph attrs are painted on the paragraphs the
  selection covers whole, or on every paragraph it touches when the copied
  text was a whole paragraph (as in Word). A plain click (no selection)
  paints the clicked paragraph only. A heading source makes a heading.
- No keyboard shortcut: Ctrl+Shift+C opens the browser's developer tools and
  Ctrl+Shift+V is the browser's "paste as plain text".
- Test: `node frontend/tests/editorFormatPainter.test.mjs`.

## Page breaks while writing

With a page-based page type (anything but Online Post) the editor draws the
chapter as separate pages, the way Word's Print Layout does, so a writer can
plan paragraphs around the natural page breaks. The document is never
changed: the gaps are widget decorations (`src/editor/pagination.js`, a
plugin added in `WritePage.vue`), and the page view (paged.js) stays the
final word.

- **Switch:** "Show pages while writing" in the Book and page panel (the
  status bar's "Page: …" button). On by default, remembered per browser
  (`localStorage` `polaris-write-pages`); no status-bar button. Online Post
  never shows pages. Narrow screens (`compact`) still break at the real
  page height, only the margins stay small.
- **Page height:** the page's height less its top and bottom margins
  (`page.height_mm`, `margin_top_mm`, `margin_bottom_mm`; mirrored margins
  only change the sides, so they don't matter here). The sheet is zoomed with
  CSS `zoom`; everything is measured in px as if it weren't (the scale is the
  doc's measured width over the text block's known width).
- **Measuring:** per text block the browser's own line boxes (Range rects:
  nothing about line height is assumed, so it follows whatever the stylesheet
  does); a picture, table, table of contents, scene break or page break is
  one unit (a table is never split between rows here; a picture with text
  wrapping takes no room of its own). One pass reads the layout, then
  writes; it runs after a pause (about 140 ms) after a change, a resize, a
  zoom, a settings or styles change, or fonts loading, never on a keystroke,
  and repeats (at most five times) until the answer stops changing. A
  10,000-word chapter takes tens of milliseconds a pass.
- **Rules (`src/editor/pageBreaks.js`, pure, `pageBreaks.test.mjs`):** input
  is the lines in order (`pos, top, bottom, block, first, last, keepNext,
  hard, unit, spaceBefore`) and the content height; the output is the line
  each new page starts at. A line fits when its bottom is within the content
  height. Widow/orphan control is always on: never one line of a paragraph
  alone at a page's foot or head, and a paragraph of 2-3 lines moves whole.
  `keepNext` (the paragraph's own or its style's: the headings) keeps a block
  on the page of the next one unless that would empty the page. A
  `page_break` node starts a new page with the next block, and takes no
  room. A paragraph's space before is dropped at the top of a page after a
  natural break and kept after a hard one. A unit that doesn't fit starts a
  page; a unit taller than a page overflows its own page.
- **No oscillation:** positions are measured as if no gap were there. Each
  drawn gap knows how much it moved what follows (`shift`) and that is taken
  off again, so measuring after a gap was drawn gives the same answer.
- **The gap** is one widget as wide as the sheet (a negative left margin
  brings it out over the margins): the rest of the page (paper), its foot
  (the bottom margin, with the footer), a dark space (`--page-gap`), and the
  next page's head (the top margin, with the header). Inside a paragraph it
  is a float just before the first character of the page's first line (a
  float as wide as the sheet, so the text starts a fresh line below it: the
  line before is laid out as before, a first-line indent still applies to a
  paragraph that begins a page, and a list marker stays with its text); in
  front of a picture, table or scene break it is a block. It is
  `contenteditable=false`, ignored by the selection, and lets clicks
  through to the text (`pointer-events: none`). The header and footer use the
  book's layout (`bookLayout.js`, `bandText`); the page number counts pages
  of this chapter (from the chapter's restart number), not of the whole book.
  The last page's foot hangs in the sheet's bottom margin.
- **Known differences from the page view:** footnotes take no room at the
  page foot here (the page view places them); page 1 has no header drawn
  (the sheet's top margin is empty padding); a table is not split between
  rows; the page numbers are the chapter's. Where the page view breaks a
  paragraph differently (its own widow/orphan handling) the two can disagree
  by a line or a paragraph until the next hard break, picture or table
  resyncs them.
- Screenshots and checks: `node frontend/tests/visual/writeShots.mjs --only
  pages` (the gaps, equal breaks after measuring again and after typing a
  character and deleting it, a hard break, widows and orphans, every page
  as tall as the page type, the switch, Online Post, a long chapter, and the
  first words of each page against the page view's).
