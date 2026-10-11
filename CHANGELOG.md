# Changelog

What changed in each version of Polaris. The same notes are on the
[Releases](https://github.com/AstraLumi/polaris/releases) page. Versions are
Major.Minor.Bugfix: new features raise Minor, fixes raise Bugfix.

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
