package main

// Fonts the app ships with, for the book's "Fonts and licenses" list
// (fontlicense.go). The key is the lower-case family name.
//
// TODO: the next step fills this with the app's own fonts (see APP_FONTS in
// frontend/src/editor/fonts.js); the list works with it empty.

type bundledFont struct {
	License    string // "OFL-1.1" or "Apache-2.0"
	Copyright  string
	LicenseURL string
	Designer   string
}

var bundledFonts = map[string]bundledFont{}
