package sonos

import (
	"bytes"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"strings"
)

// ParseEvent decodes a UPnP event propertyset payload into a flat map.
// If a LastChange property is present, it is decoded and flattened.
func ParseEvent(payload []byte) (map[string]string, error) {
	out := map[string]string{}

	dec := xml.NewDecoder(bytes.NewReader(payload))
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return nil, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if strings.EqualFold(start.Name.Local, "LastChange") {
			var raw string
			if err := dec.DecodeElement(&raw, &start); err != nil {
				return nil, err
			}
			inner := html.UnescapeString(strings.TrimSpace(raw))
			for k, v := range parseLastChange(inner) {
				out[k] = v
			}
		}
	}
}

func parseLastChange(innerXML string) map[string]string {
	out := map[string]string{}
	dec := xml.NewDecoder(strings.NewReader(innerXML))
	var inInstance bool
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "InstanceID" || t.Name.Local == "QueueID" {
				inInstance = true
				continue
			}
			if !inInstance {
				continue
			}
			var val string
			var channel string
			for _, a := range t.Attr {
				switch strings.ToLower(a.Name.Local) {
				case "val":
					val = a.Value
				case "channel":
					channel = a.Value
				}
			}
			if val == "" {
				continue
			}
			key := camelToSnake(t.Name.Local)
			if channel != "" {
				key = key + "_" + strings.ToLower(channel)
			}
			out[key] = val
		case xml.EndElement:
			if t.Name.Local == "InstanceID" || t.Name.Local == "QueueID" {
				inInstance = false
			}
		}
	}
}

func camelToSnake(s string) string {
	// Small, local helper; we don't want to depend on cli utils.
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rune(s[i-1])
			if prev >= 'a' && prev <= 'z' {
				b.WriteByte('_')
			}
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
