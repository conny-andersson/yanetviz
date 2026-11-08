package plan

import (
	"errors"
	"fmt"
	"net/netip"
	"time"
)

// ErrInvalidDHCPObservation distinguishes rejected input from persistence failures.
var ErrInvalidDHCPObservation = errors.New("invalid DHCP observation")

// DHCPObservation retains the source's client ID verbatim. It is evidence for an
// interface, not an application identifier, a serving AP, or live presence.
// FirstSeen and LastSeen describe collection times, not lease start/expiry times.
type DHCPObservation struct {
	Domain        string `json:"domain"`
	Interface     string `json:"interface"`
	Source        string `json:"source"`
	Server        string `json:"server"`
	MACAddress    string `json:"macAddress"`
	ClientID      string `json:"clientId"`
	ClientIDField string `json:"clientIdField,omitempty"`
	Address       string `json:"address"`
	Hostname      string `json:"hostname"`
	Status        string `json:"status"`
	FirstSeen     string `json:"firstSeen"`
	LastSeen      string `json:"lastSeen"`
}

func (o DHCPObservation) validate(p Plan) error {
	var item *NetworkInterface
	for _, candidate := range p.Interfaces {
		if candidate.ID == o.Interface {
			copy := candidate
			item = &copy
			break
		}
	}
	if item == nil {
		return errors.New("DHCP observation references an unknown network interface")
	}
	if o.Domain != item.Domain {
		return errors.New("DHCP observation domain does not match its interface")
	}
	matched, err := p.MatchInterface(o.Domain, o.MACAddress)
	if err != nil || matched == nil || matched.ID != item.ID {
		return errors.New("DHCP observation MAC is not an alias of its interface")
	}
	sourceKnown := false
	for _, device := range p.Devices {
		if device.ID == o.Source {
			sourceKnown = true
			break
		}
	}
	if !sourceKnown {
		return errors.New("DHCP observation source is not a documented device")
	}
	if len(o.Server) > 200 || len(o.ClientID) > 4096 || len(o.Hostname) > 1024 {
		return errors.New("DHCP observation field is too long")
	}
	if !oneOf(o.ClientIDField, "", "client-id", "active-client-id") {
		return errors.New("unknown DHCP client ID source field")
	}
	if o.Address != "" {
		if address, err := netip.ParseAddr(o.Address); err != nil || !address.Is4() {
			return errors.New("invalid observed DHCP IPv4 address")
		}
	}
	if !oneOf(o.Status, "bound", "waiting", "testing", "declined", "offered", "authorizing", "conflict", "unknown") {
		return errors.New("invalid DHCP lease status")
	}
	first, err := time.Parse(time.RFC3339Nano, o.FirstSeen)
	if err != nil {
		return fmt.Errorf("invalid first-seen timestamp: %w", err)
	}
	last, err := time.Parse(time.RFC3339Nano, o.LastSeen)
	if err != nil || first.After(last) {
		return errors.New("invalid last-seen timestamp or observation interval")
	}
	return nil
}
