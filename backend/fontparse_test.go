package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

// testFont describes a tiny sfnt made for the tests: only the tables the
// library reads (name, OS/2, head), no outlines. No third-party font is
// ever committed.
type testFont struct {
	family, style       string
	typoFamily          string // name id 16, when set
	weight, fsSelection uint16
	fsType              uint16
	license, licenseURL string
	copyright           string
	macOnly             bool // names only as Mac Roman records
	noOS2               bool
	macStyle            uint16
	magic               string // "\x00\x01\x00\x00" (default) or "OTTO"
}

func nameRecords(f testFont) []byte {
	type rec struct {
		id   uint16
		text string
	}
	recs := []rec{{0, f.copyright}, {1, f.family}, {2, f.style}, {4, f.family + " " + f.style}, {13, f.license}, {14, f.licenseURL}}
	if f.typoFamily != "" {
		recs = append(recs, rec{16, f.typoFamily})
	}
	var strs bytes.Buffer
	var dir bytes.Buffer
	n := 0
	put := func(plat, enc, lang, id uint16, data []byte) {
		binary.Write(&dir, binary.BigEndian, []uint16{plat, enc, lang, id, uint16(len(data)), uint16(strs.Len())})
		strs.Write(data)
		n++
	}
	for _, r := range recs {
		if r.text == "" {
			continue
		}
		if f.macOnly {
			put(1, 0, 0, r.id, []byte(r.text))
			continue
		}
		u := utf16.Encode([]rune(r.text))
		b := make([]byte, 2*len(u))
		for i, c := range u {
			binary.BigEndian.PutUint16(b[2*i:], c)
		}
		put(3, 1, 0x0409, r.id, b)
		// A worse record that must lose to it.
		put(1, 0, 0, r.id, []byte("WRONG"))
	}
	var out bytes.Buffer
	binary.Write(&out, binary.BigEndian, []uint16{0, uint16(n), uint16(6 + dir.Len())})
	out.Write(dir.Bytes())
	out.Write(strs.Bytes())
	return out.Bytes()
}

func buildSfnt(f testFont) []byte {
	tables := map[string][]byte{"name": nameRecords(f)}
	if !f.noOS2 {
		os2 := make([]byte, 96)
		binary.BigEndian.PutUint16(os2[0:], 4)
		binary.BigEndian.PutUint16(os2[4:], f.weight)
		binary.BigEndian.PutUint16(os2[6:], 5)
		binary.BigEndian.PutUint16(os2[8:], f.fsType)
		binary.BigEndian.PutUint16(os2[62:], f.fsSelection)
		tables["OS/2"] = os2
	}
	head := make([]byte, 54)
	binary.BigEndian.PutUint32(head[12:], 0x5F0F3CF5)
	binary.BigEndian.PutUint16(head[44:], f.macStyle)
	tables["head"] = head
	tags := []string{"OS/2", "head", "name"}
	magic := f.magic
	if magic == "" {
		magic = "\x00\x01\x00\x00"
	}
	var out bytes.Buffer
	out.WriteString(magic)
	present := 0
	for _, tg := range tags {
		if tables[tg] != nil {
			present++
		}
	}
	binary.Write(&out, binary.BigEndian, []uint16{uint16(present), 0, 0, 0})
	off := 12 + 16*present
	var body bytes.Buffer
	for _, tg := range tags {
		t := tables[tg]
		if t == nil {
			continue
		}
		out.WriteString(tg)
		binary.Write(&out, binary.BigEndian, []uint32{0, uint32(off + body.Len()), uint32(len(t))})
		body.Write(t)
		for body.Len()%4 != 0 {
			body.WriteByte(0)
		}
	}
	out.Write(body.Bytes())
	return out.Bytes()
}

