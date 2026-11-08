package plan

import (
	"encoding/json"
	"strings"
	"testing"
)

func labelPlan() Plan {
	return Plan{
		Version:  1,
		Floors:   []Floor{{ID: "floor", Name: "Floor"}},
		Cabinets: []Cabinet{{ID: "rack", Name: "Rack 1", ShortName: "R01", Floor: "floor", HeightU: rackUnits(12)}},
		Devices: []Device{
			{ID: "a", Name: "Switch A", Role: "switch", Mode: "fixed", Cabinet: "rack", RackUnit: rackUnits(10), HeightU: rackUnits(1)},
			{ID: "b", Name: "Switch B", Role: "switch", Mode: "fixed", Cabinet: "rack", RackUnit: rackUnits(12), HeightU: rackUnits(1)},
		},
		Ports: []Port{
			{ID: "ap", Device: "a", Name: "ether5", PoEMode: "unknown"},
			{ID: "bp", Device: "b", Name: "Port5", PoEMode: "unknown"},
		},
		Cables:       []Cable{{ID: "cable", A: Endpoint{Device: "a", Port: "ap"}, B: Endpoint{Device: "b", Port: "bp"}, Medium: "copper"}},
		PassThroughs: []PassThrough{},
	}
}

func TestCableLabelExamples(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Plan)
		want   string
	}{
		{"cabinet to cabinet", func(*Plan) {}, "R01:U10:P05-R01:U12:P05"},
		{"single digit unit is padded", func(p *Plan) { p.Devices[0].RackUnit = rackUnits(3) }, "R01:U03:P05-R01:U12:P05"},
		{"larger units are not truncated", func(p *Plan) { p.Cabinets[0].HeightU = rackUnits(125); p.Devices[0].RackUnit = rackUnits(125) }, "R01:U125:P05-R01:U12:P05"},
		{"cabinet to standalone", func(p *Plan) {
			p.Devices[0].RackUnit = rackUnits(12)
			p.Devices[1].Cabinet = ""
			p.Devices[1].RackUnit = nil
			p.Devices[1].Name = "PumpRoom"
			p.Ports[1].Name = "ether1"
		}, "R01:U12:P05-PumpRoom:P01"},
		{"standalone short name", func(p *Plan) {
			for i := range p.Devices {
				p.Devices[i].Cabinet = ""
				p.Devices[i].RackUnit = nil
				p.Ports[i].Name = "ether1"
			}
			p.Devices[0].Name = "PumpRoom"
			p.Devices[1].Name = "Desktop"
			p.Devices[1].ShortName = "ds01"
		}, "PumpRoom:P01-ds01:P01"},
		{"fallback cabinet name", func(p *Plan) { p.Cabinets[0].ShortName = "  " }, "Rack 1:U10:P05-Rack 1:U12:P05"},
		{"multi-unit starting unit", func(p *Plan) { p.Devices[0].HeightU = rackUnits(2) }, "R01:U10:P05-R01:U12:P05"},
		{"stable endpoint order", func(p *Plan) { p.Cables[0].A, p.Cables[0].B = p.Cables[0].B, p.Cables[0].A }, "R01:U12:P05-R01:U10:P05"},
		{"explicit fiber port number", func(p *Plan) { p.Ports[0].Name = "sfp-sfpplus1"; p.Ports[0].LabelNumber = rackUnits(25) }, "R01:U10:Psfp25-R01:U12:P05"},
		{"RouterOS fiber name", func(p *Plan) { p.Ports[0].Name = "sfp-sfpplus1" }, "R01:U10:Psfp01-R01:U12:P05"},
		{"SwOS fiber name", func(p *Plan) { p.Ports[0].Name = "SFP+2" }, "R01:U10:Psfp02-R01:U12:P05"},
		{"fiber and copper number coexist", func(p *Plan) {
			p.Ports[0].Name = "sfp1"
			p.Ports = append(p.Ports, Port{ID: "ether", Device: "a", Name: "ether1", PoEMode: "unknown"})
		}, "R01:U10:Psfp01-R01:U12:P05"},
		{"custom fiber port connector", func(p *Plan) {
			p.Ports[0].Name = "Uplink"
			p.Ports[0].Connector = "SFP+"
			p.Ports[0].LabelNumber = rackUnits(2)
		}, "R01:U10:Psfp02-R01:U12:P05"},
		{"passive terminal side", func(p *Plan) {
			p.Devices[0].Role = "patch_panel"
			p.Ports[0].Name = "1 front"
			p.Ports[0].Position = "1"
			p.Ports[0].Side = "front"
		}, "R01:U10:P01F-R01:U12:P05"},
		{"minimum two digit padding", func(p *Plan) { p.Ports[0].LabelNumber = rackUnits(125) }, "R01:U10:P125-R01:U12:P05"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := labelPlan()
			tc.mutate(&document)
			if err := document.Validate(); err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			preview := document.PreviewCableLabels()
			if len(preview) != 1 || preview[0].Label != tc.want || preview[0].Error != "" {
				t.Fatalf("want %q, got %#v", tc.want, preview)
			}
			after, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("preview mutated documentation")
			}
		})
	}
}

