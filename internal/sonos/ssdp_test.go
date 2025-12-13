package sonos

import "testing"

func TestParseSSDPResponse(t *testing.T) {
	resp := "HTTP/1.1 200 OK\r\n" +
		"CACHE-CONTROL: max-age = 1800\r\n" +
		"EXT:\r\n" +
		"LOCATION: http://192.168.1.50:1400/xml/device_description.xml\r\n" +
		"SERVER: Linux UPnP/1.0 Sonos/83.1-12345 (ZPS3)\r\n" +
		"ST: urn:schemas-upnp-org:device:ZonePlayer:1\r\n" +
		"USN: uuid:RINCON_00000000000001400::urn:schemas-upnp-org:device:ZonePlayer:1\r\n" +
		"\r\n"

	parsed, ok := parseSSDPResponse([]byte(resp))
	if !ok {
		t.Fatalf("expected ok")
	}
	if parsed.Location != "http://192.168.1.50:1400/xml/device_description.xml" {
		t.Fatalf("location: %q", parsed.Location)
	}
	if parsed.ST == "" || parsed.USN == "" {
		t.Fatalf("expected headers")
	}
}
