package sonos

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
)

type Member struct {
	Name          string `json:"name"`
	IP            string `json:"ip"`
	UUID          string `json:"uuid"`
	Location      string `json:"location"`
	IsVisible     bool   `json:"isVisible"`
	IsCoordinator bool   `json:"isCoordinator"`
}

type Group struct {
	ID          string   `json:"id"`
	Coordinator Member   `json:"coordinator"`
	Members     []Member `json:"members"`
}

type Topology struct {
	Groups      []Group           `json:"groups"`
	ByName      map[string]Member `json:"-"`
	ByIP        map[string]Member `json:"-"`
	byUUID      map[string]Member
	coordByUUID map[string]Member
}

func (c *Client) GetTopology(ctx context.Context) (Topology, error) {
	resp, err := c.soapCall(ctx, controlZoneGroupTopology, urnZoneGroupTopology, "GetZoneGroupState", nil)
	if err != nil {
		return Topology{}, err
	}
	zgs := resp["ZoneGroupState"]
	if zgs == "" {
		return Topology{}, fmt.Errorf("zone group state missing in response")
	}
	return parseZoneGroupStateXML(zgs)
}

type zgsEnvelope struct {
	ZoneGroups *struct {
		Groups []zgsGroup `xml:"ZoneGroup"`
	} `xml:"ZoneGroups"`
	Groups []zgsGroup `xml:"ZoneGroup"`
}

type zgsGroup struct {
	Coordinator string      `xml:"Coordinator,attr"`
	ID          string      `xml:"ID,attr"`
	Members     []zgsMember `xml:"ZoneGroupMember"`
}

type zgsMember struct {
	ZoneName  string `xml:"ZoneName,attr"`
	Location  string `xml:"Location,attr"`
	UUID      string `xml:"UUID,attr"`
	Invisible string `xml:"Invisible,attr"`
}

func parseZoneGroupStateXML(payload string) (Topology, error) {
	var env zgsEnvelope
	if err := xml.Unmarshal([]byte(payload), &env); err != nil {
		return Topology{}, err
	}

	groups := env.Groups
	if env.ZoneGroups != nil && len(env.ZoneGroups.Groups) > 0 {
		groups = env.ZoneGroups.Groups
	}

	t := Topology{
		ByName:      map[string]Member{},
		ByIP:        map[string]Member{},
		byUUID:      map[string]Member{},
		coordByUUID: map[string]Member{},
	}

	for _, g := range groups {
		members := make([]Member, 0, len(g.Members))
		var coordinator Member
		for _, m := range g.Members {
			ip, err := hostToIP(m.Location)
			if err != nil {
				continue
			}
			mem := Member{
				Name:      m.ZoneName,
				IP:        ip,
				UUID:      m.UUID,
				Location:  m.Location,
				IsVisible: m.Invisible != "1",
			}
			mem.IsCoordinator = mem.UUID == g.Coordinator
			if mem.IsCoordinator {
				coordinator = mem
			}
			members = append(members, mem)
			if mem.Name != "" {
				t.ByName[mem.Name] = mem
			}
			t.ByIP[mem.IP] = mem
			if mem.UUID != "" {
				t.byUUID[mem.UUID] = mem
			}
		}

		// Fallback: if we didn't find coordinator by UUID, pick first member.
		if coordinator.UUID == "" && len(members) > 0 {
			coordinator = members[0]
			coordinator.IsCoordinator = true
		}
		if coordinator.UUID != "" {
			t.coordByUUID[coordinator.UUID] = coordinator
		}

		t.Groups = append(t.Groups, Group{
			ID:          g.ID,
			Coordinator: coordinator,
			Members:     members,
		})
	}

	return t, nil
}

func (t Topology) FindByName(name string) (Member, bool) {
	mem, ok := t.ByName[name]
	return mem, ok
}

func (t Topology) FindByIP(ip string) (Member, bool) {
	mem, ok := t.ByIP[ip]
	return mem, ok
}

func (t Topology) GroupForIP(ip string) (Group, bool) {
	for _, g := range t.Groups {
		for _, m := range g.Members {
			if m.IP == ip {
				return g, true
			}
		}
	}
	return Group{}, false
}

func (t Topology) GroupForName(name string) (Group, bool) {
	mem, ok := t.FindByName(name)
	if !ok {
		// Try case-insensitive match
		for k, v := range t.ByName {
			if strings.EqualFold(k, name) {
				mem = v
				ok = true
				break
			}
		}
	}
	if !ok {
		return Group{}, false
	}
	return t.GroupForIP(mem.IP)
}

func (t Topology) CoordinatorIPFor(ip string) (string, bool) {
	// Find the group that contains this IP and return its coordinator IP.
	for _, g := range t.Groups {
		for _, m := range g.Members {
			if m.IP == ip {
				if g.Coordinator.IP != "" {
					return g.Coordinator.IP, true
				}
				return ip, true
			}
		}
	}
	return "", false
}

func (t Topology) CoordinatorIPForName(name string) (string, bool) {
	mem, ok := t.FindByName(name)
	if !ok {
		// Try case-insensitive match
		for k, v := range t.ByName {
			if strings.EqualFold(k, name) {
				mem = v
				ok = true
				break
			}
		}
	}
	if !ok {
		return "", false
	}
	return t.CoordinatorIPFor(mem.IP)
}

func (t Topology) CoordinatorUUIDForIP(ip string) (string, bool) {
	g, ok := t.GroupForIP(ip)
	if !ok {
		return "", false
	}
	if g.Coordinator.UUID == "" {
		return "", false
	}
	return g.Coordinator.UUID, true
}

func (t Topology) CoordinatorUUIDForName(name string) (string, bool) {
	g, ok := t.GroupForName(name)
	if !ok {
		return "", false
	}
	if g.Coordinator.UUID == "" {
		return "", false
	}
	return g.Coordinator.UUID, true
}
