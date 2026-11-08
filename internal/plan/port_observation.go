package plan

import (
	"fmt"
	"math"
	"time"
)

// ObservedPoE keeps vendor configuration separate from operational measurements.
// Current is in milliamps; voltage and power use volts and watts. Nil is unknown.
type ObservedPoE struct {
	Mode             string   `json:"mode"`
	VendorMode       string   `json:"vendorMode,omitempty"`
	VoltageSelection string   `json:"voltageSelection,omitempty"`
	Status           string   `json:"status"`
	VoltageV         *float64 `json:"voltageV,omitempty"`
	CurrentMA        *float64 `json:"currentMA,omitempty"`
	PowerW           *float64 `json:"powerW,omitempty"`
}

// PortReading describes a source-native physical interface. Port is an explicit
// documentation binding, never a guessed cable destination or RouterOS row ID.
type PortReading struct {
	Port        string       `json:"port,omitempty"`
	Name        string       `json:"name"`
	DefaultName string       `json:"defaultName,omitempty"`
	MAC         string       `json:"mac,omitempty"`
	Disabled    *bool        `json:"disabled,omitempty"`
	Link        string       `json:"link,omitempty"`
	RateMbps    *float64     `json:"rateMbps,omitempty"`
	FullDuplex  *bool        `json:"fullDuplex,omitempty"`
	PoE         *ObservedPoE `json:"poe,omitempty"`
	Present     bool         `json:"present"`
	LastSeen    string       `json:"lastSeen"`
}