// woffOf wraps an sfnt as a WOFF 1.0, compressing its tables.
func woffOf(t *testing.T, sfnt []byte) []byte {
	t.Helper()
	tables, err := sfntTables(sfnt)
	if err != nil {
		t.Fatal(err)
	}
	tags := []string{"OS/2", "head", "name"}
	var dir, body bytes.Buffer
	off := 44 + 20*len(tags)
	for _, tg := range tags {
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		zw.Write(tables[tg])
		zw.Close()
		data := z.Bytes()
		if len(data) >= len(tables[tg]) {
			data = tables[tg]
		}
		dir.WriteString(tg)
		binary.Write(&dir, binary.BigEndian, []uint32{uint32(off + body.Len()), uint32(len(data)), uint32(len(tables[tg])), 0})
		body.Write(data)
		for body.Len()%4 != 0 {
			body.WriteByte(0)
		}
	}
	var out bytes.Buffer
	out.WriteString("wOFF")
	out.Write(sfnt[:4])
	binary.Write(&out, binary.BigEndian, uint32(off+body.Len()))
	binary.Write(&out, binary.BigEndian, []uint16{uint16(len(tags)), 0})
	binary.Write(&out, binary.BigEndian, []uint32{uint32(len(sfnt))})
	binary.Write(&out, binary.BigEndian, []uint16{1, 0})
	binary.Write(&out, binary.BigEndian, []uint32{0, 0, 0, 0, 0})
	out.Write(dir.Bytes())
	out.Write(body.Bytes())
	return out.Bytes()
}

