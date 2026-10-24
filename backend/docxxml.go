package main

// A small XML tree for reading .docx parts (docx.go). Elements are named
// by a short namespace key ("w", "r", "wp", "a", "pic", "mc", "m", "v",
// "wps", "wpg", "rel"; "" for namespaces the reader doesn't use) and their
// local name, so both the transitional and the strict OOXML namespaces read
// the same.

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	maxXMLElements = 3_000_000
	maxXMLDepth    = 256
)

var xmlNamespaces = map[string]string{
	"http://schemas.openxmlformats.org/wordprocessingml/2006/main":              "w",
	"http://purl.oclc.org/ooxml/wordprocessingml/main":                          "w",
	"http://schemas.openxmlformats.org/officeDocument/2006/relationships":       "r",
	"http://purl.oclc.org/ooxml/officeDocument/relationships":                   "r",
	"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing":    "wp",
	"http://purl.oclc.org/ooxml/drawingml/wordprocessingDrawing":                "wp",
	"http://schemas.openxmlformats.org/drawingml/2006/main":                     "a",
	"http://purl.oclc.org/ooxml/drawingml/main":                                 "a",
	"http://schemas.openxmlformats.org/drawingml/2006/picture":                  "pic",
	"http://purl.oclc.org/ooxml/drawingml/picture":                              "pic",
	"http://schemas.openxmlformats.org/markup-compatibility/2006":               "mc",
	"http://schemas.openxmlformats.org/officeDocument/2006/math":                "m",
	"http://purl.oclc.org/ooxml/officeDocument/math":                            "m",
	"urn:schemas-microsoft-com:vml":                                             "v",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingShape":         "wps",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingGroup":         "wpg",
	"http://schemas.openxmlformats.org/package/2006/relationships":              "rel",
	"http://schemas.openxmlformats.org/package/2006/content-types":              "ct",
	"http://schemas.openxmlformats.org/drawingml/2006/chart":                    "c",
	"http://schemas.openxmlformats.org/drawingml/2006/diagram":                  "dgm",
	"http://schemas.microsoft.com/office/drawing/2010/main":                     "a14",
	"http://schemas.microsoft.com/office/word/2010/wordml":                      "w14",
	"http://schemas.openxmlformats.org/officeDocument/2006/sharedTypes":         "s",
	"http://schemas.openxmlformats.org/officeDocument/2006/extended-properties": "ep",
}

type xEl struct {
	ns    string
	name  string
	attrs []xml.Attr
	kids  []*xEl
	text  string
}

var errXMLTooBig = errors.New("the file has too much XML")

// parseXML reads a whole part into a tree.
func parseXML(data []byte) (*xEl, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		if strings.EqualFold(label, "utf-8") || strings.EqualFold(label, "utf8") {
			return input, nil
		}
		return nil, fmt.Errorf("unsupported XML encoding %q", label)
	}
	root := &xEl{}
	stack := []*xEl{root}
	count := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			count++
			if count > maxXMLElements || len(stack) > maxXMLDepth {
				return nil, errXMLTooBig
			}
			el := &xEl{ns: xmlNamespaces[t.Name.Space], name: t.Name.Local, attrs: t.Attr}
			parent := stack[len(stack)-1]
			parent.kids = append(parent.kids, el)
			stack = append(stack, el)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			cur := stack[len(stack)-1]
			if cur != root && len(cur.kids) == 0 {
				cur.text += string(t)
			}
		}
	}
	if len(root.kids) == 0 {
		return nil, errors.New("empty XML part")
	}
	return root.kids[0], nil
}

func (e *xEl) is(ns, name string) bool { return e != nil && e.ns == ns && e.name == name }

// child is the first direct child with that name (nil if none).
func (e *xEl) child(ns, name string) *xEl {
	if e == nil {
		return nil
	}
	for _, k := range e.kids {
		if k.ns == ns && k.name == name {
			return k
		}
	}
	return nil
}

func (e *xEl) childrenNamed(ns, name string) []*xEl {
	if e == nil {
		return nil
	}
	var out []*xEl
	for _, k := range e.kids {
		if k.ns == ns && k.name == name {
			out = append(out, k)
		}
	}
	return out
}

// find is the first element with that name below e, depth first.
func (e *xEl) find(ns, name string) *xEl {
	if e == nil {
		return nil
	}
	for _, k := range e.kids {
		if k.ns == ns && k.name == name {
			return k
		}
		if f := k.find(ns, name); f != nil {
			return f
		}
	}
	return nil
}

// walk visits every element below e; fn returning false skips that
// element's children.
func (e *xEl) walk(fn func(*xEl) bool) {
	if e == nil {
		return
	}
	for _, k := range e.kids {
		if fn(k) {
			k.walk(fn)
		}
	}
}

// attr is an attribute by local name, outside the relationships namespace
// (w:val, w:id, Target…).
func (e *xEl) attr(name string) string {
	if e == nil {
		return ""
	}
	for _, a := range e.attrs {
		if a.Name.Local == name && xmlNamespaces[a.Name.Space] != "r" {
			return a.Value
		}
	}
	return ""
}

func (e *xEl) hasAttr(name string) bool {
	if e == nil {
		return false
	}
	for _, a := range e.attrs {
		if a.Name.Local == name && xmlNamespaces[a.Name.Space] != "r" {
			return true
		}
	}
	return false
}

// rattr is an attribute in the relationships namespace (r:id, r:embed).
func (e *xEl) rattr(name string) string {
	if e == nil {
		return ""
	}
	for _, a := range e.attrs {
		if a.Name.Local == name && xmlNamespaces[a.Name.Space] == "r" {
			return a.Value
		}
	}
	return ""
}

// wOn reads a Word on/off element: present with no val, "1", "true" or
// "on" is on.
func wOn(e *xEl) bool {
	switch strings.ToLower(e.attr("val")) {
	case "0", "false", "off", "none":
		return false
	}
	return true
}

// txt is an element's text ("" for a missing element).
func (e *xEl) txt() string {
	if e == nil {
		return ""
	}
	return e.text
}
