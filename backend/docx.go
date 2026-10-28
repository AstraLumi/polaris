package main

// Reading a Word (.docx) file into the writing editor's chapters
// (docs/EDITOR.md, stage 2). Pure: no database, no files written; the
// import handler (POST /api/books/{volume}/import-docx) stores the result.
//
// API:
//
//	parseDocx(data []byte) (*docxImport, error)
//	    Errors: errDocxNotZip / errDocxNoDocument (answer 400),
//	    errDocxTooBig (answer 413), anything else (400, a broken file).
//	imp.Chapters   []docxChapter{Title, FrontMatter, Doc, Words}
//	    The file split at Heading 1, in order. Each chapter's doc starts
//	    with its Heading 1 (the title stays in the text, with its
//	    formatting); Title is that heading's text. Text before the first
//	    Heading 1 becomes a chapter with FrontMatter true and Title "": name
//	    it docxFrontMatterTitle. Front matter with
//	    no text and no picture is dropped. A file with no Heading 1 is one
//	    front-matter chapter.
//	imp.Pictures   []docxPicture{Name, ContentType, Data}
//	    Each picture once (PNG, JPEG, GIF or WebP, sniffed from the bytes;
//	    Name is "image3.png"-like with an extension matching the type). An
//	    image node's src is the placeholder docxPictureSrc(i) =
//	    "docx-picture:<i>", i its index here. The handler saves each
//	    picture (saveUpload) and calls imp.usePictureURLs(urls), urls[i]
//	    being uploadURL(stored path) or "" to drop that picture's images;
//	    only then are the docs valid (validatePMDoc wants /uploads/ paths).
//	imp.Page, imp.PageTypeGuess, imp.Styles, imp.Fonts, imp.LeftOut
//	    The import answer's "page", "page_type_guess", "styles", "fonts",
//	    "left_out" (JSON tags match EDITOR.md; json.Marshal(imp) gives
//	    exactly those five keys, plus "layout" and "meta" when the file
//	    gives them: see docxlayout.go).
//
// Limits (a zip bomb or a huge file fails with errDocxTooBig): at most
// maxDocxEntries entries and maxDocxTotal bytes uncompressed in all, as
// declared and as actually read; an XML part at most maxDocxXML, a picture
// at most maxDocxPicture (a larger one is left out and counted).
//
// What maps to what is written beside the code that does it: formatting in
// docxstyles.go, the body in docxbody.go, styles, paragraphs and chapters
// in docxemit.go. The left-out keys are listed at docxLeftOutKeys.

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
)

// Variables only so tests can lower them.
var (
	maxDocxEntries = 5000
	maxDocxTotal   = int64(400 << 20)
	maxDocxXML     = int64(48 << 20)
	maxDocxPicture = int64(32 << 20)
)

var (
	errDocxNotZip     = errors.New("this is not a .docx file (Word 2007 or newer)")
	errDocxNoDocument = errors.New("this .docx file has no document in it")
	errDocxTooBig     = errors.New("this .docx file is too large to import")
)

// docxLeftOutKeys: what an import may report as not imported, with what
// each counts.
//
//	headers_footers  header and footer parts the sections use
//	toc              tables of contents (a TOC field or a TOC content control)
//	fields           page numbers, page references and index entries (dropped)
//	fields_as_text   other fields (cross-references, dates…): their shown text is kept as plain text
//	footnotes        footnote references (the notes' text is not imported)
//	endnotes         endnote references
//	comments         comments
//	tables           tables nested in a table's cell (their text is kept, cell by cell, as paragraphs in that cell)
//	text_boxes       text boxes (their text is kept as paragraphs after the paragraph they sit in)
//	equations        equations
//	tracked_changes  tracked insertions and deletions (accepted: insertions kept, deletions dropped)
//	drawings         shapes, charts, SmartArt, grouped pictures
//	objects          embedded objects (OLE) and embedded files
//	pictures         pictures in a format a browser can't show (EMF, WMF, TIFF…), missing, too large, or placed wholly off the page
//	hidden_text      runs of hidden text
//	drop_caps        drop caps (their letter is kept as text)
//	borders_shading  paragraph borders and shading
//	columns          sections set in more than one column
//	sections         section breaks beyond the first section: their own page setup, numbering and headers (each becomes a page break unless continuous)
var docxLeftOutKeys = []string{"headers_footers", "toc", "fields", "fields_as_text", "footnotes", "endnotes",
	"comments", "tables", "text_boxes", "equations", "tracked_changes", "drawings", "objects", "pictures",
	"hidden_text", "drop_caps", "borders_shading", "columns", "sections"}

