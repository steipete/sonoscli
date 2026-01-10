package sonos

import (
	"testing"
)

func TestParseAppleMusicRef(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantRef AppleMusicRef
	}{
		{
			name:   "album URL",
			input:  "https://music.apple.com/us/album/1989-taylors-version/1713845538",
			wantOK: true,
			wantRef: AppleMusicRef{
				Kind:        AppleMusicAlbum,
				ID:          "1713845538",
				Canonical:   "https://music.apple.com/us/album/1989-taylors-version/1713845538",
				ServiceNums: []int{52231},
			},
		},
		{
			name:   "song URL with track ID",
			input:  "https://music.apple.com/us/album/shake-it-off/1713845538?i=1713845694",
			wantOK: true,
			wantRef: AppleMusicRef{
				Kind:        AppleMusicSong,
				ID:          "1713845694",
				Canonical:   "https://music.apple.com/us/album/shake-it-off/1713845538?i=1713845694",
				ServiceNums: []int{52231},
			},
		},
		{
			name:   "playlist URL",
			input:  "https://music.apple.com/us/playlist/todays-hits/pl.f4d106fed2bd41149aaacabb233eb5eb",
			wantOK: true,
			wantRef: AppleMusicRef{
				Kind:        AppleMusicPlaylist,
				ID:          "pl.f4d106fed2bd41149aaacabb233eb5eb",
				Canonical:   "https://music.apple.com/us/playlist/todays-hits/pl.f4d106fed2bd41149aaacabb233eb5eb",
				ServiceNums: []int{52231},
			},
		},
		{
			name:   "station URL",
			input:  "https://music.apple.com/us/station/pure-pop/ra.985485420",
			wantOK: true,
			wantRef: AppleMusicRef{
				Kind:        AppleMusicStation,
				ID:          "ra.985485420",
				Canonical:   "https://music.apple.com/us/station/pure-pop/ra.985485420",
				ServiceNums: []int{52231},
			},
		},
		{
			name:   "different country code",
			input:  "https://music.apple.com/gb/album/abbey-road/401186199",
			wantOK: true,
			wantRef: AppleMusicRef{
				Kind:        AppleMusicAlbum,
				ID:          "401186199",
				Canonical:   "https://music.apple.com/gb/album/abbey-road/401186199",
				ServiceNums: []int{52231},
			},
		},
		{
			name:   "empty input",
			input:  "",
			wantOK: false,
		},
		{
			name:   "invalid URL",
			input:  "https://spotify.com/track/abc123",
			wantOK: false,
		},
		{
			name:   "not a music URL",
			input:  "https://apple.com/music",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, ok := ParseAppleMusicRef(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseAppleMusicRef() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if !tt.wantOK {
				return
			}
			if ref.Kind != tt.wantRef.Kind {
				t.Errorf("Kind = %v, want %v", ref.Kind, tt.wantRef.Kind)
			}
			if ref.ID != tt.wantRef.ID {
				t.Errorf("ID = %v, want %v", ref.ID, tt.wantRef.ID)
			}
			if ref.Canonical != tt.wantRef.Canonical {
				t.Errorf("Canonical = %v, want %v", ref.Canonical, tt.wantRef.Canonical)
			}
			if len(ref.ServiceNums) != len(tt.wantRef.ServiceNums) {
				t.Errorf("ServiceNums length = %v, want %v", len(ref.ServiceNums), len(tt.wantRef.ServiceNums))
			} else {
				for i, sn := range ref.ServiceNums {
					if sn != tt.wantRef.ServiceNums[i] {
						t.Errorf("ServiceNums[%d] = %v, want %v", i, sn, tt.wantRef.ServiceNums[i])
					}
				}
			}
		})
	}
}

func TestAppleMusicSonosMagic(t *testing.T) {
	tests := []struct {
		kind         AppleMusicKind
		wantClass    string
		wantIDPrefix string
	}{
		{AppleMusicAlbum, "object.container.album.musicAlbum", "0004206c"},
		{AppleMusicPlaylist, "object.container.playlistContainer", "1006206c"},
		{AppleMusicSong, "object.item.audioItem.musicTrack", "10032020"},
		{AppleMusicStation, "object.item.audioItem.audioBroadcast", "000c206c"},
		{AppleMusicKind("unknown"), "", ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			class, prefix := appleMusicSonosMagic(tt.kind)
			if class != tt.wantClass {
				t.Errorf("class = %v, want %v", class, tt.wantClass)
			}
			if prefix != tt.wantIDPrefix {
				t.Errorf("prefix = %v, want %v", prefix, tt.wantIDPrefix)
			}
		})
	}
}
