package plan

import (
	"errors"
	"math"
)

// RoutePoint is normalized against one floor's original background dimensions.
type RoutePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// RouteSection draws part of one cable, ordered from endpoint A to B. Entry and
// exit are drawing portals, never physical terminals. Nil portals use automatic
// positions. The first and last sections anchor to equipment when on its floor.
// Confirmed describes manual route knowledge independently of cable draft state.
type RouteSection struct {
	Floor     string       `json:"floor"`
	Entry     *RoutePoint  `json:"entry,omitempty"`
	Exit      *RoutePoint  `json:"exit,omitempty"`
	Points    []RoutePoint `json:"points,omitempty"`
	Confirmed bool         `json:"confirmed"`
}

func validateSections(sections []RouteSection, floors map[string]bool) error {
	if len(sections) > len(floors) {
		return errors.New("a route can visit each floor plan once")
	}
	seen := map[string]bool{}
	valid := func(p RoutePoint) bool {
		return !math.IsNaN(p.X) && !math.IsNaN(p.Y) && p.X >= 0 && p.X <= 1 && p.Y >= 0 && p.Y <= 1
	}
	for _, section := range sections {
		if !floors[section.Floor] || seen[section.Floor] {
			return errors.New("route sections require distinct, known floor plans")
		}
		seen[section.Floor] = true
		if len(section.Points) > 500 {
			return errors.New("route section exceeds 500 bend points")
		}
		for _, portal := range []*RoutePoint{section.Entry, section.Exit} {
			if portal != nil && !valid(*portal) {
				return errors.New("route portal coordinates must be between 0 and 1")
			}
		}
		for _, point := range section.Points {
			if !valid(point) {
				return errors.New("route bend coordinates must be between 0 and 1")
			}
		}
	}
	return nil
}
