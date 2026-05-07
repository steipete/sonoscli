package streamproxy

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

const ICYMetaInterval = 8192

func writeICY(w io.Writer, r io.Reader, title, sourceURL string) (bool, error) {
	meta := icyBlock(title, sourceURL)
	buf := make([]byte, ICYMetaInterval)
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return false, writeErr
			}
			if n == ICYMetaInterval {
				if _, writeErr := w.Write(meta); writeErr != nil {
					return false, writeErr
				}
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return true, nil
		}
		return false, err
	}
}

func icyBlock(title, sourceURL string) []byte {
	text := fmt.Sprintf("StreamTitle='%s';StreamUrl='%s';", escapeICY(title), escapeICY(sourceURL))
	l := (len(text) + 15) / 16
	if l > 255 {
		text = text[:255*16]
		l = 255
	}
	out := make([]byte, 1+l*16)
	out[0] = byte(l)
	copy(out[1:], text)
	return out
}

func escapeICY(s string) string {
	return strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(s)
}
