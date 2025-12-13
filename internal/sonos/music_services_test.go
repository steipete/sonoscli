package sonos

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestListAvailableServices(t *testing.T) {
	t.Parallel()

	servicesXML := `<Services SchemaVersion="1">
  <Service Id="2311" Name="Spotify" Version="1.1" Uri="http://example" SecureUri="https://example" ContainerType="MService" Capabilities="513">
    <Policy Auth="DeviceLink" />
    <Presentation>
      <PresentationMap Version="2" Uri="https://pmap" />
      <Strings Version="1" Uri="https://strings" />
    </Presentation>
  </Service>
</Services>`
	escaped := strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(servicesXML)

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.Header.Get("SOAPACTION"), "MusicServices:1#ListAvailableServices") {
			t.Fatalf("SOAPACTION: %q", r.Header.Get("SOAPACTION"))
		}
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:ListAvailableServicesResponse xmlns:u="urn:schemas-upnp-org:service:MusicServices:1">
      <AvailableServiceDescriptorList>`+escaped+`</AvailableServiceDescriptorList>
    </u:ListAvailableServicesResponse>
  </s:Body>
</s:Envelope>`), nil
	})

	c := &Client{
		IP: "192.0.2.1",
		HTTP: &http.Client{
			Timeout:   time.Second,
			Transport: rt,
		},
	}

	services, err := c.ListAvailableServices(context.Background())
	if err != nil {
		t.Fatalf("ListAvailableServices: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("len: %d", len(services))
	}
	if services[0].Name != "Spotify" || services[0].ID != "2311" || services[0].Auth != MusicServiceAuthDeviceLink {
		t.Fatalf("unexpected service: %+v", services[0])
	}
	// 2311*256+7
	if services[0].ServiceType != "591623" {
		t.Fatalf("ServiceType: %q", services[0].ServiceType)
	}
}

func TestListAvailableServices_MissingPayloadErrors(t *testing.T) {
	t.Parallel()

	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return httpResponse(200, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:ListAvailableServicesResponse xmlns:u="urn:schemas-upnp-org:service:MusicServices:1">
      <AvailableServiceDescriptorList>   </AvailableServiceDescriptorList>
    </u:ListAvailableServicesResponse>
  </s:Body>
</s:Envelope>`), nil
	})

	c := &Client{IP: "192.0.2.1", HTTP: &http.Client{Timeout: time.Second, Transport: rt}}
	if _, err := c.ListAvailableServices(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}
