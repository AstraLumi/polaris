package main

// Reading a font file for the fonts library (fontlib.go; docs/EDITOR.md,
// "Fonts library"): what it is (sniffed from its first bytes, never its
// name), its names (the `name` table: family, style, full name, copyright,
// license text and URL, designer, vendor), its weight and slant (OS/2
// usWeightClass and fsSelection, else head's macStyle) and its embedding
// permissions (OS/2 fsType).
//
// Formats: TrueType and OpenType (sfnt) are read directly; WOFF is an sfnt
// whose tables may be zlib-compressed, read with compress/zlib. WOFF2
// compresses all its tables together with Brotli, which the standard
// library can't read and which isn't worth a dependency for a few names:
// a WOFF2 is recognised and kept, but its details come back unread
// (Read false) and the writer fills them in. Collections (.ttc) are
// refused: a browser can't load one with @font-face.

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"unicode/utf16"
)

// Embedding levels, from fsType's low four bits (the least restrictive
// bit set wins, as the OpenType spec asks for old fonts that set several).
const (
	embedInstallable = "installable"
	embedEditable    = "editable"
	embedPreview     = "preview" // preview & print
	embedRestricted  = "restricted"
	embedUnknown     = "unknown" // not read (WOFF2, or no OS/2 table)
)

type fontInfo struct {
	Format string // ttf, otf, woff, woff2
	Read   bool   // the tables below could be read

	Family, Style, FullName        string
	Copyright, License, LicenseURL string
	Designer, Vendor               string
	Weight                         int // 100–900
	Italic                         bool

	HasFSType    bool
	FSType       uint16
	Embedding    string // one of embed*
	NoSubsetting bool   // fsType bit 8
	BitmapOnly   bool   // fsType bit 9
}

// Limits on what a font file may claim, so a hostile file can't make the
// reader allocate much.
const (
	maxFontTables     = 512
	maxFontTableBytes = 64 << 20
)

var errNotFont = errors.New("this is not a font file the library can use (TTF, OTF, WOFF or WOFF2)")
var errFontCollection = errors.New("font collections (.ttc) can't be used; upload each font on its own")
var errBadFont = errors.New("this font file is damaged and can't be read")

// sniffFont says what format a file is from its first four bytes.
func sniffFont(data []byte) (string, error) {
	if len(data) < 12 {
		return "", errNotFont
	}
	switch string(data[:4]) {
	case "\x00\x01\x00\x00", "true":
		return "ttf", nil
	case "OTTO":
		return "otf", nil
	case "wOFF":
		return "woff", nil
	case "wOF2":
		return "woff2", nil
	case "ttcf":
		return "", errFontCollection
	}
	return "", errNotFont
}

// fontMIME is the content type a format is served with.
func fontMIME(format string) string {
	switch format {
	case "ttf":
		return "font/ttf"
	case "otf":
		return "font/otf"
	case "woff":
		return "font/woff"
	case "woff2":
		return "font/woff2"
	}
	return "application/octet-stream"
}

// parseFont reads a font file. A file of a known format whose tables can't
// be read is an error (errBadFont), except a WOFF2, which comes back with
// Read false.
func parseFont(data []byte) (*fontInfo, error) {
	format, err := sniffFont(data)
	if err != nil {
		return nil, err
	}
	info := &fontInfo{Format: format, Weight: 400, Embedding: embedUnknown}
	var tables map[string][]byte
	switch format {
	case "ttf", "otf":
		tables, err = sfntTables(data)
	case "woff":
		tables, err = woffTables(data)
	case "woff2":
		if len(data) < 48 {
			return nil, errBadFont
		}
		return info, nil
	}
	if err != nil {
		return nil, err
	}
	if tables["name"] == nil {
		return nil, errBadFont
	}
	readNames(info, tables["name"])
	if info.Family == "" {
		return nil, errBadFont
	}
	info.Read = true
	if os2 := tables["OS/2"]; len(os2) >= 64 {
		if w := int(binary.BigEndian.Uint16(os2[4:])); w >= 1 && w <= 1000 {
			info.Weight = roundWeight(w)
		}
		info.setFSType(binary.BigEndian.Uint16(os2[8:]))
		sel := binary.BigEndian.Uint16(os2[62:])
		info.Italic = sel&1 != 0 || sel&(1<<9) != 0
		if sel&(1<<5) != 0 && info.Weight < 600 {
			info.Weight = 700
		}
	} else if head := tables["head"]; len(head) >= 46 {
		mac := binary.BigEndian.Uint16(head[44:])
		if mac&1 != 0 {
			info.Weight = 700
		}
		info.Italic = mac&2 != 0
	}
	return info, nil
}

