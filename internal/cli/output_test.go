package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestNormalizeFormat(t *testing.T) {
	for _, f := range []string{formatPlain, formatJSON, formatTSV} {
		got, err := normalizeFormat(f)
		if err != nil {
			t.Fatalf("normalizeFormat(%q): %v", f, err)
		}
		if got != f {
			t.Fatalf("normalizeFormat(%q): got %q", f, got)
		}
	}
	if _, err := normalizeFormat("nope"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteJSONLineAndPlainLine(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	flagsPlain := &rootFlags{Format: formatPlain}
	writePlainLine(cmd, flagsPlain, "hello")
	if got := out.String(); got != "hello\n" {
		t.Fatalf("plain output: %q", got)
	}

	out.Reset()
	if err := writeJSONLine(cmd, map[string]any{"ok": true}); err != nil {
		t.Fatalf("writeJSONLine: %v", err)
	}
	if got := out.String(); got == "" || got[0] != '{' {
		t.Fatalf("json output: %q", got)
	}

	out.Reset()
	flagsJSON := &rootFlags{Format: formatJSON}
	writePlainLine(cmd, flagsJSON, "ignored")
	if got := out.String(); got != "" {
		t.Fatalf("expected no output in json mode, got %q", got)
	}
}
