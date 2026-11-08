package plan

import (
	"encoding/json"
	"strings"
	"testing"
)

func rackUnits(value int) *int { return &value }

func rackPlan() Plan {
	document := Seed()
	document.Cabinets[0].HeightU = rackUnits(12)
	document.Devices[0].HeightU = rackUnits(1)
	document.Devices[0].RackUnit = rackUnits(1)
	document.Devices[1].HeightU = rackUnits(2)
	document.Devices[1].RackUnit = rackUnits(3)
	return document
}

func TestRackMountingValidation(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*Plan)
		errorText string
	}{
		{"non-overlapping equipment", func(*Plan) {}, ""},
		{"adjacent equipment", func(p *Plan) { p.Devices[1].RackUnit = rackUnits(2) }, ""},
		{"cabinet height unknown", func(p *Plan) { p.Cabinets[0].HeightU = nil }, ""},
		{"unassigned device with unknown height", func(p *Plan) { p.Devices[1].HeightU = nil; p.Devices[1].RackUnit = nil }, ""},
		{"same unit in separate cabinets", func(p *Plan) { p.Devices[1].Cabinet = p.Cabinets[1].ID; p.Devices[1].RackUnit = rackUnits(1) }, ""},
		{"last unit fits", func(p *Plan) { p.Devices[1].RackUnit = rackUnits(11) }, ""},
		{"zero cabinet height", func(p *Plan) { p.Cabinets[0].HeightU = rackUnits(0) }, "height must be"},
		{"negative device height", func(p *Plan) { p.Devices[1].HeightU = rackUnits(-1) }, "height and starting unit"},
		{"zero starting unit", func(p *Plan) { p.Devices[1].RackUnit = rackUnits(0) }, "height and starting unit"},
		{"excessive cabinet height", func(p *Plan) { p.Cabinets[0].HeightU = rackUnits(1001) }, "height must be"},
		{"missing equipment height", func(p *Plan) { p.Devices[1].HeightU = nil }, "document its height"},
		{"missing cabinet", func(p *Plan) { p.Devices[1].Cabinet = "" }, "assign a cabinet"},
		{"overlapping ranges", func(p *Plan) { p.Devices[0].HeightU = rackUnits(4) }, "overlap at U3"},
		{"shared boundary unit", func(p *Plan) { p.Devices[0].HeightU = rackUnits(3) }, "overlap at U3"},
		{"extends past cabinet", func(p *Plan) { p.Devices[1].RackUnit = rackUnits(12) }, "beyond cabinet"},
		{"shrink occupied cabinet", func(p *Plan) { p.Cabinets[0].HeightU = rackUnits(3) }, "beyond cabinet"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			document := rackPlan()
			test.mutate(&document)
			err := document.Validate()
			if test.errorText == "" {
				if err != nil {
					t.Fatalf("valid rack layout rejected: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.errorText) {
				t.Fatalf("expected %q, got %v", test.errorText, err)
			}
		})
	}
}

func TestLegacyPlanWithoutRackFields(t *testing.T) {
	encoded, err := json.Marshal(Seed())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "heightU") || strings.Contains(string(encoded), "rackUnit") {
		t.Fatal("unknown rack values should be omitted")
	}
	var document Plan
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	if err := document.Validate(); err != nil {
		t.Fatalf("legacy plan rejected: %v", err)
	}
	if document.Cabinets[0].HeightU != nil || document.Devices[0].RackUnit != nil || document.Devices[0].HeightU != nil {
		t.Fatal("unknown rack dimensions were invented")
	}
}
