package sonos

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

type DIDLItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URI         string `json:"uri"`
	Class       string `json:"class,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtURI string `json:"albumArtURI,omitempty"`
	ResMD       string `json:"resMD,omitempty"`
}

func ParseDIDLItems(didlXML string) ([]DIDLItem, error) {
	didlXML = strings.TrimSpace(didlXML)
	if didlXML == "" {
		return nil, nil
	}

	dec := xml.NewDecoder(bytes.NewReader([]byte(didlXML)))
	var items []DIDLItem

	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return items, nil
			}
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != "item" && se.Name.Local != "container" {
			continue
		}
		it, err := parseDIDLItem(dec, se)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
}

func parseDIDLItem(dec *xml.Decoder, start xml.StartElement) (DIDLItem, error) {
	var it DIDLItem
	for _, a := range start.Attr {
		if strings.EqualFold(a.Name.Local, "id") {
			it.ID = strings.TrimSpace(a.Value)
		}
	}

	var current string
	for {
		tok, err := dec.Token()
		if err != nil {
			return it, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			current = strings.ToLower(t.Name.Local)
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return it, nil
			}
			current = ""
		case xml.CharData:
			if current == "" {
				continue
			}
			val := strings.TrimSpace(string(t))
			if val == "" {
				continue
			}
			switch current {
			case "title":
				if it.Title == "" {
					it.Title = val
				}
			case "res":
				if it.URI == "" {
					it.URI = val
				}
			case "resmd":
				if it.ResMD == "" {
					it.ResMD = val
				}
			case "class":
				if it.Class == "" {
					it.Class = val
				}
			case "artist", "creator":
				if it.Artist == "" {
					it.Artist = val
				}
			case "album":
				if it.Album == "" {
					it.Album = val
				}
			case "albumarturi":
				if it.AlbumArtURI == "" {
					it.AlbumArtURI = val
				}
			}
		}
	}
}
