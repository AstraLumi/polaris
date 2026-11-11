# Changelog

What changed in each version of Polaris. The same notes are on the
[Releases](https://github.com/AstraLumi/polaris/releases) page. Versions are
Major.Minor.Bugfix: new features raise Minor, fixes raise Bugfix.

## 3.2.0 (2026-11-11)

**Upgrade:** stories are upgraded to database schema 47 the first time they are opened. This can't be undone, so export your stories first. Polaris also saves an export of each story in its `backups` folder before upgrading it.

**Before you update, if you reach Polaris by a domain name** (`polaris.example.com`): add `ALLOWED_HOSTS=polaris.example.com` to the `.env` file beside `docker-compose.yml`. Without it Polaris answers that name with a page telling you to. Nothing is needed if you open it as `localhost`, by an address in numbers or by a name without a dot.

**If you keep a `docker-compose.yml` of your own** with `cap_drop: ALL`: add `cap_add: [CHOWN, SETUID, SETGID]`. Without it Polaris keeps running as root, as before, and says so in its log.

### Changed

- **Polaris only answers to names that are surely yours.** A request addressed to a domain that isn't listed in `ALLOWED_HOSTS` is refused, with a page saying which line to write. This closes a way for another website to read and change your stories through your own browser, by pointing a name of its own at your machine. `localhost`, addresses in numbers (`192.168.1.20`), names without a dot (`nas`) and names ending in `.local`, `.home.arpa` or `.internal` always work. `ALLOWED_HOSTS=*` turns the check off. In 3.1 this check existed but was off unless you set it.
- **Polaris no longer runs as root inside its container.** It runs as an ordinary user; the container starts as root only to hand your two data volumes to that user, the first time, by itself. Stories kept by earlier versions need nothing from you.
- **Long stories are much faster.** On a test story far larger than anyone has written (3,000 characters, 10,000 events, 1,000 chapters), the character list went from 47 seconds to under 0.2, the events list and the timeline from 3 seconds to about 0.2, and the chapter list, a chapter's page, "Appears in" and a character's Lines from 1.4 seconds to 0.02. Stories of ordinary size were already quick and gain a little.
- The server's log now starts by saying which names it answers to.

## 3.1.0 (2026-11-11)

No database change: stories open as they are.

Polaris 3.1 puts writing first, protects your stories better, and loads much faster.

### Added

**Writing**

- **"Write" in the sidebar** opens the chapter you wrote last, and Home has a "Continue writing" card, a count of the words written and a writing step in "Where to start".
- **A copy button** in the writing toolbar on touch screens (the page on screen or the whole chapter). Plain-text copies keep Script Mode speaker names.
- **Keep with next, Keep lines together and Page break before** as toolbar buttons and Styles panel settings, on script lines too, followed in the page view and in .docx and EPUB exports.
- **"New character" in a script line's speaker box**: type a name and the character is created and gets the line.

**Safety**

- **Daily copies.** Polaris keeps a copy of each story on the days it changes: the last 7, then one a week for 4 weeks. "Earlier copies…" in a story's menu opens one as a new story beside the present one; nothing is overwritten. They hold the text and data, not the pictures, and live on the same disk, so exporting is still the backup to keep elsewhere. `AUTO_BACKUPS=off` turns them off.
- **Undo for deletions.** Deleting a chapter, a character version, a relation, an asset or a gallery picture hides it at once and offers "Undo" for a few seconds, instead of asking "are you sure?". Deleting several events, characters or assets at once offers the same.
- **Requests from other websites that would change anything are refused**, and an optional `ALLOWED_HOSTS` setting names the addresses Polaris answers to (see the README). Uploads have size limits (32 MB per picture form, 256 MB for a gallery), and pages are served with a Content-Security-Policy.

**Interface**

- **A light theme, Daylight**, and "Follow this device", which picks light or dark with your system.
- **Keyboard access to the drawn views.** The wiki graph (arrow keys step from article to article, Enter opens one, + and − zoom, 0 shows everything), the map (arrows, + and −, 0) and the solar system editor (Tab to a body, Enter picks it, Shift+Enter shows its moons). Stat rows, zoomable pictures and tag chips work from the keyboard too.
- **"Try again"** beside a page that failed to load, a thin progress line while a page is on its way, and placeholders in place of "Loading…".

### Changed

- **Much faster first load.** Pages and translations are fetched when first needed and sent compressed: the main script went from about 1,054 KB to 85 KB. A tab left open across an update reloads itself instead of breaking.
- **The story picker no longer opens every story**, so each story is upgraded and backed up when you open it, not all at once. Stories nobody has used for half an hour are closed and give their memory back.
- **The writing toolbar shows the everyday tools**, with the rest behind "More tools" (remembered). On phones it is one row that scrolls sideways, and the side panels open as a sheet from the bottom.
- **The sidebar folds to icons by itself** on tablet-sized windows, and on the writing page when the window is too narrow for both.
- All "are you sure?" questions use Polaris's own dialog instead of the browser's box. Buttons are larger on touch screens, and error messages are announced by screen readers.
- With no connection, errors say "can't reach the server", and a page that gets no answer gives up after a minute instead of loading for ever.
- `docker-compose.yml` now runs the container read-only and with no extra capabilities, and Polaris finishes what it is saving when it is stopped. If you keep your own compose file, you may want to copy those lines.

### Fixed

- Two saves at the same moment could fail with "database is locked"; they now wait for each other.
- A story whose database Polaris could not upgrade was emptied and made anew. It is now refused and left untouched.
- An upgrade no longer starts when its backup can't be saved.
- An export that failed half way downloaded as a broken file; it now fails with a message.
- A list the server could only read in part was shown as if it were whole; the page now fails and can be tried again.
- Text typed while a save was on its way could be lost when the save met a conflict. A save the server refuses now reads "Couldn't save: kept in this browser" with the reason.
- Escape did not close pop-up forms (since 2.0.0).
- Temporary files of large uploads were never deleted.

## 3.0.1 (2026-11-01)

### Changed

- **The wiki's graph opens much faster.** Working out the connections took over 11 seconds on a small server for a story with about a hundred articles on a planet map; it now takes under a second, and usually none at all: the graph is kept ready in the background and rebuilt a few seconds after you change something, as long as you've opened it in the last half hour. It is kept compressed and forgotten after half an hour without a look, so it costs almost no memory.
- Big graphs (a thousand articles and more) lay themselves out several times faster in the browser and show within a quarter of a second, then finish settling on screen.

## 3.0.0 (2026-10-30)

**Upgrade:** stories are upgraded to database schema 46 the first time they are opened. This can't be undone, so export your stories first. Polaris now also saves an export of each story in its `backups` folder before upgrading it.

Polaris 3.0 adds three big parts: a writing editor for the manuscript itself, a 3D planet with a climate worked out from your map, and a solar system with its sky.

### Added

**Writing**

- **A writing editor** for each book (Chapters → Write): styles, fonts, find and replace, undo, lists, tables, pictures with text wrapping, drop caps, boxes and shading (consecutive boxes form one LitRPG "system window"), a Format Painter, and a Symbols panel to browse a font's characters.
- **Page types**: A4, A5, A6, a custom trim size, or an online post. Page-based books show real pages while you write, breaking like Word (widows and orphans, keep with next, page breaks), with line spacing that follows each font.
- **A page view** with headers, footers, page numbers (roman for front matter, restarts), a table of contents with page numbers, footnotes at the foot of the page, and print to PDF.
- **Word files in and out.** Upload a `.docx` and it becomes chapters, keeping formatting, pictures, tables, comments, footnotes, headers, footers and page setup; the text before the first Heading 1 becomes front matter. Export the book or a chapter as `.docx`, EPUB, HTML, Markdown or plain text.
- **Comments** with replies, **footnotes and endnotes**, and **read aloud** with the sentence highlighted.
- **A spellchecker that knows your story's names** (English US and UK, Portuguese, Spanish), with a dictionary for each story.
- **Tag who speaks** a line of dialogue (Script Mode, or Ctrl+Alt+K on prose) and **tag spells** being cast. Hovering a tag shows a card. Characters get a Lines tab, and every wiki article lists the chapters it appears in.
- **Saving you can trust**: autosave, a copy kept in the browser, a warning when the same chapter was changed in another tab, versions every 30 minutes and on demand, and **offline writing**: Polaris installs as an app, and books marked "Available offline" can be written with no connection. Changes upload when you reconnect, and edits to different paragraphs merge on their own.
- **Fonts**: 26 open-licensed fonts come with the app and work offline (book serifs, "system window" faces, handwriting and sound effects, Japanese, Noto symbols), and Merriweather is the new default. A **fonts library** takes your own font files, with a record of each one's license and allowed uses. Each book lists its fonts and their licenses, flags fonts you can't embed in an e-book, can move a flagged font's spaces or symbols to a font you can use, and copies the credits for your copyright page. The toolbar's font picker has favourites and recent fonts.
- Adding a volume offers to copy the previous volume's book settings.

**World**

- **A 3D planet.** In Settings → Map, choose a 3D planet instead of a flat map: a globe of hexes you paint like the 2D map, plus flat views (equirectangular, Mercator, Mollweide, Robinson). An existing 2D map can be carried onto it. A bucket tool fills empty areas, and names sit on the land and the water.
- **Map layers** on the planet: tectonic plates, continents with a biome per hex (23 biomes), terrain (hills, plateaus, mountains, with a height), and ocean depths.
- **A climate worked out from your map**, never typed in: winds, ocean currents, temperature, rain, clouds and climate types, for each month or the whole year. High ground steers the winds, lee slopes warm and dry, and volcanoes give off heat. Region articles show a climate table, and Settings → Calendar has seasons.
- **The solar system** (World → Solar system): stars of real classes (from brown dwarfs to black holes, with "Reality Following" sizes and temperatures), planets and moons with elliptical and tilted orbits, rings, axial tilt and day length. It shows in 3D with a clock, live n-body gravity, eclipses and shadows, a habitable zone and gravity limits. A star can sit in a Dyson sphere, the whole system can sit inside a shell, and moons can shine on their own. Other planets get their own painted maps, and every body has a wiki article.
- **The sky from any hex**: the true sky at any date and hour, with the sun, moons (with phases and eclipses), planets and the stars of your sky picture, at their real sizes and brightness, seen from the hex's ground height. You can upload your own sky picture.
- **Height chart**: characters side by side to scale, in cm or ft/in, with outlines by sex, age and build.
- **Characters**: an Earth birthdate and Earth name, blood types with their Japanese meanings, and birth date, blood type and place of birth kept on the character rather than on each version.
- **Calendar**: month names, era names for the years before year 1, and a calendar button on every date box.

**Everywhere**

- **Navigation**: breadcrumbs, the same links bar on every item page, previous and next, a "not found" page, and Ctrl+K actions with recently opened items (also on Home).
- Unsaved-changes warnings on forms, Save bars that stay in reach, drag and drop for pictures, chapters reordered by dragging, and rare actions in "…" menus.
- Phone layouts for the calendar, map, character sheet, graph (with pinch zoom), timeline and lists.

### Changed

- **The license is now the AGPL-3.0** (it was MIT), with the additional terms in `NOTICE.md`: keep the credit, and give a changed version its own name. See the README.
- **Wiki links are stored by id**, so renaming a character or place updates every link to it. Typing `[[Name]]` still works.
- Dates read one way everywhere ("15/03/1054", "15 Frostfall 1054", "1054 Q2").
- Pages use the whole width of the window.
- The README has new screenshots and describes all of the above.

### Fixed

- A story saved by a newer Polaris is now refused instead of being reset, and importing a newer export says so.
- Uploading a picture no longer replaces another file that has the same name.

## 2.1.0 (2026-10-11)

**Upgrade:** stories are upgraded to database schema 25 the first time they are opened. This can't be undone, so export your stories first.

### Added

- **Bestiary.** A new page for the monsters, animals and constructs of your world: each has a tag, a level range, the place it's native to, a description, a picture and related creatures (related both ways). Every creature has its own wiki article, and a place's article lists the creatures native to it.
- **Creature territories.** A creature lives on the area of the place it's native to, or on hexes you paint yourself in its form. Its wiki article shows the territory as a picture of the map: its hexes in the colour of its tag, the rest of the map grey, empty hexes black.
- Gear has a description, shown in the gear list, when hovering an equipped piece, and in its wiki article.
- A stat can be marked "Lower is better" in Settings → Stats, so negative bonuses and falling values show as improvements. Cold Threshold has it on.

### Changed

- The sidebar is redesigned: pages grouped into Story, World, Time and Lore with icons, the story at the top, Settings at the bottom, and a button to collapse it to icons only. It fits short screens, and on phones it slides in from the side.
- Dates read as "01 / 01 / -2000" instead of "01-01-(-2000)".

## 2.0.2 (2026-10-11)

### Added

- Stat formulas can use the bonuses from classes, subclasses, specializations, races, body types and gear: Settings → Stats has a block for each (Class bonus, Race bonus, …). A formula that uses one decides how that bonus counts, for example `Race bonus × Level × 2`, instead of it being added on its own. Bonuses a formula doesn't use are added as before.
- The stats preview in Settings has an "Each bonus" sample, to see how the bonuses come out.

## 2.0.1 (2026-10-11)

**Upgrade:** stories are upgraded to database schema 23 the first time they are opened (if 2.0.0 couldn't upgrade them). This can't be undone, so export your stories first.

### Fixed

- Some existing stories couldn't be opened or imported after updating to 2.0.0 ("FOREIGN KEY constraint failed"). Their data was never changed; they now upgrade normally.
- Deleting a character version now always removes everything that belonged to it. Before, it sometimes left rows behind, which is what tripped the 2.0.0 upgrade.

## 2.0.0 (2026-10-11)

**Upgrade:** stories are upgraded to database schema 23 the first time they are opened. This can't be undone, so export your stories first. If you made your own stat formulas in `backend/stats_custom.go`, delete that file before updating (it no longer builds) and make the formulas again under Settings → Stats.

### Added

- **Your own stats.** Settings → Stats sets which stats your characters have and how each one is calculated. In each category (Primary Stats, Base Stats, Special Stats, Special Defenses, Lifeskills) you can add, remove, rename and reorder stats, and change their formulas, units, decimals and limits. Formulas are built from blocks (Level, VIT, numbers, + − × ÷, functions such as min, max and round), a stat can be calculated from another, and a preview shows what each formula gives while you edit. The character sheet, the bonuses of classes, races and gear, the character cards and version comparisons all follow it. Every story starts with the stats Polaris has always had, so nothing changes until you change it.
- **Ready-made Docker image.** Each release is published as `ghcr.io/astralumi/polaris` (for regular PCs and servers, and ARM boards such as a Raspberry Pi). `docker compose up -d` runs it and `scripts/update.sh` updates to it; `--build` still builds from the code. `POLARIS_VERSION` in `.env` pins a release.
- Races and specializations can be native to a kingdom. Their wiki articles link it, and the kingdom's article lists them.
- The calendar marks each character's birthday every year (when the birth date has a day and month), with their age.
- Gender is picked with three icons (unknown, male, female), with a box for anything else.

### Changed

- Birth date, blood type and birthplace belong to the character instead of each version: changing them in any version changes them for all. Upgrading takes them from the current version (or another one when it has none).
- Date boxes show the digits in the order you type them (13-02-1456); the last four digits are still the year, so a year alone still works.
- Character cards show "Class | Specialization", with the subclass on its own line.

### Fixed

- Replacing a character's picture showed the old picture again after saving.
- Pressing inside a window and letting go outside it no longer closes the window.

## 1.6.4 (2026-10-11)

### Changed

- The README describes Polaris as a story organizer, with a shorter feature list and new screenshots.

### Fixed

- Family trees: a character with two pairs of parents (born to one couple, adopted by another) now hangs from each couple, instead of from a point between them.
- Link previews no longer leave a space where bold or italic marks were.

## 1.6.3 (2026-10-11)

### Added

- Kingdom articles show their capital.

### Changed

- On the graph, a kingdom and its faction are one node.

### Fixed

- A city, capital or other place with no "belongs to" of its own now belongs to the kingdom painted on its hex, as the map already showed: it is "part of" the kingdom in the wiki, listed under its places, and linked to it on the graph.

## 1.6.2 (2026-10-11)

### Changed

- The wiki infobox groups a character's facts into sections: identity, Personal, Birth and Abilities.

## 1.6.1 (2026-10-11)

**Upgrade:** stories are upgraded to database schema 21 the first time they are opened. This can't be undone, so export your stories first.

### Added

- Character infoboxes show age, blood type and human birth date when they are filled in.

### Fixed

- Saving a version with a specialization made a new copy of the specialization every time. Existing copies are merged into one on upgrade (versions, spells, stat bonuses and wiki text move over), and duplicates can no longer be made.

## 1.6.0 (2026-10-11)

**Upgrade:** stories are upgraded to database schema 20 the first time they are opened. This can't be undone, so export your stories first.

### Added

- Relations can start and end at a chapter as well as a date. Each version of a character shows the relations that hold at its own chapter or date; the Relations tab lists the others after them.
- A character's wiki article has a tab per version for its facts and relations.
- Names from a character's other versions work as aliases: [[links]] and search still find them, and the infobox lists them as "Also known as".
- Every kingdom on the map has a faction tied to it, kept in step from the map. Existing kingdoms get theirs on upgrade (a faction that already has the kingdom's name is tied to it instead of duplicated).

## 1.5.1 (2026-10-11)

### Changed

- Dates are typed in one DD-MM-YYYY box whose digits fill in from the right: 2 is the year 2, 21456 is month 2 of 1456, 3021456 is the 3rd of that month. "-" switches to a year before 1, and whole dates can be pasted.

### Fixed

- Tooltips no longer replace a form field's label for screen readers.

## 1.5.0 (2026-10-11)

**Upgrade:** stories are upgraded to database schema 19 the first time they are opened. This can't be undone, so export your stories first.

### Added

- Picture galleries on any wiki article: captioned pictures, added several at a time, reordered, and viewed full size one after another.
- A calendar: every event, birth, founding and dated lore article a month at a time, in the story's own calendar.

## 1.4.0 (2026-10-11)

### Added

- Family trees: a character's parents, children, siblings and partners in generations, with ended marriages dashed.
- A relationship web: the graph can show only what lies within 1-3 connections of an article.

## 1.3.0 (2026-10-11)

### Added

- A graph of the whole wiki: every article and connection, with story connections solid and [[links]] dashed. Hover to see an article's card and neighbours, click to open it; filter by kind, find by name.

## 1.2.0 (2026-10-10)

**Upgrade:** stories are upgraded to database schema 18 the first time they are opened. This can't be undone, so export your stories first.

### Added

- Hovering a link to a wiki article shows a preview card.
- Tags on characters, shared by all their versions, with a filter on the Characters page.
- Lore articles can have dated rows that put them on the timeline and calendar.
- Chapters can be deleted from the Chapters list.

## 1.1.0 (2026-10-10)

**Upgrade:** stories are upgraded to database schema 17 the first time they are opened. This can't be undone, so export your stories first.

### Added

- Free-form lore articles in the wiki (gods, history, magic…), with a picture and your own sections.
- Infobox values can link to articles with the link picker.

## 1.0.0 (2026-10-10)

### Added

- Styled tooltips across the app, explaining sections, tabs, stats and actions; names cut off by an ellipsis show in full on hover.

## 0.10.0 (2026-10-10)

### Added

- The timeline can highlight a person or place instead of hiding everything else; a character's Timeline button uses it.
- Links into the map centre and zoom on the place they point at.

## 0.9.1 (2026-10-10)

### Fixed

- The Relations tab was missing when editing a character version.

## 0.9.0 (2026-10-10)

### Added

- A Markdown toolbar and preview in the wiki editor, and a picker that searches the wiki to insert [[links]].

## 0.8.0 (2026-10-10)

**Upgrade:** stories are upgraded to database schema 16 the first time they are opened. This can't be undone, so export your stories first.

### Added

- Relations between characters (parent, sibling, mentor, rival, custom wording…), read from both sides, with optional dates.
- Factions with a picture, colour, headquarters, parent faction, founding date and members with roles.
- Chapters grouped into volumes, with notes; events and character versions can point to a chapter.
- The story picker shows when each story was last exported.

### Fixed

- The character page now reloads when linking to another character; the version editor fits on phones.

## 0.7.0 (2026-10-09)

**Upgrade:** stories are upgraded to database schema 15 the first time they are opened. This can't be undone, so export your stories first.

### Added

- Each version of a character is alive, missing or dead, with a badge and a filter.
- Print a character sheet, or save it as a PDF.
- Wiki: random article and recently changed.
- Custom stat formulas in a git-ignored stats_custom.go that survives updates (see the README).

## 0.6.1 (2026-10-09)

### Fixed

- Painting or erasing on the map highlights every hex the brush will touch, not only the one under the cursor.

## 0.6.0 (2026-10-09)

### Added

- Ctrl+K search across characters, places, events, assets and pages.
- Compare two versions of a character.
- Map undo and redo, brush sizes and pinch zoom.
- A phone layout; pictures open full size; timeline filters by person, place and kind; deleting several events or assets at once.

### Changed

- Long timeline gaps are drawn as a zigzag break.

## 0.5.0 (2026-10-09)

### Added

- Search, filters and sorting on the Characters page, kept in the address.
- An unsaved-changes guard and Ctrl+S in the editors; Esc closes dialogs.

### Changed

- Back links return to the page you came from.

## 0.4.0 (2026-10-09)

**Upgrade:** stories are upgraded to database schema 14 the first time they are opened. This can't be undone, so export your stories first.

### Added

- Cities and capitals on the map, with their own markers and labels.

## 0.3.0 (2026-10-08)

**Upgrade:** stories are upgraded to database schema 13 the first time they are opened. This can't be undone, so export your stories first.

### Added

- A wiki built from the story: every character, location, event and asset has an article, with your own text on top.
- The first tagged version, with characters and their versions, stats, gear, spells, the hex map, events and the timeline.
