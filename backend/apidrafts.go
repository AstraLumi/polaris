package main

// The API's tools for the drafts (drafts.go): the writer's notes and idea
// boards, kept apart from the story. Like every tool they call the story's
// own routes (call / callJSON), never a handler or the database.

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
)

// The colours a category made by add_card gets, in turn (the board's own
// list, frontend/src/drafts/board.js CATEGORY_COLORS).
var draftCategoryColors = []string{"#d0605a", "#e39b3c", "#d8c04a", "#5fae6e", "#4aa6b8", "#5b7fd6", "#9a6ad0", "#d06aa5", "#8a8f9c"}

// A closed card's tile on the board (board.js TILE).
const draftTileW, draftTileH = 180.0, 64.0

type draftsList struct {
	Pages      []draftPage     `json:"pages"`
	Categories []draftCategory `json:"categories"`
}

type draftBoard struct {
	Cards []draftCard `json:"cards"`
	Links []draftLink `json:"links"`
}

func draftTools() []apiTool {
	draftArg := num("The draft's id, from list_drafts.")
	return []apiTool{
		{Name: "list_drafts", Description: "The writer's drafts: loose notes and idea boards kept apart from the story (not canon, and not in the wiki). Each with its id, kind (note or board), title, words or cards; and the card categories (a name and a colour, shared by every board).",
			Schema: obj(map[string]any{"story": storyArg}, "story"),
			Run:    toolListDrafts},
		{Name: "read_draft", Description: "One draft. A note: its text as Markdown, with its version (needed to replace it). A board: its cards (id, title, category, Markdown body, place and size) and the lines joining them (between two card ids, with an optional label). Links show by name.",
			Schema: obj(map[string]any{"story": storyArg, "draft_id": draftArg}, "story", "draft_id"),
			Run:    toolReadDraft},
		{Name: "write_note", Write: true, Description: "Write into a draft note, from Markdown, as write_chapter does: every line a paragraph, # headings, **bold**, *italic*, - lists, [[Name]] links. mode append (default) adds after the text; replace swaps the whole text and needs base_version from read_draft.",
			Schema: obj(map[string]any{
				"story":        storyArg,
				"draft_id":     num("The note's id, from list_drafts."),
				"markdown":     str("The text."),
				"mode":         map[string]any{"type": "string", "enum": []string{"append", "replace"}, "description": "append (default) or replace."},
				"base_version": num("For replace: the version read_draft gave."),
				"allow_loss":   map[string]any{"type": "boolean", "description": "For replace: go ahead even if formatting Markdown can't hold is lost."},
			}, "story", "draft_id", "markdown"),
			Run: toolWriteNote},
		{Name: "add_card", Write: true, Description: "Add a card to a draft board, placed on a free spot (beside the `near` card when given), and join it to other cards with lines. The body is Markdown in Discord's syntax: **bold**, *italic*, __underline__, ~~strike~~, ||spoiler||, # / ## / ### headings, -# subtext, - lists (indent two spaces to nest), > quotes, `code`, [text](url) and [[Name]] links to story items. A category is given by name and made (with a colour) if the story has none of that name.",
			Schema: obj(map[string]any{
				"story":      storyArg,
				"draft_id":   num("The board's id, from list_drafts."),
				"title":      str("The card's title (short: it is what the closed card shows)."),
				"body":       str("Markdown."),
				"category":   str("A category name, such as Plot or Worldbuilding; empty for none."),
				"near":       num("Put the card beside this card id."),
				"join":       map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Card ids of this board to join the new card to."},
				"join_label": str("A label for those lines, such as \"leads to\" (optional)."),
			}, "story", "draft_id", "title"),
			Run: toolAddCard},
	}
}

func draftIDArg(args map[string]any) (int64, error) {
	id, ok := argInt(args, "draft_id")
	if !ok {
		return 0, errors.New("draft_id: the draft's id, from list_drafts")
	}
	return id, nil
}

func toolListDrafts(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	var list draftsList
	if _, err := a.callJSON(r, k, argStr(args, "story"), http.MethodGet, "drafts", nil, &list, "list_drafts"); err != nil {
		return nil, err
	}
	return list, nil
}

// draftPageOf finds a draft in the list (its kind and title) and gives the
// categories along.
func draftPageOf(a *apiServer, r *http.Request, k *apiKey, story string, id int64, tool string) (draftPage, draftsList, error) {
	var list draftsList
	if _, err := a.callJSON(r, k, story, http.MethodGet, "drafts", nil, &list, tool); err != nil {
		return draftPage{}, list, err
	}
	for _, p := range list.Pages {
		if p.ID == id {
			return p, list, nil
		}
	}
	return draftPage{}, list, fmt.Errorf("there is no draft %d: list_drafts gives the ids", id)
}

