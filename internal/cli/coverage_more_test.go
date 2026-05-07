package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/sonoscli/internal/streamproxy"
)

func TestTargetCommandsFailBeforeNetworkWithoutTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cmd  func(*rootFlags) *cobra.Command
		args []string
	}{
		{name: "mute", cmd: newMuteCmd, args: []string{"get"}},
		{name: "volume-get", cmd: newVolumeCmd, args: []string{"get"}},
		{name: "volume-set", cmd: newVolumeCmd, args: []string{"set", "10"}},
		{name: "play", cmd: newPlayCmd},
		{name: "pause", cmd: newPauseCmd},
		{name: "stop", cmd: newStopCmd},
		{name: "next", cmd: newNextCmd},
		{name: "prev", cmd: newPrevCmd},
		{name: "linein", cmd: newLineInCmd, args: []string{"Kitchen"}},
		{name: "tv", cmd: newTVCmd},
		{name: "play-uri", cmd: newPlayURICmd, args: []string{"http://example.com/audio.mp3"}},
		{name: "play-url", cmd: newPlayURLCmd, args: []string{"--resolver", "direct", "http://example.com/audio.mp3"}},
		{name: "youtube", cmd: newPlayYouTubeCmd, args: []string{"https://www.youtube.com/watch?v=abc"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			flags := &rootFlags{Timeout: time.Millisecond}
			cmd := tc.cmd(flags)
			cmd.SetOut(newDiscardWriter())
			cmd.SetErr(newDiscardWriter())
			cmd.SetArgs(tc.args)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			err := cmd.ExecuteContext(context.Background())
			if err == nil {
				t.Fatalf("expected target error")
			}
		})
	}
}

func TestPlayURLCmdSurfacesTargetAndLaunchErrors(t *testing.T) {
	flags := &rootFlags{Name: "Office", Timeout: 2 * time.Second}
	cmd := newPlayURLCmd(flags)

	origTarget := newPlayURLTarget
	origLaunch := launchStreamDaemon
	origLocalIP := chooseLocalIP
	t.Cleanup(func() {
		newPlayURLTarget = origTarget
		launchStreamDaemon = origLaunch
		chooseLocalIP = origLocalIP
	})

	newPlayURLTarget = func(ctx context.Context, flags *rootFlags) (playURLTarget, error) {
		return playURLTarget{}, errors.New("target boom")
	}
	chooseLocalIP = func(remoteIP string) (string, error) {
		return "192.168.0.25", nil
	}
	launchStreamDaemon = func(ctx context.Context, cfg streamproxy.ServerConfig, publicURL string) (streamDaemonInfo, error) {
		return streamDaemonInfo{}, errors.New("launch boom")
	}

	cmd.SetOut(newDiscardWriter())
	cmd.SetErr(newDiscardWriter())
	cmd.SetArgs([]string{"--resolver", "direct", "https://example.com/episode.mp3"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "target boom") {
		t.Fatalf("expected target error, got %v", err)
	}
}
