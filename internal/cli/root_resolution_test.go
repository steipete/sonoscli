package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/appconfig"
	"github.com/STop211650/sonoscli/internal/sonos"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestExecute_Version(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	oldLoad := loadAppConfig
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
		loadAppConfig = oldLoad
	})

	loadAppConfig = func() (appconfig.Config, error) {
		return appconfig.Config{DefaultRoom: "Office", Format: "plain"}, nil
	}

	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Args = []string{"sonos", "--version"}

	err := Execute()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(buf.String(), "sonos ") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestResolveTargetCoordinatorIP_IPAndName(t *testing.T) {
	oldNew := newSonosClient
	oldDiscover := sonosDiscover
	t.Cleanup(func() {
		newSonosClient = oldNew
		sonosDiscover = oldDiscover
	})

	zgs := `<ZoneGroupState><ZoneGroups><ZoneGroup Coordinator="RINCON_COORD1400" ID="RINCON_COORD1400:1">` +
		`<ZoneGroupMember ZoneName="Living Room" UUID="RINCON_COORD1400" Location="http://10.0.0.1:1400/xml/device_description.xml" Invisible="0" />` +
		`<ZoneGroupMember ZoneName="Office" UUID="RINCON_OFFICE1400" Location="http://10.0.0.2:1400/xml/device_description.xml" Invisible="0" />` +
		`</ZoneGroup></ZoneGroups></ZoneGroupState>`

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.Header.Get("SOAPACTION"), "ZoneGroupTopology:1#GetZoneGroupState") {
			return httpResponse(500, ""), nil
		}
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1">
      <ZoneGroupState><![CDATA[`+zgs+`]]></ZoneGroupState>
    </u:GetZoneGroupStateResponse>
  </s:Body>
</s:Envelope>`), nil
	})

	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{
			IP:   ip,
			Port: 1400,
			HTTP: &http.Client{Timeout: timeout, Transport: rt},
		}
	}
	sonosDiscover = func(ctx context.Context, opts sonos.DiscoverOptions) ([]sonos.Device, error) {
		return []sonos.Device{{IP: "10.0.0.1", Name: "Living Room"}}, nil
	}

	ctx := context.Background()

	flags := &rootFlags{IP: "10.0.0.2", Timeout: time.Second}
	ip, err := resolveTargetCoordinatorIP(ctx, flags)
	if err != nil {
		t.Fatalf("resolveTargetCoordinatorIP(ip): %v", err)
	}
	if ip != "10.0.0.1" {
		t.Fatalf("expected coordinator 10.0.0.1, got %q", ip)
	}

	flags2 := &rootFlags{Name: "Office", Timeout: time.Second}
	ip2, err := resolveTargetCoordinatorIP(ctx, flags2)
	if err != nil {
		t.Fatalf("resolveTargetCoordinatorIP(name): %v", err)
	}
	if ip2 != "10.0.0.1" {
		t.Fatalf("expected coordinator 10.0.0.1, got %q", ip2)
	}

	c, err := coordinatorClient(ctx, flags2)
	if err != nil {
		t.Fatalf("coordinatorClient: %v", err)
	}
	if c.IP != "10.0.0.1" {
		t.Fatalf("unexpected client ip: %q", c.IP)
	}
}
