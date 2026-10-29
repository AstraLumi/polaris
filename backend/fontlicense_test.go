package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestBookFontsLicenses(t *testing.T) {
	mgr, h := newTestServer(t)
	var askedMu sync.Mutex
	asked := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/css":
			fam := r.URL.Query().Get("family")
			askedMu.Lock()
			asked[fam]++
			askedMu.Unlock()
			switch fam {
			case "Band Face":
				fmt.Fprintf(w, "@font-face { src: url(http://%s/band.ttf) format('truetype'); }", r.Host)
			case "Far Face":
				http.Error(w, "busy", 500)
			default:
				http.NotFound(w, r)
			}
		case "/band.ttf":
			w.Write(buildSfnt(testFont{family: "Band Face", style: "Regular", weight: 400,
				copyright:  "Copyright 2016 The Band Face Project Authors",
				license:    "This Font Software is licensed under the SIL Open Font License, Version 1.1.",
				licenseURL: "https://openfontlicense.org"}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	mgr.fonts.googleBase = srv.URL + "/css"

	uploadFonts(t, h, nil, map[string][]byte{
		"a.ttf": buildSfnt(testFont{family: "Test Serif", style: "Regular", weight: 400}),
		"b.ttf": buildSfnt(testFont{family: "Shop Script", style: "Regular", weight: 400}),
	})
	list := listFonts(t, h)
	put := func(id int64, body string) {
		if rec := do(t, h, "PUT", fmt.Sprintf("/api/fonts/%d", id), strings.NewReader(body), "application/json"); rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	put(familyNamed(list, "Test Serif").ID, `{"license_kind":"commercial","uses":{"print":true,"ebook":false,"web":false,"commercial":true}}`)
	put(familyNamed(list, "Shop Script").ID, `{"license_kind":"other","source":"https://shop.example/script","uses":{"print":true,"ebook":true,"web":false,"commercial":true}}`)

	id, vol, _ := fontStory(t, h, "License Story")
	settings := `{"settings":{"page_type":"a5","styles":[
		{"id":"normal","name":"Normal","kind":"paragraph","builtin":true,"text":{"font":"test serif"}},
		{"id":"quote","name":"Quote","kind":"paragraph","builtin":true,"text":{"font":"Shop Script"}},
		{"id":"cap","name":"Caption","kind":"paragraph","text":{"font":"Georgia"}},
		{"id":"far","name":"Far","kind":"paragraph","text":{"font":"Far Face"}}],
		"layout":{"header":{"odd":{"center":"{chapter}","font":"Band Face"}}}}}`
	if rec := do(t, h, "PUT", fmt.Sprintf("/api/s/%s/books/%d", id, vol), strings.NewReader(settings), "application/json"); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	get := func() map[string]bookFontOut {
		rec := do(t, h, "GET", fmt.Sprintf("/api/s/%s/books/%d/fonts", id, vol), nil, "")
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body.String())
		}
		var out []bookFontOut
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		m := map[string]bookFontOut{}
		for _, o := range out {
			m[strings.ToLower(o.Family)] = o
		}
		return m
	}
	by := get()
	if o := by["test serif"]; o.Source != "library" || o.Verdict != "check" || o.Reason != "uses" || o.Allowed == nil || o.Allowed.Ebook || strings.Join(o.Styles, ",") != "Normal" {
		t.Errorf("test serif: %+v", o)
	}
	if o := by["shop script"]; o.Source != "library" || o.Verdict != "ok" || o.SourceURL != "https://shop.example/script" {
		t.Errorf("shop script: %+v", o)
	}
	if o := by["georgia"]; o.Source != "system" || o.Verdict != "warn" || strings.Join(o.Styles, ",") != "Caption" {
		t.Errorf("georgia: %+v", o)
	}
	o := by["band face"]
	if o.Source != "google" || o.Verdict != "ok" || o.LicenseID != "OFL-1.1" || o.Bands != 1 || !strings.Contains(o.Copyright, "2016") || o.LicenseURL != "https://openfontlicense.org" {
		t.Errorf("band face: %+v", o)
	}
	if o := by["mark face"]; o.Source != "unknown" || o.Verdict != "check" || o.Chapters != 1 {
		t.Errorf("mark face (only in the text): %+v", o)
	}
	if o := by["far face"]; o.Source != "google" || o.Verdict != "check" || o.LicenseID != "" {
		t.Errorf("far face (Google unreachable): %+v", o)
	}
	if asked["Georgia"] != 0 || asked["Shop Script"] != 0 {
		t.Errorf("system and library fonts were asked of Google: %v", asked)
	}
	get()
	if asked["Band Face"] != 1 {
		t.Errorf("Band Face asked %d times, want 1 (kept)", asked["Band Face"])
	}
}