type docxImport struct {
	Chapters      []docxChapter     `json:"-"`
	Pictures      []docxPicture     `json:"-"`
	Page          bookPage          `json:"page"`
	PageTypeGuess string            `json:"page_type_guess"`
	Styles        []bookStyle       `json:"styles"`
	Fonts         []string          `json:"fonts"`
	LeftOut       map[string]int    `json:"left_out"`
	Layout        *bookLayout       `json:"layout,omitempty"` // docxlayout.go
	Meta          map[string]string `json:"meta,omitempty"`
	Restarts      map[int]int       `json:"-"` // chapter index: the page number it restarts at (docxlayout.go)
}

type docxChapter struct {
	Title       string
	FrontMatter bool
	Doc         *pmNode
	Words       int
}

type docxPicture struct {
	Name        string
	ContentType string
	Data        []byte
}

// bookPage is EDITOR.md's book page setup ("page" in a book's settings).
type bookPage struct {
	WidthMM        float64 `json:"width_mm"`
	HeightMM       float64 `json:"height_mm"`
	MarginTopMM    float64 `json:"margin_top_mm"`
	MarginBottomMM float64 `json:"margin_bottom_mm"`
	MarginInnerMM  float64 `json:"margin_inner_mm"`
	MarginOuterMM  float64 `json:"margin_outer_mm"`
	GutterMM       float64 `json:"gutter_mm"`
	Mirror         bool    `json:"mirror"`
}

// bookStyle is EDITOR.md's style JSON. Para and Text hold only the keys
// the style sets (a key with a nil value sets "none", e.g. no colour).
type bookStyle struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Kind    string         `json:"kind"`
	BasedOn *string        `json:"based_on"`
	Builtin bool           `json:"builtin"`
	Para    map[string]any `json:"para,omitempty"`
	Text    map[string]any `json:"text"`
}

const docxPicturePrefix = "docx-picture:"

func docxPictureSrc(i int) string { return fmt.Sprintf("%s%d", docxPicturePrefix, i) }

// docxFrontMatterTitle names the chapter made of the text before the first
// Heading 1. Not the file's name: those are often numbered or prefixed
// ("01._NOVEL_-_…"), and the writer renames it in one click anyway.
const docxFrontMatterTitle = "Front matter"

// usePictureURLs puts the saved pictures' addresses in place of the
// placeholders; an image whose url is "" (or out of range) is removed.
func (imp *docxImport) usePictureURLs(urls []string) {
	for ci := range imp.Chapters {
		doc := imp.Chapters[ci].Doc
		var fix func(n *pmNode)
		fix = func(n *pmNode) {
			kept := n.Content[:0]
			for _, c := range n.Content {
				if c.Type == "image" {
					src, _ := c.Attrs["src"].(string)
					if strings.HasPrefix(src, docxPicturePrefix) {
						var i int
						if _, err := fmt.Sscanf(src[len(docxPicturePrefix):], "%d", &i); err != nil || i < 0 || i >= len(urls) || urls[i] == "" {
							continue
						}
						c.Attrs["src"] = urls[i]
					}
				}
				fix(c)
				kept = append(kept, c)
			}
			n.Content = kept
		}
		fix(doc)
		if len(doc.Content) == 0 {
			doc.Content = []*pmNode{{Type: "paragraph"}}
		}
	}
}

// --- the package ---

type docxPackage struct {
	files map[string]*zip.File
	read  int64 // bytes read so far
}

func (pk *docxPackage) file(name string) *zip.File {
	name = strings.TrimPrefix(name, "/")
	if f := pk.files[name]; f != nil {
		return f
	}
	for n, f := range pk.files {
		if strings.EqualFold(n, name) {
			return f
		}
	}
	return nil
}

// readPart reads one entry, at most max bytes (errDocxTooBig past it, or
// past the whole file's budget).
func (pk *docxPackage) readPart(name string, max int64) ([]byte, error) {
	f := pk.file(name)
	if f == nil {
		return nil, nil
	}
	if f.UncompressedSize64 > uint64(max) {
		return nil, errDocxTooBig
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errDocxTooBig
	}
	pk.read += int64(len(data))
	if pk.read > maxDocxTotal {
		return nil, errDocxTooBig
	}
	return data, nil
}

