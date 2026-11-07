package plan

import (
	"crypto/rand"
	"fmt"
)

// NewID creates a random UUID for a persistent application entity.
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Seed includes confirmed equipment and draft backbone connections. No cabinet
// positions, AP floors, or unknown port assignments are inferred.
func Seed() Plan {
	p := Plan{Version: 1, Floors: []Floor{}, Cabinets: []Cabinet{}, Devices: []Device{}, Ports: []Port{}, Cables: []Cable{}, PassThroughs: []PassThrough{}}
	for _, f := range []struct{ ID, Name string }{{"basement", "Basement"}, {"entrance", "Entrance floor"}, {"attic", "Attic"}, {"garage", "Garage"}} {
		p.Floors = append(p.Floors, Floor{ID: f.ID, Name: f.Name, Background: "/floorplans/" + f.ID + ".svg"})
	}
	basement, attic := NewID(), NewID()
	p.Cabinets = []Cabinet{{ID: basement, Name: "Basement cabinet", Floor: "basement", X: .5, Y: .5}, {ID: attic, Name: "Attic cabinet", Floor: "attic", X: .5, Y: .5}}
	ids := map[string]string{}
	for _, v := range []struct{ Name, Role, Address, Cabinet string }{{"IV14-RT01", "router", "10.81.0.254", basement}, {"IV14-SW01", "switch", "10.81.0.250", basement}, {"IV14-SW02", "switch", "10.81.0.251", attic}, {"IV14-SW04", "switch", "10.81.0.253", attic}, {"IV14-AP01", "access_point", "10.81.0.241", ""}, {"IV14-AP03", "access_point", "10.81.0.243", ""}, {"IV14-AP04", "access_point", "10.81.0.244", ""}} {
		id := NewID()
		ids[v.Name] = id
		d := Device{ID: id, Name: v.Name, Role: v.Role, Address: v.Address, Cabinet: v.Cabinet, Mode: "fixed", X: .5, Y: .5}
		if v.Name == "IV14-SW04" {
			d.Model = "CSS610-8G-2S+ · SwOS"
		}
		p.Devices = append(p.Devices, d)
	}
	ports := map[string]string{}
	labels := []string{"IV14-SW02-Attic", "Port2", "Kontor", "Desktop", "LivingRoom", "Port6", "Port7", "Port8", "IV14-SW01-Core", "SFP2"}
	for i, label := range labels {
		name := fmt.Sprintf("Port%d", i+1)
		connector := "RJ45"
		if i >= 8 {
			name = fmt.Sprintf("SFP+%d", i-7)
			connector = "SFP+"
		}
		id := NewID()
		ports[name] = id
		p.Ports = append(p.Ports, Port{ID: id, Device: ids["IV14-SW04"], Name: name, Label: label, Connector: connector, PoEMode: "unknown", Notes: "Label documented from SwOS screenshot, 2026-11-07. Current link state unknown."})
	}
	for _, v := range []struct{ A, B, Medium, Port string }{{"IV14-RT01", "IV14-SW01", "fiber", ""}, {"IV14-SW01", "IV14-SW04", "fiber", ports["SFP+1"]}, {"IV14-SW04", "IV14-SW02", "copper", ports["Port1"]}} {
		a, b := Endpoint{Device: ids[v.A]}, Endpoint{Device: ids[v.B]}
		if v.A == "IV14-SW04" {
			a.Port = v.Port
		} else if v.B == "IV14-SW04" {
			b.Port = v.Port
		}
		p.Cables = append(p.Cables, Cable{ID: NewID(), A: a, B: b, Medium: v.Medium, Draft: true, Label: "Backbone", Notes: "Device-level connection confirmed. Exact ports and intermediate panel routing still need documentation."})
	}
	return p
}
