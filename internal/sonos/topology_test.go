package sonos

import "testing"

func TestParseZoneGroupStateXML(t *testing.T) {
	payload := `
<ZoneGroupState>
  <ZoneGroups>
    <ZoneGroup Coordinator="RINCON_ABC1400" ID="RINCON_ABC1400:1">
      <ZoneGroupMember ZoneName="Kitchen" UUID="RINCON_ABC1400" Location="http://192.168.1.10:1400/xml/device_description.xml" Invisible="0" />
      <ZoneGroupMember ZoneName="Living Room" UUID="RINCON_DEF1400" Location="http://192.168.1.11:1400/xml/device_description.xml" Invisible="0">
        <Satellite ZoneName="Master Bathroom" UUID="RINCON_MBA1400" Location="http://192.168.1.12:1400/xml/device_description.xml" Invisible="0" />
      </ZoneGroupMember>
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
	if uuid, ok := top.CoordinatorUUIDForName("Living Room"); !ok || uuid != "RINCON_ABC1400" {
		t.Fatalf("CoordinatorUUIDForName: %v %q", ok, uuid)
	}
	if g, ok := top.GroupForName("living room"); !ok || g.ID != "RINCON_ABC1400:1" {
		t.Fatalf("GroupForName: %v %+v", ok, g)
	}
	if g, ok := top.GroupForIP("192.168.1.11"); !ok || g.Coordinator.Name != "Kitchen" {
		t.Fatalf("GroupForIP: %v %+v", ok, g)
	}
	if mem, ok := top.FindByName("Master Bathroom"); !ok || mem.IP != "192.168.1.12" {
		t.Fatalf("satellite parse: %v %+v", ok, mem)
	}
}

func TestParseZoneGroupStateXML_NamePrefersVisibleOverSatellite(t *testing.T) {
	payload := `
	<ZoneGroupState>
	  <ZoneGroups>
	    <ZoneGroup Coordinator="RINCON_ABC1400" ID="RINCON_ABC1400:1">
	      <ZoneGroupMember ZoneName="Kitchen" UUID="RINCON_ABC1400" Location="http://192.168.1.10:1400/xml/device_description.xml" Invisible="0" />
	      <ZoneGroupMember ZoneName="Living Room" UUID="RINCON_DEF1400" Location="http://192.168.1.11:1400/xml/device_description.xml" Invisible="0">
	        <Satellite ZoneName="Living Room" UUID="RINCON_SAT1400" Location="http://192.168.1.12:1400/xml/device_description.xml" Invisible="1" />
	      </ZoneGroupMember>
	    </ZoneGroup>
	  </ZoneGroups>
	</ZoneGroupState>`

	top, err := parseZoneGroupStateXML(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	mem, ok := top.FindByName("Living Room")
	if !ok {
		t.Fatalf("expected FindByName to succeed")
	}
	if mem.IP != "192.168.1.11" || !mem.IsVisible {
		t.Fatalf("expected visible member (192.168.1.11) but got %+v", mem)
	}
}

func TestTopology_FindByIP_AndCoordinatorUUIDForIP(t *testing.T) {
	payload := `
<ZoneGroupState>
  <ZoneGroups>
    <ZoneGroup Coordinator="RINCON_ABC1400" ID="RINCON_ABC1400:1">
      <ZoneGroupMember ZoneName="Office" UUID="RINCON_ABC1400" Location="http://192.168.1.10:1400/xml/device_description.xml" Invisible="0" />
      <ZoneGroupMember ZoneName="Kitchen" UUID="RINCON_DEF1400" Location="http://192.168.1.11:1400/xml/device_description.xml" Invisible="0" />
    </ZoneGroup>
  </ZoneGroups>
</ZoneGroupState>`

	top, err := parseZoneGroupStateXML(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if mem, ok := top.FindByIP("192.168.1.11"); !ok || mem.Name != "Kitchen" {
		t.Fatalf("FindByIP: ok=%v mem=%+v", ok, mem)
	}
	if uuid, ok := top.CoordinatorUUIDForIP("192.168.1.11"); !ok || uuid != "RINCON_ABC1400" {
		t.Fatalf("CoordinatorUUIDForIP: ok=%v uuid=%q", ok, uuid)
	}
	if _, ok := top.CoordinatorUUIDForIP("192.168.1.222"); ok {
		t.Fatalf("expected CoordinatorUUIDForIP to fail for unknown ip")
	}
}
