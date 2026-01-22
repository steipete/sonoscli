package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/STop211650/sonoscli/internal/sonos"
)

type statusClient interface {
	GetDeviceDescription(ctx context.Context) (sonos.Device, error)
	GetTransportInfo(ctx context.Context) (sonos.TransportInfo, error)
	GetPositionInfo(ctx context.Context) (sonos.PositionInfo, error)
	GetVolume(ctx context.Context) (int, error)
	GetMute(ctx context.Context) (bool, error)
}

var newStatusClient = func(ctx context.Context, flags *rootFlags) (statusClient, error) {
	return coordinatorClient(ctx, flags)
}

type statusOutput struct {
	Device      sonos.Device        `json:"device"`
	Transport   sonos.TransportInfo `json:"transport"`
	Position    sonos.PositionInfo  `json:"position"`
	NowPlaying  *sonos.DIDLItem     `json:"nowPlaying,omitempty"`
	AlbumArtURL string              `json:"albumArtURL,omitempty"`
	Volume      int                 `json:"volume"`
	Mute        bool                `json:"mute"`
}

func newStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:          "status",
		Aliases:      []string{"now"},
		Short:        "Show current playback status",
		Long:         "Prints coordinator status (transport state, track URI, time, volume/mute). Parses TrackMetaData when available to show title/artist/album/album art. Use --format json for machine-readable output.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTarget(flags); err != nil {
				return err
			}
			ctx := cmd.Context()
			c, err := newStatusClient(ctx, flags)
			if err != nil {
				return err
			}

			dev, _ := c.GetDeviceDescription(ctx)
			transport, _ := c.GetTransportInfo(ctx)
			position, _ := c.GetPositionInfo(ctx)
			vol, _ := c.GetVolume(ctx)
			mute, _ := c.GetMute(ctx)

			var nowPlaying *sonos.DIDLItem
			var albumArtURL string
			if np, ok := sonos.ParseNowPlaying(position.TrackMeta); ok {
				nowPlaying = &np
				albumArtURL = sonos.AlbumArtURL(dev.IP, np.AlbumArtURI)
			}

			out := statusOutput{
				Device:      dev,
				Transport:   transport,
				Position:    position,
				NowPlaying:  nowPlaying,
				AlbumArtURL: albumArtURL,
				Volume:      vol,
				Mute:        mute,
			}

			if isJSON(flags) {
				return writeJSON(cmd, out)
			}

			if isTSV(flags) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "speaker\t%s\n", dev.Name)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ip\t%s\n", dev.IP)
				if dev.UDN != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "udn\t%s\n", dev.UDN)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "state\t%s\n", transport.State)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "track\t%s\n", position.Track)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "uri\t%s\n", position.TrackURI)
				if nowPlaying != nil {
					if nowPlaying.Title != "" {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "title\t%s\n", nowPlaying.Title)
					}
					if nowPlaying.Artist != "" {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "artist\t%s\n", nowPlaying.Artist)
					}
					if nowPlaying.Album != "" {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "album\t%s\n", nowPlaying.Album)
					}
					if nowPlaying.AlbumArtURI != "" {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "album_art\t%s\n", albumArtURL)
					}
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "time\t%s\n", position.RelTime)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "duration\t%s\n", position.TrackDuration)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "volume\t%d\n", vol)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "mute\t%v\n", mute)
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Speaker:\t%s (%s)\n", dev.Name, dev.IP)
			if dev.UDN != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "UDN:\t\t%s\n", dev.UDN)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "State:\t\t%s\n", transport.State)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Track:\t\t%s\n", position.Track)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "URI:\t\t%s\n", position.TrackURI)
			if nowPlaying != nil {
				if nowPlaying.Title != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Title:\t\t%s\n", nowPlaying.Title)
				}
				if nowPlaying.Artist != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Artist:\t\t%s\n", nowPlaying.Artist)
				}
				if nowPlaying.Album != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Album:\t\t%s\n", nowPlaying.Album)
				}
				if nowPlaying.AlbumArtURI != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "AlbumArt:\t%s\n", albumArtURL)
				}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Time:\t\t%s / %s\n", position.RelTime, position.TrackDuration)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Volume:\t\t%d\n", vol)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Mute:\t\t%v\n", mute)
			return nil
		},
	}
}
