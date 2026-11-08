package plan

import (
	"errors"
	"reflect"
	"testing"
)

func identityPlan() Plan {
	document := Seed()
	document.Interfaces = []NetworkInterface{{ID: NewID(), Device: document.Devices[4].ID, Name: "Wi-Fi", Domain: "home", MACAddresses: []string{"02:11:22:33:44:55"}}}
	return document
}

func TestDomainAndMACAreTheMatchingKey(t *testing.T) {
	document := identityPlan()
	other := NetworkInterface{ID: NewID(), Device: document.Devices[5].ID, Name: "Guest Wi-Fi", Domain: "guest", MACAddresses: []string{"02:11:22:33:44:55"}}
	document.Interfaces = append(document.Interfaces, other)
	document.Interfaces[0].MACAddresses = append(document.Interfaces[0].MACAddresses, "02:11:22:33:44:66")
	if err := document.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"02-11-22-33-44-55", "0211.2233.4466"} {
		match, err := document.MatchInterface(" HOME ", alias)
		if err != nil || match == nil || match.ID != document.Interfaces[0].ID {
			t.Fatalf("MAC alias failed to match: %#v %v", match, err)
		}
	}
	match, err := document.MatchInterface("guest", "02:11:22:33:44:55")
	if err != nil || match == nil || match.ID != other.ID {
		t.Fatal("network domain was ignored")
	}
	match, err = document.MatchInterface("other", "02:11:22:33:44:55")
	if err != nil || match != nil {
		t.Fatal("unknown domain should not match")
	}
	document.Interfaces[1].Domain = "home"
	if err := document.Validate(); err == nil {
		t.Fatal("duplicate domain/MAC accepted")
	}
	if _, err := document.MatchInterface("home", "02:11:22:33:44:55"); !errors.Is(err, ErrAmbiguousIdentity) {
		t.Fatal("ambiguous owner was selected")
	}
}

func TestIdentityNormalizationDoesNotMutateWorkingCopy(t *testing.T) {
	original := identityPlan()
	original.Interfaces[0].Domain = " HOME "
	original.Interfaces[0].MACAddresses = []string{"02-11-22-33-44-55", ""}
	normalized := original.NormalizeIdentities()
	if err := normalized.Validate(); err != nil {
		t.Fatal(err)
	}
	if normalized.Interfaces[0].Domain != "home" || !reflect.DeepEqual(normalized.Interfaces[0].MACAddresses, []string{"02:11:22:33:44:55"}) {
		t.Fatal("identity was not canonicalized")
	}
	if original.Interfaces[0].Domain != " HOME " || len(original.Interfaces[0].MACAddresses) != 2 {
		t.Fatal("normalization mutated the working copy")
	}
}

func TestRejectInvalidInterfaceIdentities(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Plan)
	}{
		{"unknown device", func(p *Plan) { p.Interfaces[0].Device = "missing" }},
		{"empty name", func(p *Plan) { p.Interfaces[0].Name = " " }},
		{"empty domain", func(p *Plan) { p.Interfaces[0].Domain = "" }},
		{"invalid domain", func(p *Plan) { p.Interfaces[0].Domain = "two words" }},
		{"invalid MAC", func(p *Plan) { p.Interfaces[0].MACAddresses = []string{"invalid"} }},
		{"multicast MAC", func(p *Plan) { p.Interfaces[0].MACAddresses = []string{"01:00:5E:00:00:01"} }},
		{"zero MAC", func(p *Plan) { p.Interfaces[0].MACAddresses = []string{"00:00:00:00:00:00"} }},
		{"eight byte MAC", func(p *Plan) { p.Interfaces[0].MACAddresses = []string{"02:11:22:33:44:55:66:77"} }},
		{"duplicate alias", func(p *Plan) {
			p.Interfaces[0].MACAddresses = append(p.Interfaces[0].MACAddresses, "02-11-22-33-44-55")
		}},
		{"duplicate entity ID", func(p *Plan) { p.Interfaces[0].ID = p.Devices[0].ID }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := identityPlan()
			tc.mutate(&p)
			if err := p.Validate(); err == nil {
				t.Fatal("invalid interface identity accepted")
			}
		})
	}
}

func TestInterfacePortReferenceBelongsToItsDevice(t *testing.T) {
	document := identityPlan()
	physical := Port{ID: NewID(), Device: document.Interfaces[0].Device, Name: "ether1", PoEMode: "unknown"}
	document.Ports = append(document.Ports, physical)
	document.Interfaces[0].Port = physical.ID
	if err := document.Validate(); err != nil {
		t.Fatal(err)
	}
	virtual := document.Interfaces[0]
	virtual.ID = NewID()
	virtual.Name = "Management VLAN"
	virtual.MACAddresses = []string{"02:11:22:33:44:77"}
	document.Interfaces = append(document.Interfaces, virtual)
	if err := document.Validate(); err != nil {
		t.Fatalf("virtual interfaces must be allowed to share a port: %v", err)
	}
	document.Interfaces[0].Port = "missing"
	if err := document.Validate(); err == nil {
		t.Fatal("dangling port reference accepted")
	}
	document.Interfaces[0].Port = document.Ports[0].ID
	if err := document.Validate(); err == nil {
		t.Fatal("port on a different device accepted")
	}
	document.Interfaces[0].Port = ""
	if err := document.Validate(); err != nil {
		t.Fatalf("unlinked wireless interface rejected: %v", err)
	}
}