func (pk *docxPackage) xml(name string) (*xEl, error) {
	data, err := pk.readPart(name, maxDocxXML)
	if err != nil || data == nil {
		return nil, err
	}
	root, err := parseXML(data)
	if err == errXMLTooBig {
		return nil, errDocxTooBig
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return root, nil
}

type docxRel struct {
	typ, target string
	external    bool
}

// rels reads a part's relationships; targets are resolved to package paths
// (external ones kept as written).
func (pk *docxPackage) rels(part string) (map[string]docxRel, error) {
	dir, base := path.Split(part)
	root, err := pk.xml(dir + "_rels/" + base + ".rels")
	if err != nil || root == nil {
		return map[string]docxRel{}, err
	}
	out := map[string]docxRel{}
	for _, r := range root.kids {
		if r.name != "Relationship" {
			continue
		}
		rel := docxRel{typ: r.attr("Type"), target: r.attr("Target"), external: strings.EqualFold(r.attr("TargetMode"), "External")}
		if !rel.external {
			if strings.HasPrefix(rel.target, "/") {
				rel.target = strings.TrimPrefix(path.Clean(rel.target), "/")
			} else {
				rel.target = strings.TrimPrefix(path.Clean(path.Join(dir, rel.target)), "/")
			}
		}
		out[r.attr("Id")] = rel
	}
	return out, nil
}

func relOfType(rels map[string]docxRel, suffix string) string {
	ids := make([]string, 0, len(rels))
	for id := range rels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := rels[id]
		if !r.external && strings.HasSuffix(r.typ, suffix) {
			return r.target
		}
	}
	return ""
}

func openDocxPackage(data []byte) (*docxPackage, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errDocxNotZip
	}
	if len(zr.File) > maxDocxEntries {
		return nil, errDocxTooBig
	}
	pk := &docxPackage{files: map[string]*zip.File{}}
	var total uint64
	for _, f := range zr.File {
		total += f.UncompressedSize64
		if total > uint64(maxDocxTotal) {
			return nil, errDocxTooBig
		}
		pk.files[strings.TrimPrefix(f.Name, "/")] = f
	}
	return pk, nil
}

func parseDocx(data []byte) (*docxImport, error) {
	pk, err := openDocxPackage(data)
	if err != nil {
		return nil, err
	}
	// The main document: from the package relationships, else the usual
	// place.
	pkgRels, err := pk.rels("")
	if err != nil {
		return nil, err
	}
	main := relOfType(pkgRels, "/officeDocument")
	if main == "" || pk.file(main) == nil {
		main = "word/document.xml"
	}
	docRoot, err := pk.xml(main)
	if err != nil {
		return nil, err
	}
	if docRoot == nil || !docRoot.is("w", "document") || docRoot.child("w", "body") == nil {
		return nil, errDocxNoDocument
	}
	rels, err := pk.rels(main)
	if err != nil {
		return nil, err
	}
	dir := path.Dir(main)
	partOr := func(suffix, fallback string) string {
		if t := relOfType(rels, suffix); t != "" {
			return t
		}
		return path.Join(dir, fallback)
	}

	var th docxTheme
	if root, err := pk.xml(partOr("/theme", "theme/theme1.xml")); err != nil {
		return nil, err
	} else if root != nil {
		th = parseTheme(root)
	}
	stylesRoot, err := pk.xml(partOr("/styles", "styles.xml"))
	if err != nil {
		return nil, err
	}
	numRoot, err := pk.xml(partOr("/numbering", "numbering.xml"))
	if err != nil {
		return nil, err
	}
	settings, err := pk.xml(partOr("/settings", "settings.xml"))
	if err != nil {
		return nil, err
	}

	p := newDocxParser(pk, rels, parseStyles(stylesRoot, th), parseNumbering(numRoot), th)
	p.mirror = settings.child("w", "mirrorMargins") != nil && wOn(settings.child("w", "mirrorMargins"))
	if err := p.parseBody(docRoot.child("w", "body")); err != nil {
		return nil, err
	}
	p.hoistFullPagePictures() // docxlayout.go
	imp := p.finish()
	p.guessLayout(imp, settings, stylesRoot)
	if imp.Layout != nil {
		imp.Restarts = p.chapterRestarts(imp)
	}
	imp.Meta = docxMeta(pk, pkgRels)
	return imp, nil
}

// sniffPicture accepts the picture formats a browser shows.
func sniffPicture(data []byte) (contentType, ext string, ok bool) {
	ct := http.DetectContentType(data)
	switch ct {
	case "image/png":
		return ct, ".png", true
	case "image/jpeg":
		return ct, ".jpg", true
	case "image/gif":
		return ct, ".gif", true
	case "image/webp":
		return ct, ".webp", true
	}
	return "", "", false
}