// DeviceInventory is observed hardware/software metadata, separate from the
// user's model/name fields. Failed refreshes retain the last successful values.
type DeviceInventory struct {
	Model        string `json:"model,omitempty"`
	BoardName    string `json:"boardName,omitempty"`
	OSVersion    string `json:"osVersion,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	LastAttempt  string `json:"lastAttempt"`
	LastSuccess  string `json:"lastSuccess,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Validate checks inventory timestamps and bounds device-supplied descriptive
// fields before persistence. Inventory never changes the manual device record.
func (inventory DeviceInventory) Validate() error {
	if _, err := time.Parse(time.RFC3339Nano, inventory.LastAttempt); err != nil {
		return fmt.Errorf("invalid inventory attempt time")
	}
	if inventory.LastSuccess != "" {
		if _, err := time.Parse(time.RFC3339Nano, inventory.LastSuccess); err != nil {
			return fmt.Errorf("invalid inventory success time")
		}
	}
	for _, value := range []string{inventory.Model, inventory.BoardName, inventory.OSVersion, inventory.Architecture} {
		if len(value) > 200 {
			return fmt.Errorf("device inventory field exceeds 200 bytes")
		}
	}
	if len(inventory.Error) > 500 {
		return fmt.Errorf("device inventory error exceeds 500 bytes")
	}
	return nil
}

// PortSnapshot stores complete link or PoE samples independently, so a PoE
// failure does not invalidate a successful link poll on the same device.
type PortSnapshot struct {
	Inventory   *DeviceInventory `json:"inventory,omitempty"`
	Source      string           `json:"source"`
	Device      string           `json:"device"`
	Channel     string           `json:"channel"`
	Capability  string           `json:"capability"`
	LastAttempt string           `json:"lastAttempt"`
	LastSuccess string           `json:"lastSuccess,omitempty"`
	Error       string           `json:"error,omitempty"`
	Ports       []PortReading    `json:"ports"`
}

// PortSourceView exposes collection health without endpoint credentials.
type PortSourceView struct {
	Inventory   *DeviceInventory `json:"inventory,omitempty"`
	Source      string           `json:"source"`
	Device      string           `json:"device"`
	Channel     string           `json:"channel"`
	Capability  string           `json:"capability"`
	State       string           `json:"state"`
	LastAttempt string           `json:"lastAttempt,omitempty"`
	LastSuccess string           `json:"lastSuccess,omitempty"`
	Error       string           `json:"error,omitempty"`
}

// PortObservation reconciles a reading against the current physical plan.
type PortObservation struct {
	PortReading
	Source  string `json:"source"`
	Device  string `json:"device"`
	Channel string `json:"channel"`
	State   string `json:"state"`
}

// PortView is separate from manual plan JSON and its revision.
type PortView struct {
	Sources  []PortSourceView  `json:"sources"`
	Readings []PortObservation `json:"readings"`
}

// ReconcilePorts retains historical evidence, marks restart/expired data stale,
// and drops bindings to removed or reassigned ports without changing the plan.
func ReconcilePorts(p Plan, snapshots []PortSnapshot, now, started time.Time, enabled map[string]bool) PortView {
	view := PortView{Sources: []PortSourceView{}, Readings: []PortObservation{}}
	devices := map[string]bool{}
	ports := map[string]Port{}
	for _, device := range p.Devices {
		devices[device.ID] = true
	}
	for _, port := range p.Ports {
		ports[port.ID] = port
	}
	for _, snapshot := range snapshots {
		success, err := time.Parse(time.RFC3339Nano, snapshot.LastSuccess)
		fresh := err == nil && success.After(started) && !success.After(now) && now.Sub(success) < ObservationMaxAge && enabled[snapshot.Source] && devices[snapshot.Device] && snapshot.Capability == "supported"
		state := "stale"
		if fresh {
			state = "fresh"
		}
		if oneOf(snapshot.Capability, "unavailable", "unsupported", "disabled") {
			state = snapshot.Capability
		}
		if snapshot.Error != "" {
			state = "error"
		}
		if !enabled[snapshot.Source] {
			state = "disabled"
		}
		view.Sources = append(view.Sources, PortSourceView{Inventory: snapshot.Inventory, Source: snapshot.Source, Device: snapshot.Device, Channel: snapshot.Channel, Capability: snapshot.Capability, State: state, LastAttempt: snapshot.LastAttempt, LastSuccess: snapshot.LastSuccess, Error: snapshot.Error})
		for _, reading := range snapshot.Ports {
			state := "stale"
			if fresh {
				state = "fresh"
				if !reading.Present {
					state = "missing"
				}
			}
			port, known := ports[reading.Port]
			if !known || port.Device != snapshot.Device {
				reading.Port = ""
			}
			view.Readings = append(view.Readings, PortObservation{PortReading: reading, Source: snapshot.Source, Device: snapshot.Device, Channel: snapshot.Channel, State: state})
		}
	}
	return view
}

func (s PortSnapshot) validate() error {
	if s.Source == "" || len(s.Source) > 100 || s.Device == "" || !oneOf(s.Channel, "link", "poe") || !oneOf(s.Capability, "", "supported", "unavailable", "disabled", "unsupported") || len(s.Ports) > 2000 {
		return fmt.Errorf("invalid port snapshot identity or size")
	}
	if _, err := time.Parse(time.RFC3339Nano, s.LastAttempt); err != nil {
		return fmt.Errorf("invalid port snapshot time")
	}
	if s.Inventory != nil {
		if err := s.Inventory.Validate(); err != nil {
			return err
		}
	}
	if s.Error != "" || oneOf(s.Capability, "unavailable", "disabled", "unsupported") {
		return nil
	}
	keys := map[string]bool{}
	for _, port := range s.Ports {
		key := port.nativeKey()
		if key == "" || keys[key] || port.Name == "" {
			return fmt.Errorf("port readings require distinct native interfaces")
		}
		if !oneOf(port.Link, "", "up", "down", "unknown") {
			return fmt.Errorf("invalid observed link state")
		}
		metrics := []*float64{port.RateMbps}
		if port.PoE != nil {
			if !oneOf(port.PoE.Mode, "auto", "forced", "off", "unknown") {
				return fmt.Errorf("invalid observed PoE mode")
			}
			metrics = append(metrics, port.PoE.VoltageV, port.PoE.CurrentMA, port.PoE.PowerW)
		}
		for _, value := range metrics {
			if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0) {
				return fmt.Errorf("invalid observed port measurement")
			}
		}
		keys[key] = true
	}
	return nil
}

func (p PortReading) nativeKey() string {
	if p.DefaultName != "" {
		return p.DefaultName
	}
	return p.Name
}
