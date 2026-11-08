package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// Floor describes a named canvas and its background image.
type Floor struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Background string `json:"background"`
}

// Cabinet groups equipment on a known floor. Placed distinguishes a known floor
// from a documented position on that floor.
type Cabinet struct {
	ShortName string  `json:"shortName,omitempty"`
	HeightU   *int    `json:"heightU,omitempty"`
	Placed    bool    `json:"placed"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Floor     string  `json:"floor"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
}

// Device is a user-documented physical device, independent of live observations.
type Device struct {
	ShortName string `json:"shortName,omitempty"`
	// RackUnit is the lowest occupied unit, numbered from U1 at the bottom.
	// Nil values preserve unknown dimensions and unassigned mounting positions.
	RackUnit *int    `json:"rackUnit,omitempty"`
	HeightU  *int    `json:"heightU,omitempty"`
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	Model    string  `json:"model"`
	Address  string  `json:"address"`
	Notes    string  `json:"notes"`
	Mode     string  `json:"mode"`
	Floor    string  `json:"floor"`
	Cabinet  string  `json:"cabinet"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}

// Port identifies one physical connector or one side of a passive position.
// PoE fields describe configuration; they do not establish live power delivery.
type Port struct {
	LabelNumber *int   `json:"labelNumber,omitempty"`
	ID          string `json:"id"`
	Device      string `json:"device"`
	Name        string `json:"name"`
	Label       string `json:"label"`
	Connector   string `json:"connector"`
	Position    string `json:"position"`
	Side        string `json:"side"`
	PoEMode     string `json:"poeMode"`
	PoEStandard string `json:"poeStandard"`
	Voltage     string `json:"voltage"`
	Notes       string `json:"notes"`
}

// Endpoint retains a known device even when its precise port is unresolved.
type Endpoint struct {
	Device string `json:"device"`
	Port   string `json:"port"`
}

// Cable documents a single physical segment. Draft keeps incomplete routing explicit.
type Cable struct {
	ID     string   `json:"id"`
	A      Endpoint `json:"a"`
	B      Endpoint `json:"b"`
	Medium string   `json:"medium"`
	Label  string   `json:"label"`
	Length *float64 `json:"length"`
	Notes  string   `json:"notes"`
	Draft  bool     `json:"draft"`
}

// PassThrough joins front/rear terminals without consuming external cable capacity.
type PassThrough struct {
	ID string `json:"id"`
	A  string `json:"a"`
	B  string `json:"b"`
}

// Plan is the versioned documentation snapshot exchanged with the editor.
type Plan struct {
	Interfaces   []NetworkInterface `json:"interfaces,omitempty"`
	Version      int                `json:"version"`
	Revision     int64              `json:"revision"`
	Floors       []Floor            `json:"floors"`
	Cabinets     []Cabinet          `json:"cabinets"`
	Devices      []Device           `json:"devices"`
	Ports        []Port             `json:"ports"`
	Cables       []Cable            `json:"cables"`
	PassThroughs []PassThrough      `json:"passThroughs"`
}

// Validate enforces documentation invariants before import or transactional save.
func (p Plan) Validate() error {
	if p.Floors == nil || p.Cabinets == nil || p.Devices == nil || p.Ports == nil || p.Cables == nil || p.PassThroughs == nil {
		return errors.New("plan collections must be arrays")
	}
	if p.Version != 1 {
		return errors.New("unsupported plan version")
	}
	if len(p.Floors) == 0 || len(p.Floors) > 20 || len(p.Devices) > 2000 || len(p.Ports) > 20000 || len(p.Cables) > 20000 {
		return errors.New("plan size is invalid")
	}
	ids := map[string]bool{}
	id := func(s string) error {
		if strings.TrimSpace(s) == "" || len(s) > 100 || ids[s] {
			return fmt.Errorf("missing or duplicate ID: %s", s)
		}
		ids[s] = true
		return nil
	}
	name := func(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 200 }
	xy := func(x, y float64) bool {
		return !math.IsNaN(x) && !math.IsNaN(y) && x >= 0 && x <= 1 && y >= 0 && y <= 1
	}
	floors := map[string]bool{}
	cabinets := map[string]Cabinet{}
	devices := map[string]Device{}
	ports := map[string]Port{}
	for _, f := range p.Floors {
		if err := id(f.ID); err != nil {
			return err
		}
		if !name(f.Name) {
			return errors.New("floor name is required")
		}
		if err := validateBackground(f.Background); err != nil {
			return err
		}
		floors[f.ID] = true
	}
	for _, c := range p.Cabinets {
		if err := id(c.ID); err != nil {
			return err
		}
		if !name(c.Name) || !floors[c.Floor] || !xy(c.X, c.Y) {
			return errors.New("invalid cabinet placement")
		}
		if len(c.ShortName) > 100 {
			return fmt.Errorf("cabinet %q: short name must be at most 100 characters", c.Name)
		}
		cabinets[c.ID] = c
	}
	for _, d := range p.Devices {
		if err := id(d.ID); err != nil {
			return err
		}
		if !name(d.Name) || !oneOf(d.Mode, "fixed", "mobile") || !oneOf(d.Role, "router", "switch", "access_point", "client", "patch_panel", "wall_socket", "termination", "injector", "other") {
			return errors.New("invalid device name, role or placement mode")
		}
		if d.Floor != "" && !floors[d.Floor] {
			return errors.New("unknown floor")
		}
		if len(d.ShortName) > 100 {
			return fmt.Errorf("equipment %q: short name must be at most 100 characters", d.Name)
		}
		if d.Cabinet != "" {
			if _, ok := cabinets[d.Cabinet]; !ok {
				return errors.New("unknown cabinet")
			}
			if d.Floor != "" {
				return errors.New("cabinet members inherit placement")
			}
		}
		if d.Mode == "mobile" && (d.Floor != "" || d.Cabinet != "") {
			return errors.New("mobile device cannot have a physical placement")
		}
		if !xy(d.X, d.Y) {
			return errors.New("invalid device coordinates")
		}
		devices[d.ID] = d
	}
	if err := p.validateInterfaces(id, devices); err != nil {
		return err
	}
	if err := p.validateRackMounting(); err != nil {
		return err
	}
	portNames := map[string]bool{}
	for _, port := range p.Ports {
		if err := id(port.ID); err != nil {
			return err
		}
		if _, ok := devices[port.Device]; !ok {
			return errors.New("port has unknown device")
		}
		if port.LabelNumber != nil && (*port.LabelNumber < 1 || *port.LabelNumber > 9999) {
			return fmt.Errorf("port %q: cable label number must be between 1 and 9999", port.Name)
		}
		k := port.Device + "/" + port.Name
		if !name(port.Name) || portNames[k] {
			return errors.New("port names must be nonempty and unique per device")
		}
		portNames[k] = true
		if !oneOf(port.PoEMode, "unknown", "auto", "forced", "off", "unsupported") {
			return errors.New("invalid PoE mode")
		}
		if !oneOf(port.Side, "", "front", "rear") {
			return errors.New("invalid terminal side")
		}
		ports[port.ID] = port
	}
	used := map[string]bool{}
	links := map[string]bool{}
	for _, c := range p.Cables {
		if err := id(c.ID); err != nil {
			return err
		}
		if !oneOf(c.Medium, "copper", "fiber", "unknown") {
			return errors.New("invalid cable medium")
		}
		if c.Length != nil && (*c.Length < 0 || math.IsInf(*c.Length, 0) || math.IsNaN(*c.Length)) {
			return errors.New("invalid cable length")
		}
		if c.A.Device == c.B.Device {
			return errors.New("cable must join distinct devices")
		}
		if !c.Draft && (c.A.Port == "" || c.B.Port == "") {
			return errors.New("complete cables need both ports")
		}
		for _, e := range []Endpoint{c.A, c.B} {
			if _, ok := devices[e.Device]; !ok {
				return errors.New("cable has unknown device")
			}
			if e.Port != "" {
				port, ok := ports[e.Port]
				if !ok || port.Device != e.Device {
					return errors.New("cable port does not belong to endpoint device")
				}
				if used[e.Port] {
					return errors.New("a terminal can have only one external cable")
				}
				used[e.Port] = true
			}
		}
		a, b := c.A.Device+"/"+c.A.Port, c.B.Device+"/"+c.B.Port
		if a > b {
			a, b = b, a
		}
		k := a + "|" + b
		if links[k] {
			return errors.New("duplicate cable")
		}
		links[k] = true
	}
	paired := map[string]bool{}
	for _, v := range p.PassThroughs {
		if err := id(v.ID); err != nil {
			return err
		}
		a, aok := ports[v.A]
		b, bok := ports[v.B]
		d := devices[a.Device]
		samePosition := a.Position != "" && a.Position == b.Position
		oppositeSides := (a.Side == "front" && b.Side == "rear") || (a.Side == "rear" && b.Side == "front")
		passiveDevice := oneOf(d.Role, "patch_panel", "wall_socket", "termination")
		if !aok || !bok || a.ID == b.ID || a.Device != b.Device || !passiveDevice || !samePosition || !oppositeSides || paired[a.ID] || paired[b.ID] {
			return errors.New("pass-through must pair unused front/rear terminals of one passive position")
		}
		paired[a.ID] = true
		paired[b.ID] = true
	}
	// Reject oversized free text as well as malformed structure before committing.
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode plan: %w", err)
	}
	if len(raw) > 32<<20 {
		return errors.New("plan exceeds 32 MiB")
	}
	return nil
}
func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
