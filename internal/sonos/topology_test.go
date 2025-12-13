package sonos

import "testing"

func TestParseZoneGroupStateXML(t *testing.T) {
	payload := `
<ZoneGroupState>
  <ZoneGroups>
    <ZoneGroup Coordinator="RINCON_ABC1400" ID="RINCON_ABC1400:1">
      <ZoneGroupMember ZoneName="Kitchen" UUID="RINCON_ABC1400" Location="http://192.168.1.10:1400/xml/device_description.xml" Invisible="0" />
      <ZoneGroupMember ZoneName="Living Room" UUID="RINCON_DEF1400" Location="http://192.168.1.11:1400/xml/device_description.xml" Invisible="0" />
    </ZoneGroup>
  </ZoneGroups>
</ZoneGroupState>`

	top, err := parseZoneGroupStateXML(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(top.Groups) != 1 {
		t.Fatalf("groups: %d", len(top.Groups))
	}
	if top.Groups[0].Coordinator.IP != "192.168.1.10" {
		t.Fatalf("coordinator ip: %q", top.Groups[0].Coordinator.IP)
	}
	if ip, ok := top.CoordinatorIPForName("Living Room"); !ok || ip != "192.168.1.10" {
		t.Fatalf("CoordinatorIPForName: %v %q", ok, ip)
	}
}
