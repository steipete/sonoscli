package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/appconfig"
)

func TestHelpWorksForAllCommands(t *testing.T) {
	orig := loadAppConfig
	t.Cleanup(func() { loadAppConfig = orig })
	loadAppConfig = func() (appconfig.Config, error) { return appconfig.Config{}.Normalize(), nil }

	root, _, err := newRootCmd()
	if err != nil {
		t.Fatalf("newRootCmd: %v", err)
	}

	paths := allCommandPaths(root)
	for _, path := range paths {
		path := path
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			// Use a fresh command tree per run to avoid cross-test state (flags, etc).
			root, _, err := newRootCmd()
			if err != nil {
				t.Fatalf("newRootCmd: %v", err)
			}

			var out captureWriter
			root.SetOut(&out)
			root.SetErr(&out)
			root.SilenceErrors = true
			root.SilenceUsage = true
			root.SetArgs(append(path, "--help"))

			if err := root.ExecuteContext(context.Background()); err != nil {
				t.Fatalf("help failed for %v: %v\noutput:\n%s", path, err, out.String())
			}
			if got := out.String(); strings.TrimSpace(got) == "" {
				t.Fatalf("expected help output for %v", path)
			}
		})
	}
}

func TestVersionFlagWorks(t *testing.T) {
	orig := loadAppConfig
	t.Cleanup(func() { loadAppConfig = orig })
	loadAppConfig = func() (appconfig.Config, error) { return appconfig.Config{}.Normalize(), nil }

	root, _, err := newRootCmd()
	if err != nil {
		t.Fatalf("newRootCmd: %v", err)
	}
	var out captureWriter
	root.SetOut(&out)
	root.SetErr(&out)
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs([]string{"--version"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("version failed: %v\noutput:\n%s", err, out.String())
	}
	if got := out.String(); !strings.Contains(got, "sonos "+Version) {
		t.Fatalf("unexpected version output: %q", got)
	}
}

func allCommandPaths(root *cobra.Command) [][]string {
	// Return all command paths (excluding root) as argv slices (without --help).
	var out [][]string
	var walk func(cmd *cobra.Command, prefix []string)
	walk = func(cmd *cobra.Command, prefix []string) {
		for _, child := range cmd.Commands() {
			if child.Hidden {
				continue
			}
			p := append(append([]string{}, prefix...), child.Name())
			out = append(out, p)
			walk(child, p)
		}
	}
	walk(root, nil)
	return out
}