func toolReadDraft(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	id, err := draftIDArg(args)
	if err != nil {
		return nil, err
	}
	story := argStr(args, "story")
	page, list, err := draftPageOf(a, r, k, story, id, "read_draft")
	if err != nil {
		return nil, err
	}
	x := a.linkIndex(r, k, story)
	if page.Kind == "note" {
		var t chapterText
		if _, err := a.callJSON(r, k, story, http.MethodGet, fmt.Sprintf("drafts/%d/note", id), nil, &t, "read_draft"); err != nil {
			return nil, err
		}
		doc, err := parsePMDoc(t.Doc)
		if err != nil {
			return nil, errors.New("the note's text could not be read")
		}
		return map[string]any{"id": id, "kind": "note", "title": page.Title, "version": t.Version, "words": t.Words,
			"updated_at": t.UpdatedAt, "markdown": docMarkdown(doc, x)}, nil
	}
	var b draftBoard
	if _, err := a.callJSON(r, k, story, http.MethodGet, fmt.Sprintf("drafts/%d/board", id), nil, &b, "read_draft"); err != nil {
		return nil, err
	}
	catName := map[int64]string{}
	for _, c := range list.Categories {
		catName[c.ID] = c.Name
	}
	title := map[int64]string{}
	cards := make([]map[string]any, 0, len(b.Cards))
	for _, c := range b.Cards {
		title[c.ID] = c.Title
		cat := ""
		if c.CategoryID != nil {
			cat = catName[*c.CategoryID]
		}
		cards = append(cards, map[string]any{"id": c.ID, "title": c.Title, "category": cat, "category_id": c.CategoryID,
			"body": shownByName(x, c.Body), "x": c.X, "y": c.Y, "w": c.W, "h": c.H})
	}
	lines := make([]map[string]any, 0, len(b.Links))
	for _, l := range b.Links {
		lines = append(lines, map[string]any{"id": l.ID, "a": l.A, "b": l.B, "a_title": title[l.A], "b_title": title[l.B], "label": l.Label})
	}
	return map[string]any{"id": id, "kind": "board", "title": page.Title, "categories": list.Categories, "cards": cards, "lines": lines}, nil
}

func toolWriteNote(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	id, err := draftIDArg(args)
	if err != nil {
		return nil, err
	}
	page, _, err := draftPageOf(a, r, k, argStr(args, "story"), id, "write_note")
	if err != nil {
		return nil, err
	}
	if page.Kind != "note" {
		return nil, errors.New("that draft is a board: add_card writes on a board")
	}
	return writeMarkdownText(a, r, k, args, fmt.Sprintf("drafts/%d/note", id), "note", "read_draft", "write_note")
}

// draftFreeSpot finds a place for a new tile from (x, y): moved down, then
// right, until it covers no other card (board.js freeSpot, from a corner).
func draftFreeSpot(cards []draftCard, x, y float64) (float64, float64) {
	hits := func(px, py float64) bool {
		for _, c := range cards {
			if px < c.X+draftTileW+12 && c.X < px+draftTileW+12 && py < c.Y+draftTileH+12 && c.Y < py+draftTileH+12 {
				return true
			}
		}
		return false
	}
	for col := 0; col < 40; col++ {
		for row := 0; row < 8; row++ {
			px := x + float64(col)*(draftTileW+24)
			py := y + float64(row)*(draftTileH+20)
			if !hits(px, py) {
				return px, py
			}
		}
	}
	return x, y
}

func toolAddCard(a *apiServer, r *http.Request, k *apiKey, args map[string]any) (any, error) {
	id, err := draftIDArg(args)
	if err != nil {
		return nil, err
	}
	story := argStr(args, "story")
	page, list, err := draftPageOf(a, r, k, story, id, "add_card")
	if err != nil {
		return nil, err
	}
	if page.Kind != "board" {
		return nil, errors.New("that draft is a note: write_note writes in a note")
	}
	var b draftBoard
	if _, err := a.callJSON(r, k, story, http.MethodGet, fmt.Sprintf("drafts/%d/board", id), nil, &b, "add_card"); err != nil {
		return nil, err
	}

	// Where: beside the `near` card, else under everything.
	x, y := 0.0, 0.0
	nearID, hasNear := argInt(args, "near")
	placed := false
	if hasNear {
		for _, c := range b.Cards {
			if c.ID == nearID {
				x, y, placed = c.X+draftTileW+60, c.Y, true
			}
		}
		if !placed {
			return nil, fmt.Errorf("there is no card %d on this board", nearID)
		}
	} else if len(b.Cards) > 0 {
		minX, maxY := math.Inf(1), math.Inf(-1)
		for _, c := range b.Cards {
			minX = math.Min(minX, c.X)
			maxY = math.Max(maxY, c.Y)
		}
		x, y = minX, maxY+draftTileH+40
	}
	x, y = draftFreeSpot(b.Cards, x, y)

	card := map[string]any{"title": argStr(args, "title"), "x": x, "y": y, "w": 320, "h": 260}
	if body, ok := args["body"].(string); ok {
		card["body"] = body
	}
	if name := argStr(args, "category"); name != "" {
		var catID int64
		for _, c := range list.Categories {
			if strings.EqualFold(c.Name, name) {
				catID = c.ID
			}
		}
		if catID == 0 {
			color := draftCategoryColors[len(list.Categories)%len(draftCategoryColors)]
			var made draftCategory
			if _, err := a.callJSON(r, k, story, http.MethodPost, "draft-categories", map[string]any{"name": name, "color": color}, &made, "add_card"); err != nil {
				return nil, err
			}
			catID = made.ID
		}
		card["category_id"] = catID
	}
	var made draftCard
	if _, err := a.callJSON(r, k, story, http.MethodPost, fmt.Sprintf("drafts/%d/cards", id), card, &made, "add_card"); err != nil {
		return nil, err
	}

	out := map[string]any{"card": made}
	if raw, ok := args["join"].([]any); ok && len(raw) > 0 {
		label := argStr(args, "join_label")
		var joined []int64
		var failed []string
		for _, v := range raw {
			other, ok := v.(float64)
			if !ok || other != math.Trunc(other) {
				failed = append(failed, fmt.Sprint(v))
				continue
			}
			var l draftLink
			if _, err := a.callJSON(r, k, story, http.MethodPost, fmt.Sprintf("drafts/%d/links", id),
				map[string]any{"a": made.ID, "b": int64(other), "label": label}, &l, "add_card"); err != nil {
				failed = append(failed, fmt.Sprintf("%d (%s)", int64(other), err.Error()))
				continue
			}
			joined = append(joined, int64(other))
		}
		out["joined"] = joined
		if len(failed) > 0 {
			out["not_joined"] = failed
		}
	}
	return out, nil
}
