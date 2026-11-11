package main

// Schema 47: indexes only. They were found by filling a story far beyond
// what anyone has written (3,000 characters, 10,000 events, 1,000 chapters)
// and timing every page on it; each one below is for a lookup that read a
// whole table once per row of another.
//
// An index is paid for on every write to its table, so this is not "one
// for every column": a new one needs a read it makes faster.
var schema47Statements = []string{
	// The chapter list counts each chapter's events and character versions,
	// and deleting a chapter unlinks both (chapters_unlink).
	`CREATE INDEX idx_events_chapter ON events(chapter_id)`,
	`CREATE INDEX idx_character_versions_chapter ON character_versions(chapter_id)`,
	`CREATE INDEX idx_chapters_volume ON chapters(volume_id)`,

	// A character's bonuses: the rows of its class, race, gear…, not all of
	// them (loadModifierRowsFor).
	`CREATE INDEX idx_stat_modifiers_source ON stat_modifiers(source_type, source_id)`,

	// A wiki article's own sections and infobox, in their order.
	`CREATE INDEX idx_wiki_sections_entry ON wiki_sections(entry_id, position)`,
	`CREATE INDEX idx_wiki_infobox_entry ON wiki_infobox(entry_id, position)`,

	// "Whose is this?" from the other side: a character's events, who knows
	// a spell or wears a piece of gear (and deleting either), what happened
	// at a place, what lies inside one.
	`CREATE INDEX idx_event_characters_character ON event_characters(character_id)`,
	`CREATE INDEX idx_character_spells_spell ON character_spells(spell_id)`,
	`CREATE INDEX idx_character_gear_gear ON character_gear(gear_id)`,
	`CREATE INDEX idx_events_location ON events(location_id)`,
	`CREATE INDEX idx_locations_belongs_to ON locations(belongs_to_id)`,
}
