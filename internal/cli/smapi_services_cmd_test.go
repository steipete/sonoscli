package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/STop211650/sonoscli/internal/sonos"
)

func TestSMAPIServicesCmdPlainAndJSON(t *testing.T) {
	t.Parallel()

	serviceList := `<Services SchemaVersion="1">
  <Service Id="2311" Name="Spotify" Version="1.1" Uri="http://example" SecureUri="http://example/smapi" ContainerType="MService" Capabilities="513">
    <Policy Auth="DeviceLink" />
  </Service>
  <Service Id="163" Name="TuneIn" Version="1.1" Uri="http://example" SecureUri="http://example/smapi" ContainerType="MService" Capabilities="513">
    <Policy Auth="Anonymous" />
  </Service>
</Services>`

	mux := http.NewServeMux()
	mux.HandleFunc("/MusicServices/Control", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if !strings.Contains(r.Header.Get("SOAPACTION"), "MusicServices:1#ListAvailableServices") {
			t.Fatalf("unexpected SOAPACTION: %q", r.Header.Get("SOAPACTION"))
		}
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:ListAvailableServicesResponse xmlns:u="urn:schemas-upnp-org:service:MusicServices:1">
      <AvailableServiceDescriptorList><![CDATA[` + serviceList + `]]></AvailableServiceDescriptorList>
    </u:ListAvailableServicesResponse>
  </s:Body>
</s:Envelope>`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	oldNew := newSonosClient
	t.Cleanup(func() { newSonosClient = oldNew })
	newSonosClient = func(ip string, timeout time.Duration) *sonos.Client {
		return &sonos.Client{
			IP:   u.Hostname(),
			Port: port,
			HTTP: srv.Client(),
		}
	}

	// Plain output
	{
		flags := &rootFlags{IP: u.Hostname(), Timeout: 2 * time.Second, Format: formatPlain}
		cmd := newSMAPIServicesCmd(flags)
		var out captureWriter
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("plain: %v", err)
		}
		// Header + rows; sorted by name.
		if !strings.Contains(out.String(), "NAME") || !strings.Contains(out.String(), "AUTH") || !strings.Contains(out.String(), "ID") {
			t.Fatalf("unexpected output: %q", out.String())
		}
		if !strings.Contains(out.String(), "Spotify") || !strings.Contains(out.String(), "TuneIn") {
			t.Fatalf("unexpected output: %q", out.String())
		}
	}

	// JSON output
	{
		flags := &rootFlags{IP: u.Hostname(), Timeout: 2 * time.Second, Format: formatJSON}
		cmd := newSMAPIServicesCmd(flags)
		var out captureWriter
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("json: %v", err)
		}
		if !strings.Contains(out.String(), "\"services\"") || !strings.Contains(out.String(), "\"Spotify\"") {
			t.Fatalf("unexpected json output: %q", out.String())
		}
	}
}