func TestCableLabelRefusesIncompleteOrAmbiguousInformation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Plan)
		want   string
	}{
		{"unknown port", func(p *Plan) { p.Cables[0].A.Port = ""; p.Cables[0].Draft = true }, "endpoint port is unknown"},
		{"unknown unit", func(p *Plan) { p.Devices[0].RackUnit = nil }, "starting rack unit"},
		{"ambiguous physical numbering", func(p *Plan) {
			p.Ports = append(p.Ports, Port{ID: "other", Device: "a", Name: "Port5", PoEMode: "unknown"})
		}, "P05 is ambiguous"},
		{"custom number cannot be guessed", func(p *Plan) { p.Ports[0].Name = "uplink" }, "needs a cable label number"},
		{"fiber aliases remain ambiguous", func(p *Plan) {
			p.Ports[0].Name = "sfp1"
			p.Ports = append(p.Ports, Port{ID: "alias", Device: "a", Name: "SFP+1", PoEMode: "unknown"})
		}, "Psfp01 is ambiguous"},
		{"duplicate endpoint names", func(p *Plan) {
			for i := range p.Devices {
				p.Devices[i].Cabinet = ""
				p.Devices[i].RackUnit = nil
				p.Devices[i].ShortName = "same"
			}
		}, "endpoint label"},
		{"label already in use", func(p *Plan) {
			p.Cables = append(p.Cables, Cable{ID: "other", A: Endpoint{Device: "a"}, B: Endpoint{Device: "b"}, Medium: "copper", Draft: true, Label: "R01:U10:P05-R01:U12:P05"})
		}, "already used"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := labelPlan()
			tc.mutate(&document)
			if err := document.Validate(); err != nil {
				t.Fatal(err)
			}
			preview := document.PreviewCableLabels()[0]
			if preview.Label != "" || !strings.Contains(preview.Error, tc.want) {
				t.Fatalf("wanted %q, got %#v", tc.want, preview)
			}
		})
	}
}

func TestRejectInvalidLabelMetadata(t *testing.T) {
	for _, mutate := range []func(*Plan){
		func(p *Plan) { p.Cabinets[0].ShortName = strings.Repeat("x", 101) },
		func(p *Plan) { p.Devices[0].ShortName = strings.Repeat("x", 101) },
		func(p *Plan) { p.Ports[0].LabelNumber = rackUnits(0) },
		func(p *Plan) { p.Ports[0].LabelNumber = rackUnits(10000) },
	} {
		document := labelPlan()
		mutate(&document)
		if err := document.Validate(); err == nil {
			t.Fatal("invalid cable-label metadata accepted")
		}
	}
}
