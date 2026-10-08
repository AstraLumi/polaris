package main

import "strings"

// Built-in icons are drawn by the frontend (src/builtinIcons.js), so the
// database only stores a name: "builtin:flame". An uploaded icon is stored
// as a path under the uploads directory instead. The two never mix — see
// iconChoiceFor.
const builtinIconPrefix = "builtin:"

// Keep in sync with frontend/src/builtinIcons.js.
var builtinSpellIcons = map[string]bool{
	"flame": true, "water": true, "leaf": true, "bolt": true, "ice": true,
	"wind": true, "earth": true, "sun": true, "moon": true, "sparkles": true,
	"heart": true, "shield": true, "skull": true, "eye": true, "sword": true,
	"tome": true, "star": true,
}

// Keep in sync with GEAR_ICONS in frontend/src/builtinIcons.js.
var builtinGearIcons = map[string]bool{
	"helmet": true, "mask": true, "necklace": true, "cape": true, "armor": true,
	"glove": true, "pants": true, "boots": true, "ring": true, "sword": true,
	"shield": true, "bow": true, "staff": true, "axe": true, "dagger": true,
	"hammer": true,
}

func isBuiltinIcon(stored string) bool {
	return strings.HasPrefix(stored, builtinIconPrefix)
}

// iconChoice is what a form says about a spell's icon, apart from any
// uploaded file: leave it alone (""), clear it ("none"), or use one of the
// built-in icons ("builtin:flame"). Anything unrecognized means "leave it".
type iconChoice struct {
	change bool
	value  string // "" clears; otherwise "builtin:<name>"
}

func parseIconChoice(raw string) iconChoice {
	return parseIconChoiceFrom(raw, builtinSpellIcons)
}

// parseIconChoiceFrom is parseIconChoice for any set of allowed built-in names.
func parseIconChoiceFrom(raw string, allowed map[string]bool) iconChoice {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "none":
		return iconChoice{change: true}
	case isBuiltinIcon(raw) && allowed[strings.TrimPrefix(raw, builtinIconPrefix)]:
		return iconChoice{change: true, value: raw}
	}
	return iconChoice{}
}
