package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestUploadEditorPicture(t *testing.T) {
	_, h, base, vol := editorSetup(t)
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{200, 30, 30, 255})
	var pngData, jpgData, gifData bytes.Buffer
	png.Encode(&pngData, img)
	jpeg.Encode(&jpgData, img, nil)
	gif.Encode(&gifData, img, nil)

	post := func(url, name string, data []byte) (int, string) {
		body, ct := form(t, nil, "file", name, data)
		rec := do(t, h, "POST", url, body, ct)
		return rec.Code, rec.Body.String()
	}
	pictures := fmt.Sprintf("%s/books/%d/pictures", base, vol)
	sid := strings.TrimPrefix(base, "/api/s/")

	// Each kind is kept with the extension of what it is, whatever it was called.
	for name, c := range map[string]struct {
		data []byte
		ext  string
		mime string
	}{"a photo.png": {pngData.Bytes(), ".png", "image/png"}, "scan.dat": {jpgData.Bytes(), ".jpg", "image/jpeg"}, "lies.png": {gifData.Bytes(), ".gif", "image/gif"}} {
		code, out := post(pictures, name, c.data)
		if code != 200 {
			t.Fatalf("%s: %d %s", name, code, out)
		}
		var res struct{ Src string }
		json.Unmarshal([]byte(out), &res)
		prefix := "/uploads/s/" + sid + "/editor/"
		if !strings.HasPrefix(res.Src, prefix) || !strings.HasSuffix(res.Src, c.ext) {
			t.Fatalf("%s: src %q", name, res.Src)
		}
		pic := do(t, h, "GET", res.Src, nil, "")
		if pic.Code != 200 || !bytes.Equal(pic.Body.Bytes(), c.data) || pic.Header().Get("Content-Security-Policy") != "sandbox" {
			t.Errorf("%s: served %d", name, pic.Code)
		}
		// The address is what an image node holds, and the text saves.
		ch := addChapter(t, h, base, "With a picture", vol)
		doc := fmt.Sprintf(`{"type":"doc","content":[{"type":"image","attrs":{"src":%q,"alt":"","width":null,"align":"center"}}]}`, res.Src)
		if code, _ := putText(t, h, base, ch, doc, 0); code != 200 {
			t.Errorf("%s: saving the text: %d", name, code)
		}
		if got := getText(t, h, base, ch); !strings.Contains(string(got.Doc), `"`+res.Src+`"`) {
			t.Errorf("%s: read back %s", name, got.Doc)
		}
	}
	// Two uploads never share a file.
	_, a := post(pictures, "a.png", pngData.Bytes())
	_, b := post(pictures, "a.png", pngData.Bytes())
	if a == b {
		t.Errorf("same address twice: %s", a)
	}
	// Book 0 takes pictures too.
	if code, out := post(base+"/books/0/pictures", "a.png", pngData.Bytes()); code != 200 {
		t.Errorf("book 0: %d %s", code, out)
	}

	for name, c := range map[string]struct {
		url  string
		data []byte
		want int
	}{
		"text":         {pictures, []byte("hello"), 400},
		"svg":          {pictures, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), 400},
		"html as png":  {pictures, []byte("<html><script>alert(1)</script></html>"), 400},
		"empty":        {pictures, nil, 400},
		"unknown book": {base + "/books/999/pictures", pngData.Bytes(), 404},
	} {
		if code, out := post(c.url, "x.png", c.data); code != c.want {
			t.Errorf("%s: %d %s", name, code, out)
		}
	}
	body, ct := form(t, nil, "", "", nil)
	if rec := do(t, h, "POST", pictures, body, ct); rec.Code != 400 {
		t.Errorf("no file: %d", rec.Code)
	}

	// Too large (the limit lowered for the test).
	defer func(old int64) { maxEditorPicture = old }(maxEditorPicture)
	maxEditorPicture = int64(pngData.Len() - 1)
	if code, _ := post(pictures, "a.png", pngData.Bytes()); code != 413 {
		t.Errorf("too large: %d", code)
	}
}
