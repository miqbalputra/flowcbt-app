// Sanitasi rich text (PRD 12): allowlist tag & atribut, anti-XSS pada HTML soal.
package questionbank

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// Tag & atribut yang diizinkan pada body_html / stimulus / penjelasan.
var allowedTags = map[string]map[string]bool{
	"p":      {"class": true},
	"br":     {},
	"b":      {},
	"strong": {},
	"i":      {},
	"em":     {},
	"u":      {},
	"ul":     {"class": true},
	"ol":     {"class": true},
	"li":     {"class": true},
	"table":  {"class": true},
	"tr":     {"class": true},
	"td":     {"colspan": true, "rowspan": true, "class": true},
	"th":     {"colspan": true, "rowspan": true, "class": true},
	"code":   {"class": true},
	"pre":    {"class": true},
	"sub":    {},
	"sup":    {},
	"a":      {"href": true, "title": true},
	"span":   {"class": true},
}

// Elemen berbahaya dibuang BERSAMA isinya.
var strippedTags = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true,
	"embed": true, "form": true, "noscript": true, "base": true, "svg": true, "math": true,
}

// Sanitize memfilter HTML: tag/atribut di luar allowlist dilepas, URL hanya
// http/https/#. Rich text siswa tidak boleh membawa skrip apa pun.
func Sanitize(src string) (string, error) {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return "", err
	}
	walk(doc)
	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func walk(n *html.Node) {
	child := n.FirstChild
	for child != nil {
		next := child.NextSibling
		if child.Type == html.ElementNode {
			tag := strings.ToLower(child.Data)
			if strippedTags[tag] {
				n.RemoveChild(child)
				child = next
				continue
			}
			child.Attr = filterAttrs(tag, child.Attr)
		}
		walk(child)
		child = next
	}
}

func filterAttrs(tag string, attrs []html.Attribute) []html.Attribute {
	allowed := allowedTags[tag]
	if allowed == nil {
		return nil
	}
	out := attrs[:0]
	for _, a := range attrs {
		key := strings.ToLower(a.Key)
		if !allowed[key] {
			continue
		}
		if key == "href" {
			lv := strings.ToLower(strings.TrimSpace(a.Val))
			if !strings.HasPrefix(lv, "http://") && !strings.HasPrefix(lv, "https://") && !strings.HasPrefix(lv, "#") {
				continue
			}
		}
		out = append(out, html.Attribute{Key: key, Val: a.Val})
	}
	return out
}
