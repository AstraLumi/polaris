package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// uploadFonts posts files to the library (field "file", several allowed).
func uploadFonts(t *testing.T, h http.Handler, fields map[string]string, files map[string][]byte) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	for name, data := range files {
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write(data)
	}
	mw.Close()
	rec := do(t, h, "POST", "/api/fonts", &buf, mw.FormDataContentType())
	return rec.Code, rec.Body.String()
}

func listFonts(t *testing.T, h http.Handler) []fontFamilyOut {
	t.Helper()
	rec := do(t, h, "GET", "/api/fonts", nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var out []fontFamilyOut
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func familyNamed(list []fontFamilyOut, name string) *fontFamilyOut {
	for i := range list {
		if strings.EqualFold(list[i].Name, name) {
			return &list[i]
		}
	}
	return nil
}

func TestFontLibraryUpload(t *testing.T) {
	mgr, h := newTestServer(t)
	regular := buildSfnt(testFont{family: "Test Serif", style: "Regular", weight: 400, license: "SIL Open Font License 1.1"})
	bold := buildSfnt(testFont{family: "Test Serif", style: "Bold", weight: 700, fsSelection: 1 << 5})
	other := buildSfnt(testFont{family: "Shop Script", style: "Regular", weight: 400, fsType: 0x2, license: "Free for personal use only"})
	code, body := uploadFonts(t, h, nil, map[string][]byte{"TestSerif-Regular.ttf": regular, "TestSerif-Bold.ttf": bold, "Shop.ttf": other})
	if code != 200 {
		t.Fatal(code, body)
	}
	list := listFonts(t, h)
	if len(list) != 2 {
		t.Fatalf("%d families, want 2: %+v", len(list), list)
	}
	serif := familyNamed(list, "Test Serif")
	if serif == nil || len(serif.Files) != 2 {
		t.Fatalf("Test Serif: %+v", serif)
	}
	// Prefilled from the name table: OFL allows everything.
	if serif.LicenseKind != "ofl" || !serif.Uses.Print || !serif.Uses.Ebook || !serif.Uses.Web || !serif.Uses.Commercial {
		t.Errorf("OFL prefill: %+v", serif)
	}
	if serif.Files[0].Weight != 400 || serif.Files[1].Weight != 700 || serif.Files[0].Format != "ttf" || !serif.Files[0].DetailsRead {
		t.Errorf("files: %+v", serif.Files)
	}
	shop := familyNamed(list, "Shop Script")
	if shop.LicenseKind != "personal" || shop.Uses.Commercial || shop.Files[0].Embedding != embedRestricted {
		t.Errorf("personal prefill: %+v", shop)
	}

	// The same file again is refused; so is something that isn't a font.
	if code, body := uploadFonts(t, h, nil, map[string][]byte{"again.ttf": regular}); code != 400 || !strings.Contains(body, "already in the library") {
		t.Errorf("duplicate: %d %s", code, body)
	}
	if code, body := uploadFonts(t, h, nil, map[string][]byte{"evil.ttf": []byte("<html><script>alert(1)</script></html>")}); code != 400 || !strings.Contains(body, "not a font file") {
		t.Errorf("not a font: %d %s", code, body)
	}
	// Mixed: one good, one bad → 200 with the problem listed.
	italic := buildSfnt(testFont{family: "Test Serif", style: "Italic", weight: 400, fsSelection: 1})
	code, body = uploadFonts(t, h, nil, map[string][]byte{"i.ttf": italic, "x.ttc": []byte("ttcf\x00\x01\x00\x00\x00\x00\x00\x00")})
	if code != 200 || !strings.Contains(body, "font collections") {
		t.Errorf("mixed: %d %s", code, body)
	}
	// Too large.
	old := maxFontFile
	maxFontFile = 100
	code, _ = uploadFonts(t, h, nil, map[string][]byte{"big.ttf": buildSfnt(testFont{family: "Big", style: "R"})})
	maxFontFile = old
	if code != http.StatusRequestEntityTooLarge {
		t.Errorf("too large: %d", code)
	}
	// A WOFF2 goes in by its file name, into the family asked for.
	w2 := append([]byte("wOF2\x00\x01\x00\x00"), make([]byte, 60)...)
	code, body = uploadFonts(t, h, map[string]string{"family_id": fmt.Sprint(serif.ID)}, map[string][]byte{"TestSerif-BoldItalic.woff2": w2})
	if code != 200 {
		t.Fatal(code, body)
	}
	serif = familyNamed(listFonts(t, h), "Test Serif")
	var woff2 *fontFileOut
	for i := range serif.Files {
		if serif.Files[i].Format == "woff2" {
			woff2 = &serif.Files[i]
		}
	}
	if woff2 == nil || woff2.DetailsRead || woff2.Weight != 700 || !woff2.Italic || woff2.Embedding != embedUnknown {
		t.Errorf("woff2: %+v", woff2)
	}
	if code, _ := uploadFonts(t, h, map[string]string{"family_id": "999"}, map[string][]byte{"x.ttf": buildSfnt(testFont{family: "Q", style: "R"})}); code != 400 {
		t.Errorf("unknown family: %d", code)
	}

	// Served for @font-face: its type, kept for good, never sniffed.
	rec := do(t, h, "GET", serif.Files[0].URL, nil, "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "font/ttf" || rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(rec.Header().Get("Cache-Control"), "immutable") || !bytes.Equal(rec.Body.Bytes(), regular) {
		t.Errorf("serving: %d %v", rec.Code, rec.Header())
	}
	for _, u := range []string{"/uploads/fonts/../stories.db", "/uploads/fonts/licenses", "/uploads/fonts/0123.ttf"} {
		if rec := do(t, h, "GET", u, nil, ""); rec.Code == 200 && rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
			t.Errorf("%s served: %d", u, rec.Code)
		}
	}
	// The files live outside every story's uploads.
	if _, err := os.Stat(filepath.Join(mgr.uploadsRoot, fontsDirName, filepath.Base(serif.Files[0].URL))); err != nil {
		t.Error(err)
	}
}

func TestFontLibraryRecord(t *testing.T) {
	_, h := newTestServer(t)
	data := buildSfnt(testFont{family: "Bought Serif", style: "Regular", weight: 400})
	if code, body := uploadFonts(t, h, nil, map[string][]byte{"b.otf": data}); code != 200 {
		t.Fatal(code, body)
	}
	fam := familyNamed(listFonts(t, h), "Bought Serif")
	if fam.LicenseKind != "" || fam.Uses.Print {
		t.Errorf("no license words: %+v", fam)
	}
	put := func(body string) (int, string) {
		rec := do(t, h, "PUT", fmt.Sprintf("/api/fonts/%d", fam.ID), strings.NewReader(body), "application/json")
		return rec.Code, rec.Body.String()
	}
	code, body := put(fmt.Sprintf(`{"name":"Bought Serif Pro","license_kind":"commercial","source":"https://shop.example/serif","purchase_date":"2026-03-15",
		"uses":{"print":true,"ebook":false,"web":false,"commercial":true},"notes":"Desktop licence, 1 seat","files":[{"id":%d,"weight":500,"italic":true}]}`, fam.Files[0].ID))
	if code != 200 {
		t.Fatal(code, body)
	}
	var got fontFamilyOut
	json.Unmarshal([]byte(body), &got)
	if got.Name != "Bought Serif Pro" || got.LicenseKind != "commercial" || got.PurchaseDate != "2026-03-15" || !got.Uses.Print || got.Uses.Ebook ||
		got.Notes != "Desktop licence, 1 seat" || got.Files[0].Weight != 500 || !got.Files[0].Italic {
		t.Errorf("saved: %+v", got)
	}
	for _, bad := range []string{`{"license_kind":"stolen"}`, `{"purchase_date":"15-03-2026"}`, `{"name":""}`, `{"name":"a\"b"}`,
		`{"files":[{"id":99999,"weight":400}]}`, fmt.Sprintf(`{"files":[{"id":%d,"weight":450}]}`, fam.Files[0].ID), `not json`} {
		if code, body := put(bad); code != 400 {
			t.Errorf("%s: %d %s", bad, code, body)
		}
	}
	// Names are unique whatever the case.
	uploadFonts(t, h, nil, map[string][]byte{"o.ttf": buildSfnt(testFont{family: "Other", style: "R"})})
	if code, body := put(`{"name":"other"}`); code != 400 || !strings.Contains(body, "another font") {
		t.Errorf("duplicate name: %d %s", code, body)
	}
	if rec := do(t, h, "PUT", "/api/fonts/9999", strings.NewReader(`{}`), "application/json"); rec.Code != 404 {
		t.Errorf("unknown: %d", rec.Code)
	}

	// The receipt: sniffed (a PDF), downloaded as an attachment, replaced, removed.
	lic := fmt.Sprintf("/api/fonts/%d/license-file", fam.ID)
	body2, ct := form(t, nil, "file", "receipt.pdf", []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF"))
	rec := do(t, h, "POST", lic, body2, ct)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"license_file":"receipt.pdf"`) {
		t.Fatalf("license upload: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, "GET", lic, nil, "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" || !strings.Contains(rec.Header().Get("Content-Disposition"), "receipt.pdf") ||
		rec.Header().Get("Content-Security-Policy") != "sandbox" {
		t.Errorf("license download: %d %v", rec.Code, rec.Header())
	}
	body2, ct = form(t, nil, "file", "x.html", []byte("<html><body>hi</body></html>"))
	if rec := do(t, h, "POST", lic, body2, ct); rec.Code != 400 {
		t.Errorf("html as a receipt: %d", rec.Code)
	}
	body2, ct = form(t, nil, "file", "terms.txt", []byte("You may print 5000 copies."))
	if rec := do(t, h, "POST", lic, body2, ct); rec.Code != 200 || !strings.Contains(rec.Body.String(), "terms.txt") {
		t.Errorf("text receipt: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "DELETE", lic, nil, ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"license_file":""`) {
		t.Errorf("license delete: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "GET", lic, nil, ""); rec.Code != 404 {
		t.Errorf("license after delete: %d", rec.Code)
	}
}

func TestFontLibraryDelete(t *testing.T) {
	mgr, h := newTestServer(t)
	a := buildSfnt(testFont{family: "Gone", style: "Regular", weight: 400})
	b := buildSfnt(testFont{family: "Gone", style: "Bold", weight: 700})
	uploadFonts(t, h, nil, map[string][]byte{"a.ttf": a, "b.ttf": b})
	fam := familyNamed(listFonts(t, h), "Gone")
	body, ct := form(t, nil, "file", "r.txt", []byte("receipt"))
	do(t, h, "POST", fmt.Sprintf("/api/fonts/%d/license-file", fam.ID), body, ct)
	dir := filepath.Join(mgr.uploadsRoot, fontsDirName)

	// One file.
	rec := do(t, h, "DELETE", fmt.Sprintf("/api/font-files/%d", fam.Files[1].ID), nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.Base(fam.Files[1].URL))); !os.IsNotExist(err) {
		t.Error("the deleted file is still there")
	}
	// The family, its files and its receipt.
	if rec := do(t, h, "DELETE", fmt.Sprintf("/api/fonts/%d", fam.ID), nil, ""); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if len(listFonts(t, h)) != 0 {
		t.Error("the family is still listed")
	}
	left, _ := os.ReadDir(dir)
	for _, e := range left {
		if !e.IsDir() {
			t.Errorf("left behind: %s", e.Name())
		}
	}
	lics, _ := os.ReadDir(filepath.Join(dir, fontLicenseSubdir))
	if len(lics) != 0 {
		t.Errorf("receipt left behind: %d", len(lics))
	}
	if rec := do(t, h, "DELETE", fmt.Sprintf("/api/fonts/%d", fam.ID), nil, ""); rec.Code != 404 {
		t.Errorf("twice: %d", rec.Code)
	}
}

// fontStory makes a story with a book whose Normal style and one chapter's
// text name library families.
func fontStory(t *testing.T, h http.Handler, name string) (string, int64, int64) {
	t.Helper()
	s := createStory(t, h, map[string]string{"name": name})
	base := "/api/s/" + s.ID
	rec := do(t, h, "POST", base+"/volumes", strings.NewReader(`{"title":"Book One"}`), "application/json")
	var v struct{ ID int64 }
	json.Unmarshal(rec.Body.Bytes(), &v)
	settings := `{"settings":{"page_type":"a5","styles":[
		{"id":"normal","name":"Normal","kind":"paragraph","builtin":true,"text":{"font":"test serif","size":10}},
		{"id":"quote","name":"Quote","kind":"paragraph","builtin":true,"text":{"font":"Shop Script"}},
		{"id":"loud","name":"Loud","kind":"character","text":{"font":"Shop Script"}}],
		"layout":{"header":{"odd":{"center":"{chapter}","font":"Band Face"}}}}}`
	if rec := do(t, h, "PUT", fmt.Sprintf("%s/books/%d", base, v.ID), strings.NewReader(settings), "application/json"); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	ch := addChapter(t, h, base, "Opening", v.ID)
	doc := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Plain "},` +
		`{"type":"text","marks":[{"type":"font","attrs":{"family":"Mark Face"}}],"text":"fancy"}]}]}`
	if code, _ := putText(t, h, base, ch, doc, 0); code != 200 {
		t.Fatal(code)
	}
	return s.ID, v.ID, ch
}

func TestFontsUsedIn(t *testing.T) {
	_, h := newTestServer(t)
	uploadFonts(t, h, nil, map[string][]byte{
		"a.ttf": buildSfnt(testFont{family: "Test Serif", style: "Regular", weight: 400}),
		"b.ttf": buildSfnt(testFont{family: "Shop Script", style: "Regular", weight: 400}),
		"c.ttf": buildSfnt(testFont{family: "Mark Face", style: "Regular", weight: 400}),
		"d.ttf": buildSfnt(testFont{family: "Unused", style: "Regular", weight: 400}),
	})
	id, vol, _ := fontStory(t, h, "Fonts One")
	fontStory(t, h, "Fonts Two")

	// One book's families, for the export warnings.
	rec := do(t, h, "GET", fmt.Sprintf("/api/s/%s/books/%d/fonts", id, vol), nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var uses []bookFontUse
	json.Unmarshal(rec.Body.Bytes(), &uses)
	byFam := map[string]bookFontUse{}
	for _, u := range uses {
		byFam[strings.ToLower(u.Family)] = u
	}
	if u := byFam["shop script"]; strings.Join(u.Styles, ",") != "Quote,Loud" || u.Book != "Book One" {
		t.Errorf("shop script: %+v", u)
	}
	if u := byFam["band face"]; u.Bands != 1 || len(u.Styles) != 0 {
		t.Errorf("band face: %+v", u)
	}
	if u := byFam["mark face"]; u.Chapters != 1 {
		t.Errorf("mark face: %+v", u)
	}
	if rec := do(t, h, "GET", fmt.Sprintf("/api/s/%s/books/999/fonts", id), nil, ""); rec.Code != 404 {
		t.Errorf("unknown book: %d", rec.Code)
	}

	// Across every story.
	rec = do(t, h, "GET", "/api/fonts/used", nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var used map[string][]fontUseOut
	json.Unmarshal(rec.Body.Bytes(), &used)
	list := listFonts(t, h)
	serif := used[fmt.Sprint(familyNamed(list, "Test Serif").ID)]
	if len(serif) != 2 || serif[0].Story != "Fonts One" || serif[1].Story != "Fonts Two" || strings.Join(serif[0].Styles, ",") != "Normal" {
		t.Errorf("Test Serif used in: %+v", serif)
	}
	if u := used[fmt.Sprint(familyNamed(list, "Mark Face").ID)]; len(u) != 2 || u[0].Chapters != 1 {
		t.Errorf("Mark Face used in: %+v", u)
	}
	if u := used[fmt.Sprint(familyNamed(list, "Unused").ID)]; len(u) != 0 {
		t.Errorf("Unused used in: %+v", u)
	}
}

func TestEPUBEmbedsAllowedFonts(t *testing.T) {
	_, h := newTestServer(t)
	serifR := buildSfnt(testFont{family: "Test Serif", style: "Regular", weight: 400, license: "SIL Open Font License"})
	serifI := buildSfnt(testFont{family: "Test Serif", style: "Italic", weight: 400, fsSelection: 1, license: "SIL Open Font License"})
	serifLocked := buildSfnt(testFont{family: "Test Serif", style: "Bold", weight: 700, fsType: 0x2, license: "SIL Open Font License"})
	shop := buildSfnt(testFont{family: "Shop Script", style: "Regular", weight: 400, license: "personal use only"})
	mark := buildSfnt(testFont{family: "Mark Face", style: "Regular", weight: 400, license: "Apache License 2.0"})
	unused := buildSfnt(testFont{family: "Unused", style: "Regular", weight: 400, license: "SIL Open Font License"})
	if code, body := uploadFonts(t, h, nil, map[string][]byte{"r.ttf": serifR, "i.ttf": serifI, "b.ttf": serifLocked, "s.ttf": shop, "m.ttf": mark, "u.ttf": unused}); code != 200 {
		t.Fatal(code, body)
	}
	id, vol, _ := fontStory(t, h, "Embed")
	rec := do(t, h, "GET", fmt.Sprintf("/api/s/%s/books/%d/export.epub", id, vol), nil, "")
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	files, _ := unzip(t, rec.Body.Bytes())
	css := string(files["OEBPS/style.css"])
	opf := string(files["OEBPS/content.opf"])
	enc := string(files["META-INF/encryption.xml"])
	wellFormed(t, "encryption.xml", files["META-INF/encryption.xml"])
	wellFormed(t, "content.opf", files["OEBPS/content.opf"])

	var fontFiles []string
	for name := range files {
		if strings.HasPrefix(name, "OEBPS/fonts/") {
			fontFiles = append(fontFiles, name)
		}
	}
	// Test Serif regular and italic (not the restricted bold), Mark Face;
	// not the personal-use Shop Script, not the unused one.
	if len(fontFiles) != 3 {
		t.Fatalf("%d fonts embedded: %v\n%s", len(fontFiles), fontFiles, css)
	}
	if strings.Count(css, "@font-face") != 3 || !strings.Contains(css, `font-family: "Test Serif"; src: url(fonts/`) ||
		!strings.Contains(css, `font-family: "Mark Face"`) || strings.Contains(css, `"Shop Script"; src`) || strings.Contains(css, "font-weight: 700") {
		t.Errorf("css:\n%s", css)
	}
	if !strings.Contains(css, "font-style: italic") {
		t.Errorf("no italic face:\n%s", css)
	}
	// The identifier unlocks them.
	var opfID struct {
		Identifier string `xml:"metadata>identifier"`
	}
	if err := xml.Unmarshal(files["OEBPS/content.opf"], &opfID); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{string(serifR): true, string(serifI): true, string(mark): true}
	for _, name := range fontFiles {
		href := strings.TrimPrefix(name, "OEBPS/")
		if !strings.Contains(opf, `href="`+href+`" media-type="font/ttf"`) {
			t.Errorf("%s not in the manifest", href)
		}
		if !strings.Contains(enc, `URI="`+name+`"`) {
			t.Errorf("%s not in encryption.xml", name)
		}
		if bytes.Equal(files[name][:64], serifR[:64]) {
			t.Errorf("%s is not obfuscated", name)
		}
		if !want[string(obfuscateFont(files[name], opfID.Identifier))] {
			t.Errorf("%s doesn't deobfuscate to an allowed font", name)
		}
	}

	// Without the library (a direct call), nothing is embedded.
	b, _ := testBook(t)
	data, _ := exportEPUB(b)
	plain, _ := unzip(t, data)
	if plain["META-INF/encryption.xml"] != nil || strings.Contains(string(plain["OEBPS/style.css"]), "@font-face") {
		t.Error("fonts embedded without a library")
	}
}

// fontRunsDoc is also in frontend/tests/fontPlaces.test.mjs: both count it
// the same way.
const fontRunsDoc = `{"type":"doc","content":[
	{"type":"paragraph","content":[
		{"type":"text","marks":[{"type":"font","attrs":{"family":"Cambria"}}],"text":"a"},
		{"type":"text","marks":[{"type":"bold"},{"type":"font","attrs":{"family":"Cambria"}}],"text":"b"},
		{"type":"text","text":"c"},
		{"type":"text","marks":[{"type":"font","attrs":{"family":"cambria"}}],"text":"d"},
		{"type":"hard_break","marks":[{"type":"font","attrs":{"family":"Cambria"}}]},
		{"type":"text","marks":[{"type":"font","attrs":{"family":"Other"}}],"text":"e"}]},
	{"type":"paragraph","content":[
		{"type":"text","marks":[{"type":"font","attrs":{"family":"Cambria"}}],"text":"f"}]}]}`

func TestDocFontRuns(t *testing.T) {
	var d pmNode
	if err := json.Unmarshal([]byte(fontRunsDoc), &d); err != nil {
		t.Fatal(err)
	}
	got := docFontRuns(&d)
	if len(got) != 2 || got["Cambria"] != 3 || got["Other"] != 1 {
		t.Errorf("runs: %v", got)
	}
}
