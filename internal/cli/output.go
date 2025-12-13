package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

const (
	formatPlain = "plain"
	formatJSON  = "json"
	formatTSV   = "tsv"
)

func normalizeFormat(format string) (string, error) {
	switch format {
	case formatPlain, formatJSON, formatTSV:
		return format, nil
	default:
		return "", errors.New("invalid --format (expected plain|json|tsv): " + format)
	}
}

func isJSON(flags *rootFlags) bool { return flags.Format == formatJSON }
func isTSV(flags *rootFlags) bool  { return flags.Format == formatTSV }

func writeJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeOK(cmd *cobra.Command, flags *rootFlags, action string, extra map[string]any) error {
	if !isJSON(flags) {
		return nil
	}
	out := map[string]any{
		"ok":     true,
		"action": action,
	}
	for k, v := range extra {
		out[k] = v
	}
	return writeJSON(cmd, out)
}

func writePlainLine(cmd *cobra.Command, flags *rootFlags, s string) {
	if isJSON(flags) {
		return
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), s)
}
