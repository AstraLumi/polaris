package main

import (
	"database/sql"
	"log"
	"net/http"
	"sort"
	"strconv"
)

// timelineNode is one dot on the timeline. Free-standing events come from
// the events table; births and foundings are generated live from
// characters and locations, so renaming, redating or deleting the source
// is reflected immediately and nothing can go stale. When a birth or
// founding also has a details record (see events.go), that record's
// description, tags, people, picture and location ride along.
type timelineNode struct {
	Key          string        `json:"key"`  // "event:12", "birth:3", "founding:7"
	Kind         string        `json:"kind"` // "event" | "birth" | "founding"
	Name         string        `json:"name"`
	Date         string        `json:"date"`
	T            float64       `json:"t"`         // days on one number line, see timeValue
	EventID      *int64        `json:"event_id"`  // the events row, if there is one
	SourceID     *int64        `json:"source_id"` // character id (birth) / location id (founding)
	Description  string        `json:"description"`
	LocationID   *int64        `json:"location_id"`
	LocationName string        `json:"location_name"`
	PicturePath  string        `json:"picture_path"`
	Tags         []string      `json:"tags"`
	People       []eventPerson `json:"people"`
}

type timelinePayload struct {
	Calendar calendarSettings `json:"calendar"`
	YearDays int              `json:"year_days"`
	Nodes    []timelineNode   `json:"nodes"`
	// Undated counts free-standing events with no readable date. They exist
	// but have nowhere to sit, so the page says how many were left out.
	Undated int `json:"undated"`
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func kindRank(kind string) int {
	switch kind {
	case "birth":
		return 0
	case "founding":
		return 1
	}
	return 2
}

func timelineHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cal := loadCalendar(db)
		out := timelinePayload{
			Calendar: cal,
			YearDays: cal.MonthsPerYear * cal.DaysPerMonth,
			Nodes:    []timelineNode{},
		}

		rows, err := db.Query(eventSelect)
		if err != nil {
			http.Error(w, "failed to load timeline", http.StatusInternalServerError)
			log.Printf("timeline events: %v", err)
			return
		}
		var free []eventRow
		details := map[string]eventRow{} // "character:5" / "location:2" -> its details record
		for rows.Next() {
			er, err := scanEventRow(rows)
			if err != nil {
				rows.Close()
				http.Error(w, "failed to read timeline", http.StatusInternalServerError)
				log.Printf("timeline scan: %v", err)
				return
			}
			if er.sourceType == "" {
				free = append(free, er)
			} else {
				details[er.sourceType+":"+itoa(er.sourceID)] = er
			}
		}
		rows.Close()

		// withDetails copies a details record's content onto a node.
		withDetails := func(n *timelineNode, er eventRow) {
			id := er.d.ID
			n.EventID = &id
			n.Description = er.d.Description
			n.LocationID = er.d.LocationID
			n.LocationName = er.d.LocationName
			n.PicturePath = er.d.PicturePath
			n.Tags = loadEventTags(db, id)
			n.People = loadEventPeople(db, id)
		}
		place := func(n *timelineNode, date string) bool {
			d, ok := parseStoryDate(date)
			if !ok {
				return false
			}
			n.Date = d.String()
			n.T = timeValue(d, cal)
			return true
		}

		for _, er := range free {
			n := timelineNode{Key: "event:" + itoa(er.d.ID), Kind: "event", Name: er.d.Name,
				Tags: []string{}, People: []eventPerson{}}
			if !place(&n, er.d.EventDate) {
				out.Undated++
				continue
			}
			withDetails(&n, er)
			out.Nodes = append(out.Nodes, n)
		}

		// Births: only each character's current version counts. The in-story
		// birth date doesn't change between versions, so one per character.
		crow, err := db.Query(`
			SELECT c.id, v.name, COALESCE(st.birth_date, '')
			FROM characters c
			JOIN character_versions v ON v.character_id = c.id AND v.is_current = 1
			LEFT JOIN character_story st ON st.version_id = v.id`)
		if err != nil {
			http.Error(w, "failed to load timeline", http.StatusInternalServerError)
			log.Printf("timeline births: %v", err)
			return
		}
		for crow.Next() {
			var id int64
			var name, birth string
			if crow.Scan(&id, &name, &birth) != nil {
				continue
			}
			sid := id
			n := timelineNode{Key: "birth:" + itoa(id), Kind: "birth", Name: "Birth of " + name,
				SourceID: &sid, Tags: []string{}, People: []eventPerson{}}
			if !place(&n, normalizeStoryDate(birth)) {
				continue
			}
			if er, ok := details["character:"+itoa(id)]; ok {
				withDetails(&n, er)
			}
			out.Nodes = append(out.Nodes, n)
		}
		crow.Close()

		// Foundings: any location with a founding date, kingdom or not.
		lrow, err := db.Query(`SELECT id, name, COALESCE(founding_date, '') FROM locations`)
		if err != nil {
			http.Error(w, "failed to load timeline", http.StatusInternalServerError)
			log.Printf("timeline foundings: %v", err)
			return
		}
		for lrow.Next() {
			var id int64
			var name, founded string
			if lrow.Scan(&id, &name, &founded) != nil {
				continue
			}
			sid := id
			n := timelineNode{Key: "founding:" + itoa(id), Kind: "founding", Name: "Founding of " + name,
				SourceID: &sid, Tags: []string{}, People: []eventPerson{}}
			if !place(&n, normalizeStoryDate(founded)) {
				continue
			}
			if er, ok := details["location:"+itoa(id)]; ok {
				withDetails(&n, er)
			}
			out.Nodes = append(out.Nodes, n)
		}
		lrow.Close()

		sort.SliceStable(out.Nodes, func(i, j int) bool {
			a, b := out.Nodes[i], out.Nodes[j]
			if a.T != b.T {
				return a.T < b.T
			}
			if kindRank(a.Kind) != kindRank(b.Kind) {
				return kindRank(a.Kind) < kindRank(b.Kind)
			}
			return a.Key < b.Key
		})

		writeJSON(w, out)
	}
}
