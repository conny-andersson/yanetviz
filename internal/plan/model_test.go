package plan

import "testing"

func TestRejectInvalidDocumentation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Plan)
	}{
		{"unknown floor", func(p *Plan) { p.Devices[4].Floor = "missing" }},
		{"mobile placement", func(p *Plan) { p.Devices[0].Mode = "mobile" }},
		{"out of bounds position", func(p *Plan) { p.Devices[4].X = 1.1 }},
		{"wrong endpoint device", func(p *Plan) { p.Cables[0].A.Port = p.Ports[0].ID }},
		{"occupied terminal", func(p *Plan) { p.Cables[0].B = Endpoint{Device: p.Ports[0].Device, Port: p.Ports[0].ID} }},
		{"complete cable with unknown port", func(p *Plan) { p.Cables[0].Draft = false }},
		{"duplicate cable", func(p *Plan) { c := p.Cables[0]; c.ID = NewID(); c.A, c.B = c.B, c.A; p.Cables = append(p.Cables, c) }},
		{"duplicate port name", func(p *Plan) { p.Ports[1].Name = p.Ports[0].Name }},
		{"untrusted SVG upload", func(p *Plan) { p.Floors[0].Background = "data:image/svg+xml;base64,PHN2Zy8+" }},
		{"external background", func(p *Plan) { p.Floors[0].Background = "https://example.com/image.png" }},
		{"null collection", func(p *Plan) { p.Ports = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := Seed()
			tc.mutate(&document)
			if err := document.Validate(); err == nil {
				t.Fatal("invalid documentation was accepted")
			}
		})
	}
}

func TestPassivePassThroughDoesNotConsumeExternalAttachment(t *testing.T) {
	document := Seed()
	panel := Device{ID: NewID(), Name: "Basement panel", Role: "patch_panel", Mode: "fixed"}
	document.Devices = append(document.Devices, panel)
	front := Port{ID: NewID(), Device: panel.ID, Name: "1 front", Position: "1", Side: "front", PoEMode: "unknown"}
	rear := Port{ID: NewID(), Device: panel.ID, Name: "1 rear", Position: "1", Side: "rear", PoEMode: "unknown"}
	document.Ports = append(document.Ports, front, rear)
	document.PassThroughs = append(document.PassThroughs, PassThrough{ID: NewID(), A: front.ID, B: rear.ID})
	document.Cables = append(document.Cables,
		Cable{ID: NewID(), A: Endpoint{Device: document.Devices[0].ID}, B: Endpoint{Device: panel.ID, Port: front.ID}, Medium: "copper", Draft: true},
		Cable{ID: NewID(), A: Endpoint{Device: panel.ID, Port: rear.ID}, B: Endpoint{Device: document.Devices[4].ID}, Medium: "copper", Draft: true},
	)
	if err := document.Validate(); err != nil {
		t.Fatalf("valid passive wiring rejected: %v", err)
	}
	document.PassThroughs = append(document.PassThroughs, PassThrough{ID: NewID(), A: front.ID, B: rear.ID})
	if err := document.Validate(); err == nil {
		t.Fatal("duplicate internal mapping accepted")
	}
}