func TestParseFontNamesAndStyle(t *testing.T) {
	data := buildSfnt(testFont{family: "Test Serif", style: "Bold Italic", weight: 700, fsSelection: 1 | 1<<5,
		license: "This Font Software is licensed under the SIL Open Font License, Version 1.1.", licenseURL: "https://openfontlicense.org",
		copyright: "Copyright 2026 Nobody"})
	f, err := parseFont(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Format != "ttf" || !f.Read || f.Family != "Test Serif" || f.Style != "Bold Italic" || f.FullName != "Test Serif Bold Italic" {
		t.Errorf("names: %+v", f)
	}
	if f.Weight != 700 || !f.Italic {
		t.Errorf("weight %d italic %v", f.Weight, f.Italic)
	}
	if f.Copyright != "Copyright 2026 Nobody" || f.LicenseURL != "https://openfontlicense.org" {
		t.Errorf("copyright/license: %+v", f)
	}
	if f.Embedding != embedInstallable || f.forbidsEmbedding() {
		t.Errorf("fsType 0 is installable: %s", f.Embedding)
	}
	if kind, all := guessLicense(f); kind != "ofl" || !all {
		t.Errorf("license guess %s %v", kind, all)
	}
}

func TestParseFontTypographicFamilyAndWeights(t *testing.T) {
	f, err := parseFont(buildSfnt(testFont{family: "Test Sans Light", typoFamily: "Test Sans", style: "Regular", weight: 300, magic: "OTTO"}))
	if err != nil {
		t.Fatal(err)
	}
	if f.Format != "otf" || f.Family != "Test Sans" || f.Weight != 300 || f.Italic {
		t.Errorf("%+v", f)
	}
	// Old weight classes on a 1–9 scale; odd values round to the step.
	for in, want := range map[uint16]int{6: 600, 350: 400, 449: 400, 451: 500, 950: 900} {
		f, _ := parseFont(buildSfnt(testFont{family: "W", style: "x", weight: in}))
		if f.Weight != want {
			t.Errorf("weight %d → %d, want %d", in, f.Weight, want)
		}
	}
	// No OS/2: head's macStyle.
	f, _ = parseFont(buildSfnt(testFont{family: "Old", style: "Bold", noOS2: true, macStyle: 3}))
	if f.Weight != 700 || !f.Italic || f.Embedding != embedUnknown {
		t.Errorf("macStyle: %+v", f)
	}
	// Names only in Mac Roman.
	f, _ = parseFont(buildSfnt(testFont{family: "Caf\x8e", style: "Regular", macOnly: true}))
	if f.Family != "Café" {
		t.Errorf("Mac Roman family %q", f.Family)
	}
}

func TestParseFontEmbeddingFlags(t *testing.T) {
	cases := []struct {
		fs        uint16
		level     string
		forbids   bool
		noSubset  bool
		bitmapOnl bool
	}{
		{0, embedInstallable, false, false, false},
		{0x2, embedRestricted, true, false, false},
		{0x4, embedPreview, false, false, false},
		{0x8, embedEditable, false, false, false},
		{0x4 | 0x2, embedPreview, false, false, false}, // the least restrictive wins
		{0x8 | 0x100, embedEditable, false, true, false},
		{0x4 | 0x200, embedPreview, true, false, true},
	}
	for _, c := range cases {
		f, err := parseFont(buildSfnt(testFont{family: "E", style: "R", weight: 400, fsType: c.fs}))
		if err != nil {
			t.Fatal(err)
		}
		if f.Embedding != c.level || f.forbidsEmbedding() != c.forbids || f.NoSubsetting != c.noSubset || f.BitmapOnly != c.bitmapOnl || !f.HasFSType || f.FSType != c.fs {
			t.Errorf("fsType %#x: %+v", c.fs, f)
		}
	}
}

func TestParseWoffAndWoff2(t *testing.T) {
	sfnt := buildSfnt(testFont{family: "Web Serif", style: "Italic", weight: 400, fsSelection: 1, fsType: 0x8,
		license: "Licensed under the Apache License, Version 2.0"})
	w := woffOf(t, sfnt)
	f, err := parseFont(w)
	if err != nil {
		t.Fatal(err)
	}
	if f.Format != "woff" || f.Family != "Web Serif" || !f.Italic || f.Embedding != embedEditable {
		t.Errorf("woff: %+v", f)
	}
	if kind, all := guessLicense(f); kind != "apache" || !all {
		t.Errorf("apache guess: %s %v", kind, all)
	}
	// A broken zlib stream is a damaged file.
	bad := append([]byte(nil), w...)
	for i := 44 + 60; i < len(bad); i++ {
		bad[i] ^= 0x5a
	}
	if _, err := parseFont(bad); err != errBadFont {
		t.Errorf("damaged woff: %v", err)
	}
	// WOFF2: recognised, details unread.
	w2 := append([]byte("wOF2\x00\x01\x00\x00"), make([]byte, 60)...)
	f, err = parseFont(w2)
	if err != nil || f.Format != "woff2" || f.Read || f.Embedding != embedUnknown {
		t.Errorf("woff2: %+v %v", f, err)
	}
}

func TestSniffFontRefusals(t *testing.T) {
	if _, err := parseFont([]byte("ttcf\x00\x01\x00\x00\x00\x00\x00\x00")); err != errFontCollection {
		t.Errorf("ttc: %v", err)
	}
	if _, err := parseFont([]byte("%PDF-1.7 not a font at all")); err != errNotFont {
		t.Errorf("pdf: %v", err)
	}
	if _, err := parseFont([]byte("OTTO")); err != errNotFont {
		t.Errorf("too short: %v", err)
	}
	// A table directory pointing past the end.
	d := buildSfnt(testFont{family: "X", style: "Y"})
	if _, err := parseFont(d[:40]); err != errBadFont {
		t.Errorf("truncated: %v", err)
	}
	// No name table, or no family in it.
	if _, err := parseFont(buildSfnt(testFont{style: "Regular"})); err != errBadFont {
		t.Errorf("no family: %v", err)
	}
	if fontMIME("woff2") != "font/woff2" || fontMIME("ttf") != "font/ttf" {
		t.Error("MIME types")
	}
	if len([]rune(macRomanUpper)) != 128 {
		t.Errorf("Mac Roman table has %d entries", len([]rune(macRomanUpper)))
	}
}

func TestGuessFromFileName(t *testing.T) {
	cases := []struct {
		name   string
		family string
		weight int
		italic bool
	}{
		{"Foo-SemiBoldItalic.woff2", "Foo", 600, true},
		{"Foo-Italic.woff2", "Foo", 400, true},
		{"Bar_Bold.woff2", "Bar", 700, false},
		{"Open_Sans.woff2", "Open Sans", 400, false},
		{`C:\fonts\Plain.woff2`, "Plain", 400, false},
	}
	for _, c := range cases {
		fam, w, it := guessFromFileName(c.name)
		if fam != c.family || w != c.weight || it != c.italic {
			t.Errorf("%s → %q %d %v", c.name, fam, w, it)
		}
	}
	if kind, all := guessLicense(&fontInfo{License: "Free for personal use only."}); kind != "personal" || all {
		t.Errorf("personal: %s %v", kind, all)
	}
	if kind, _ := guessLicense(&fontInfo{License: "All rights reserved."}); kind != "" {
		t.Errorf("unknown: %s", kind)
	}
}
