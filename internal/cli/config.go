package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/appconfig"
)

var newConfigStore = func() (appconfig.Store, error) { return appconfig.NewDefaultStore() }

func newConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage local CLI defaults",
		Long:  "Stores small, local defaults under your user config directory (e.g. ~/.config/sonoscli/config.json).",
	}
	cmd.AddCommand(newConfigGetCmd(flags))
	cmd.AddCommand(newConfigSetCmd(flags))
	cmd.AddCommand(newConfigUnsetCmd(flags))
	cmd.AddCommand(newConfigPathCmd(flags))
	return cmd
}

func newConfigPathCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newConfigStore()
			if err != nil {
				return err
			}
			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{"path": s.Path()})
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), s.Path())
			return nil
		},
	}
}

func newConfigGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Get current config (or one key)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newConfigStore()
			if err != nil {
				return err
			}
			cfg, err := s.Load()
			if err != nil {
				return err
			}

			if len(args) == 0 {
				if isJSON(flags) {
					return writeJSON(cmd, cfg)
				}
				printConfigPlain(cmd, cfg)
				return nil
			}

			key := strings.TrimSpace(args[0])
			val, ok := getConfigKey(cfg, key)
			if !ok {
				return errors.New("unknown key: " + key)
			}
			if isJSON(flags) {
				return writeJSON(cmd, map[string]any{key: val})
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", key, val)
			return nil
		},
	}
}

func newConfigSetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newConfigStore()
			if err != nil {
				return err
			}
			cfg, err := s.Load()
			if err != nil {
				return err
			}

			key := strings.TrimSpace(args[0])
			value := args[1]
			cfg, err = setConfigKey(cfg, key, value)
			if err != nil {
				return err
			}
			if err := s.Save(cfg); err != nil {
				return err
			}
			return writeOK(cmd, flags, "config.set", map[string]any{"key": key, "value": value})
		},
	}
}

func newConfigUnsetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Unset a config key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newConfigStore()
			if err != nil {
				return err
			}
			cfg, err := s.Load()
			if err != nil {
				return err
			}

			key := strings.TrimSpace(args[0])
			cfg, err = unsetConfigKey(cfg, key)
			if err != nil {
				return err
			}
			if err := s.Save(cfg); err != nil {
				return err
			}
			return writeOK(cmd, flags, "config.unset", map[string]any{"key": key})
		},
	}
}

func printConfigPlain(cmd *cobra.Command, cfg appconfig.Config) {
	entries := map[string]string{
		"defaultRoom": cfg.DefaultRoom,
		"format":      cfg.Format,
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, entries[k])
	}
}

func getConfigKey(cfg appconfig.Config, key string) (string, bool) {
	switch key {
	case "defaultRoom":
		return cfg.DefaultRoom, true
	case "format":
		return cfg.Format, true
	default:
		return "", false
	}
}

func setConfigKey(cfg appconfig.Config, key, value string) (appconfig.Config, error) {
	switch key {
	case "defaultRoom":
		cfg.DefaultRoom = value
		return cfg, nil
	case "format":
		value = strings.TrimSpace(value)
		switch strings.ToLower(value) {
		case "", "plain", "json", "tsv":
			cfg.Format = value
		default:
			return appconfig.Config{}, errors.New("invalid format (expected plain|json|tsv): " + value)
		}
		return cfg, nil
	default:
		return appconfig.Config{}, errors.New("unknown key: " + key)
	}
}

func unsetConfigKey(cfg appconfig.Config, key string) (appconfig.Config, error) {
	switch key {
	case "defaultRoom":
		cfg.DefaultRoom = ""
		return cfg, nil
	case "format":
		cfg.Format = ""
		return cfg, nil
	default:
		return appconfig.Config{}, errors.New("unknown key: " + key)
	}
}
