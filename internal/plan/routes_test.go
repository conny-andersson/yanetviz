package plan

import (
	"math"
	"testing"
)

func TestRouteValidation(t *testing.T) {
	tests := []struct {
		name     string
		sections []RouteSection
		valid    bool
	}{
		{"legacy", nil, true},
		{"intermediate portals", []RouteSection{
			{Floor: "basement", Exit: &RoutePoint{.3, .4}, Confirmed: true},
			{Floor: "entrance", Entry: &RoutePoint{.2, .7}, Exit: &RoutePoint{.8, .1}, Points: []RoutePoint{{.4, .5}}},
			{Floor: "attic", Entry: &RoutePoint{.9, .2}},
		}, true},
		{"unknown floor", []RouteSection{{Floor: "missing"}}, false},
		{"repeated floor", []RouteSection{{Floor: "basement"}, {Floor: "basement"}}, false},
		{"negative portal", []RouteSection{{Floor: "basement", Entry: &RoutePoint{-.1, .5}}}, false},
		{"outside bend", []RouteSection{{Floor: "basement", Points: []RoutePoint{{.4, 1.01}}}}, false},
		{"nonfinite bend", []RouteSection{{Floor: "basement", Points: []RoutePoint{{math.NaN(), .5}}}}, false},
		{"too many bends", []RouteSection{{Floor: "basement", Points: make([]RoutePoint, 501)}}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := Seed()
			p.Cables[0].Sections = test.sections
			if err := p.Validate(); (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}

func TestFloorLevelsAreOptionalAndIndependentOfRoutes(t *testing.T) {
	p := Seed()
	p.Floors[0].Level = rackUnits(-1)
	p.Floors[1].Level = rackUnits(0)
	p.Floors[2].Level = rackUnits(0) // Multiple areas can share a level.
	p.Cables[0].Sections = []RouteSection{{Floor: p.Floors[0].ID, Confirmed: true}, {Floor: p.Floors[2].ID}}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	p.Floors[0].Level = rackUnits(1001)
	if err := p.Validate(); err == nil {
		t.Fatal("accepted out-of-range floor level")
	}
}