// roundWeight puts a weight class on the CSS steps (100–900). Some old
// fonts write 1–9 for 100–900.
func roundWeight(w int) int {
	if w < 10 {
		w *= 100
	}
	w = (w + 50) / 100 * 100
	return min(max(w, 100), 900)
}

func (f *fontInfo) setFSType(t uint16) {
	f.HasFSType = true
	f.FSType = t
	f.NoSubsetting = t&0x100 != 0
	f.BitmapOnly = t&0x200 != 0
	switch {
	case t&0xF == 0:
		f.Embedding = embedInstallable
	case t&0x8 != 0:
		f.Embedding = embedEditable
	case t&0x4 != 0:
		f.Embedding = embedPreview
	default: // 0x2 (or 0x1, reserved)
		f.Embedding = embedRestricted
	}
}

// forbidsEmbedding: the font may not be put in a document (restricted, or
// only its bitmaps may, which an outline font can't give).
func (f *fontInfo) forbidsEmbedding() bool {
	return f.Embedding == embedRestricted || f.BitmapOnly
}

// sfntTables reads an sfnt's table directory.
func sfntTables(data []byte) (map[string][]byte, error) {
	if len(data) < 12 {
		return nil, errBadFont
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	if n == 0 || n > maxFontTables || 12+16*n > len(data) {
		return nil, errBadFont
	}
	out := map[string][]byte{}
	for i := 0; i < n; i++ {
		rec := data[12+16*i:]
		tag := string(rec[:4])
		off := int64(binary.BigEndian.Uint32(rec[8:]))
		length := int64(binary.BigEndian.Uint32(rec[12:]))
		if off+length > int64(len(data)) {
			if tag == "name" || tag == "OS/2" || tag == "head" {
				return nil, errBadFont
			}
			continue
		}
		out[tag] = data[off : off+length]
	}
	return out, nil
}

// woffTables reads a WOFF 1.0's tables (only the few the library needs are
// inflated).
func woffTables(data []byte) (map[string][]byte, error) {
	if len(data) < 44 {
		return nil, errBadFont
	}
	n := int(binary.BigEndian.Uint16(data[12:]))
	if n == 0 || n > maxFontTables || 44+20*n > len(data) {
		return nil, errBadFont
	}
	out := map[string][]byte{}
	for i := 0; i < n; i++ {
		rec := data[44+20*i:]
		tag := string(rec[:4])
		if tag != "name" && tag != "OS/2" && tag != "head" {
			continue
		}
		off := int64(binary.BigEndian.Uint32(rec[4:]))
		comp := int64(binary.BigEndian.Uint32(rec[8:]))
		orig := int64(binary.BigEndian.Uint32(rec[12:]))
		if off+comp > int64(len(data)) || orig > maxFontTableBytes || comp > orig {
			return nil, errBadFont
		}
		raw := data[off : off+comp]
		if comp == orig {
			out[tag] = raw
			continue
		}
		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, errBadFont
		}
		table, err := io.ReadAll(io.LimitReader(zr, orig+1))
		zr.Close()
		if err != nil || int64(len(table)) != orig {
			return nil, errBadFont
		}
		out[tag] = table
	}
	return out, nil
}

// Name ids the library reads.
const (
	nameCopyright   = 0
	nameFamily      = 1
	nameSubfamily   = 2
	nameFull        = 4
	nameVendor      = 8
	nameDesigner    = 9
	nameLicense     = 13
	nameLicenseURL  = 14
	nameTypoFamily  = 16
	nameTypoSubfam  = 17
	maxNameRunes    = 5000
	maxShortNameLen = 200
)

