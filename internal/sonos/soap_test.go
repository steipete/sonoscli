package sonos

import (
	"strings"
	"testing"
)

func TestBuildSOAPEnvelope_SortsArgs(t *testing.T) {
	env := string(buildSOAPEnvelope("urn:schemas-upnp-org:service:AVTransport:1", "Play", map[string]string{
		"Speed":      "1",
		"InstanceID": "0",
	}))

	// InstanceID should appear before Speed due to sorted keys.
	if !strings.Contains(env, "<InstanceID>0</InstanceID><Speed>1</Speed>") {
		t.Fatalf("expected args to be sorted, got: %s", env)
	}
}

func TestParseSOAPResponse(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:GetVolumeResponse xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1">
      <CurrentVolume>25</CurrentVolume>
    </u:GetVolumeResponse>
  </s:Body>
</s:Envelope>`)

	out, err := parseSOAPResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if out["CurrentVolume"] != "25" {
		t.Fatalf("CurrentVolume: %q", out["CurrentVolume"])
	}
}

func TestParseUPnPError(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <s:Fault>
      <faultcode>s:Client</faultcode>
      <faultstring>UPnPError</faultstring>
      <detail>
        <UPnPError xmlns="urn:schemas-upnp-org:control-1-0">
          <errorCode>701</errorCode>
          <errorDescription>Transition not available</errorDescription>
        </UPnPError>
      </detail>
    </s:Fault>
  </s:Body>
</s:Envelope>`)

	e, ok := parseUPnPError(raw)
	if !ok {
		t.Fatalf("expected ok")
	}
	if e.Code != "701" {
		t.Fatalf("code: %q", e.Code)
	}
	if e.Description == "" {
		t.Fatalf("expected description")
	}
}
