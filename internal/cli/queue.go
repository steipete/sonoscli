package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

type queueClient interface {
	ListQueue(ctx context.Context, start, count int) (sonos.QueuePage, error)
	ClearQueue(ctx context.Context) error
	RemoveQueuePosition(ctx context.Context, position int) error
	PlayQueuePosition(ctx context.Context, position int) error
}

var newQueueClient = func(ctx context.Context, flags *rootFlags) (queueClient, error) {
	return coordinatorClient(ctx, flags)
}

func newQueueCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage the playback queue",
	}
	cmd.AddCommand(newQueueListCmd(flags))
	cmd.AddCommand(newQueueClearCmd(flags))
	cmd.AddCommand(newQueuePlayCmd(flags))
	cmd.AddCommand(newQueueRemoveCmd(flags))
	return cmd
}

func newQueueListCmd(flags *rootFlags) *cobra.Command {
	var start int
	var limit int

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List queue entries",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			ctx := cmd.Context()
			c, err := newQueueClient(ctx, flags)
			if err != nil {
				return err
			}

			page, err := c.ListQueue(ctx, start, limit)
			if err != nil {
				return err
			}

			if isJSON(flags) {
				return writeJSON(cmd, page)
			}
			if isTSV(flags) {
				for _, qi := range page.Items {
					title := qi.Item.Title
					if title == "" {
						title = qi.Item.ID
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\n", qi.Position, title, qi.Item.URI)
				}
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintf(w, "POS\tTITLE\tURI\n")
			for _, qi := range page.Items {
				title := qi.Item.Title
				if title == "" {
					title = qi.Item.ID
				}
				_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", qi.Position, title, qi.Item.URI)
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&start, "start", 0, "Starting index (0-based)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results to return")
	return cmd
}

func newQueueClearCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "clear",
		Short:        "Clear the queue",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			ctx := cmd.Context()
			c, err := newQueueClient(ctx, flags)
			if err != nil {
				return err
			}
			if err := c.ClearQueue(ctx); err != nil {
				return err
			}
			return writeOK(cmd, flags, "queue.clear", nil)
		},
	}
	return cmd
}

func newQueuePlayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "play <pos>",
		Short:        "Play a queue entry (1-based)",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			pos, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.New("pos must be an integer (1-based)")
			}
			ctx := cmd.Context()
			c, err := newQueueClient(ctx, flags)
			if err != nil {
				return err
			}
			if err := c.PlayQueuePosition(ctx, pos); err != nil {
				return err
			}
			return writeOK(cmd, flags, "queue.play", map[string]any{"pos": pos})
		},
	}
	return cmd
}

func newQueueRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "remove <pos>",
		Short:        "Remove a queue entry (1-based)",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			pos, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.New("pos must be an integer (1-based)")
			}
			ctx := cmd.Context()
			c, err := newQueueClient(ctx, flags)
			if err != nil {
				return err
			}
			if err := c.RemoveQueuePosition(ctx, pos); err != nil {
				return err
			}
			return writeOK(cmd, flags, "queue.remove", map[string]any{"pos": pos})
		},
	}
	return cmd
}