// readNames fills the names, preferring Windows English (US), then any
// Windows or Unicode record, then Macintosh Roman. The typographic family
// and style (16, 17) win over the legacy ones (1, 2), so "Foo Light" is
// the weight 300 of "Foo".
func readNames(info *fontInfo, t []byte) {
	if len(t) < 6 {
		return
	}
	count := int(binary.BigEndian.Uint16(t[2:]))
	strOff := int(binary.BigEndian.Uint16(t[4:]))
	best := map[int]string{}
	rank := map[int]int{}
	for i := 0; i < count; i++ {
		if 6+12*i+12 > len(t) {
			break
		}
		r := t[6+12*i:]
		plat, enc, lang := binary.BigEndian.Uint16(r), binary.BigEndian.Uint16(r[2:]), binary.BigEndian.Uint16(r[4:])
		id := int(binary.BigEndian.Uint16(r[6:]))
		length, off := int(binary.BigEndian.Uint16(r[8:])), int(binary.BigEndian.Uint16(r[10:]))
		switch id {
		case nameCopyright, nameFamily, nameSubfamily, nameFull, nameVendor, nameDesigner, nameLicense, nameLicenseURL, nameTypoFamily, nameTypoSubfam:
		default:
			continue
		}
		start := strOff + off
		if start+length > len(t) {
			continue
		}
		raw := t[start : start+length]
		var s string
		score := 0
		switch {
		case plat == 3 && (enc == 1 || enc == 10 || enc == 0):
			s = utf16BE(raw)
			score = 3
			if lang == 0x0409 {
				score = 4
			}
		case plat == 0:
			s = utf16BE(raw)
			score = 2
		case plat == 1 && enc == 0:
			s = macRoman(raw)
			score = 1
			if lang == 0 {
				score = 2
			}
		default:
			continue
		}
		s = cleanFontName(s)
		if s == "" || score <= rank[id] {
			continue
		}
		best[id], rank[id] = s, score
	}
	short := func(s string) string { return truncateRunes(s, maxShortNameLen) }
	info.Family = short(firstOf(best[nameTypoFamily], best[nameFamily]))
	info.Style = short(firstOf(best[nameTypoSubfam], best[nameSubfamily]))
	info.FullName = short(best[nameFull])
	info.Copyright = truncateRunes(best[nameCopyright], maxNameRunes)
	info.License = truncateRunes(best[nameLicense], maxNameRunes)
	info.LicenseURL = short(best[nameLicenseURL])
	info.Designer = short(best[nameDesigner])
	info.Vendor = short(best[nameVendor])
}

func firstOf(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func utf16BE(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.BigEndian.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

// macRoman decodes Mac OS Roman (ASCII as is; the upper half mapped).
const macRomanUpper = "ÄÅÇÉÑÖÜáàâäãåçéèêëíìîïñóòôöõúùûü†°¢£§•¶ß®©™´¨≠ÆØ∞±≤≥¥µ∂∑∏π∫ªºΩæø¿¡¬√ƒ≈∆«»… ÀÃÕŒœ–—“”‘’÷◊ÿŸ⁄€‹›ﬁﬂ‡·‚„‰ÂÊÁËÈÍÎÏÌÓÔÒÚÛÙıˆ˜¯˘˙˚¸˝˛ˇ"

func macRoman(b []byte) string {
	table := []rune(macRomanUpper)
	var s strings.Builder
	for _, c := range b {
		if c < 0x80 {
			s.WriteByte(c)
		} else if int(c-0x80) < len(table) {
			s.WriteRune(table[c-0x80])
		}
	}
	return s.String()
}

// cleanFontName drops control characters and trims.
func cleanFontName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f || r == 0xfffd {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// guessFromFileName reads a family, weight and slant from a file name like
// "Foo-SemiBoldItalic.woff2", for a file whose tables can't be read.
func guessFromFileName(name string) (family string, weight int, italic bool) {
	base := name
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	weight = 400
	family, style := base, ""
	if i := strings.LastIndexAny(base, "-_"); i > 0 {
		family, style = base[:i], base[i+1:]
	}
	low := strings.ToLower(style)
	if strings.Contains(low, "italic") || strings.Contains(low, "oblique") {
		italic = true
		low = strings.NewReplacer("italic", "", "oblique", "").Replace(low)
	}
	weights := []struct {
		key string
		w   int
	}{
		{"extralight", 200}, {"ultralight", 200}, {"semibold", 600}, {"demibold", 600}, {"extrabold", 800},
		{"ultrabold", 800}, {"thin", 100}, {"hairline", 100}, {"light", 300}, {"regular", 400}, {"book", 400},
		{"normal", 400}, {"medium", 500}, {"bold", 700}, {"black", 900}, {"heavy", 900},
	}
	known := low == ""
	for _, x := range weights {
		if low == x.key {
			weight, known = x.w, true
			break
		}
	}
	if !known {
		// Not a style word: the whole name is the family.
		family, weight, italic = base, 400, false
	}
	family = strings.TrimSpace(strings.NewReplacer("_", " ").Replace(family))
	return truncateRunes(family, maxShortNameLen), weight, italic
}
