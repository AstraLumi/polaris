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
- `char_style` {`id`}: the text uses a character style of the book. Order:
  the text's own marks > its character style (and that style's
  `based_on`) > its paragraph's style.
- `comment` {`id`} (stage 5): the text is commented on; `id` is the row of
  the chapter's comment (a positive integer). A text may carry several
  `comment` marks with different ids (overlapping comments; never the same
  id twice). Not inclusive: typing at its end doesn't extend it. It never
  shows outside the writing page (see "Comments (stage 5)").

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
  dropped; other fields keep their shown text; tables and text boxes keep
  their text as paragraphs; a nested list level (Word's `ilvl`, up to nine
  levels) is a list inside the item before it; a numbered heading gets its number in its text. A paragraph of separator
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
separates words like a space. A `dialogue` counts its spoken line only,
not its speaker or manner (they are attrs, as a script's cue isn't text).

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
    stable `urn:uuid`); no fonts embedded until the fonts library knows
    their licenses (stage 5).
  - `/api/books/{volume}/export.html`, `.md`, `.txt`: Online Post exports.
    HTML is clean for AO3: p, em, strong, u, s, sup, sub, h1–h3, br, ul/
    ol/li, blockquote for paragraphs in the Quote style, a for web links;
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
  Heading 1; applying the layout marks those chapters as front matter.
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
  chapter); a front-matter chapter's number shows as "·".
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
- Book panel: the meta fields, the header and footer bands with a field
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

Status (2026-10-27): stages 1–4 are built and pushed (`126fdc3` the MVP,
`12df0da` stages 3–4; schemas 42–45). Nothing of stage 5 exists yet.
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

Stage 5, suggested order (each its own piece of work):

1. **Fonts library with a license tracker.** One library for all stories
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
2. **Spellchecker that knows the story.** LibreOffice/Hunspell
   dictionaries (en, pt-BR, es to match the interface), checked in a
   worker; every character, place, faction, creature, asset name,
   nickname and former name counts as a word; a per-story "add to
   dictionary". Underlines through a ProseMirror decoration plugin, with
   suggestions on right click. The browser's own spellcheck is turned off
   in the editor once this is on.
3. **Comments.** Built (schema 46): see "Comments (stage 5)" below.
4. **Footnotes and endnotes.** An inline atom holding its own content;
   numbered automatically; page view puts footnotes at the page's foot
   (paged.js float), exports write Word footnotes and EPUB endnotes;
   the import stops dropping them (`left_out.footnotes`).
5. **Tables.** prosemirror-tables; for stat blocks in LitRPG; import,
   export, page view.
6. **Boxes and drop caps.** Paragraph borders and shading (LitRPG system
   windows), and a drop cap on a chapter's first paragraph; both as
   paragraph attrs, imported from Word (`left_out.borders_shading`,
   `drop_caps`) and exported.
7. **Pictures with text wrapping.** Float left/right with wrap, in the
   editor, page view and exports.
8. **Read aloud.** The browser's speech synthesis reading the selection or
   from the caret, with the current sentence highlighted.
9. **Offline writing.** A service worker caching the app's files and a
   manifest so it installs as an app; books marked "Available offline"
   kept in IndexedDB (chapters, styles, settings, fonts) and edited there
   with no connection; on reconnect, changes upload with the existing
   version check, and a conflict shows both texts as today (ProseMirror
   can merge edits to different paragraphs). Needs HTTPS (or localhost).
   Ask the browser for persistent storage.

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
