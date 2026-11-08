package plan

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// NetworkInterface links one or more explicitly documented MAC aliases to a
// physical device. DHCP IDs, IPs and hostnames are separate source observations.
type NetworkInterface struct {
	ID           string   `json:"id"`
	Device       string   `json:"device"`
	Name         string   `json:"name"`
	Domain       string   `json:"domain"`
	MACAddresses []string `json:"macAddresses"`
	// Port optionally identifies a physical connector on the owning device.
	// Virtual interfaces may share a connector; wireless interfaces may omit it.
	Port string `json:"port,omitempty"`
}

// ErrAmbiguousIdentity prevents callers from choosing an arbitrary owner when
// multiple interfaces claim the same domain and MAC in an unvalidated plan.
var ErrAmbiguousIdentity = errors.New("multiple interfaces match this network domain and MAC")

var identityDomain = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,99}$`)

// NormalizeMAC accepts common 48-bit Ethernet notation and produces an uppercase,
// colon-separated unicast address suitable for matching Ethernet and Wi-Fi peers.
func NormalizeMAC(value string) (string, error) {
	mac, err := net.ParseMAC(strings.TrimSpace(value))
	if err != nil || len(mac) != 6 {
		return "", fmt.Errorf("invalid 48-bit MAC address %q", value)
	}
	zero := true
	for _, part := range mac {
		if part != 0 {
			zero = false
		}
	}
	if zero || mac[0]&1 != 0 {
		return "", fmt.Errorf("MAC %q must be a nonzero unicast address", value)
	}
	return strings.ToUpper(mac.String()), nil
}

// NormalizeIdentities returns an independent copy of the identity records, so
// canonicalization never mutates the caller's working copy or undo history.
func (p Plan) NormalizeIdentities() Plan {
	if len(p.Interfaces) == 0 {
		return p
	}
	p.Interfaces = append([]NetworkInterface(nil), p.Interfaces...)
	for index := range p.Interfaces {
		item := &p.Interfaces[index]
		item.Name = strings.TrimSpace(item.Name)
		item.Domain = strings.ToLower(strings.TrimSpace(item.Domain))
		addresses := []string{}
		for _, raw := range item.MACAddresses {
			if strings.TrimSpace(raw) == "" {
				continue
			}
			normalized, err := NormalizeMAC(raw)
			if err != nil {
				normalized = strings.TrimSpace(raw)
			}
			addresses = append(addresses, normalized)
		}
		item.MACAddresses = addresses
	}
	return p
}

func (p Plan) validateInterfaces(registerID func(string) error, devices map[string]Device) error {
	if len(p.Interfaces) > 20000 {
		return errors.New("too many network interfaces")
	}
	owners := map[string]string{}
	ports := make(map[string]Port, len(p.Ports))
	for _, port := range p.Ports {
		ports[port.ID] = port
	}
	for _, item := range p.Interfaces {
		if err := registerID(item.ID); err != nil {
			return err
		}
		if _, ok := devices[item.Device]; !ok {
			return errors.New("network interface has unknown device")
		}
		if item.Port != "" {
			port, known := ports[item.Port]
			if !known {
				return fmt.Errorf("interface %q: referenced port does not exist", item.Name)
			}
			if port.Device != item.Device {
				return fmt.Errorf("interface %q: referenced port must belong to the same device", item.Name)
			}
		}
		if strings.TrimSpace(item.Name) == "" || len(item.Name) > 200 {
			return errors.New("network interface name is required (maximum 200 bytes)")
		}
		domain := strings.ToLower(strings.TrimSpace(item.Domain))
		if !identityDomain.MatchString(domain) {
			return fmt.Errorf("interface %q: use a network domain such as home or guest", item.Name)
		}
		if len(item.MACAddresses) > 64 {
			return fmt.Errorf("interface %q: too many MAC aliases", item.Name)
		}
		for _, raw := range item.MACAddresses {
			mac, err := NormalizeMAC(raw)
			if err != nil {
				return fmt.Errorf("interface %q: %w", item.Name, err)
			}
			key := domain + "/" + mac
			if previous, ok := owners[key]; ok {
				return fmt.Errorf("MAC %s in domain %q is claimed more than once (%s and %s)", mac, domain, previous, item.Name)
			}
			owners[key] = item.Name
		}
	}
	return nil
}

// MatchInterface matches only the normalized domain/MAC key. IPs, hostnames and
// DHCP client IDs deliberately cannot select or merge a physical device.
func (p Plan) MatchInterface(domain, address string) (*NetworkInterface, error) {
	mac, err := NormalizeMAC(address)
	if err != nil {
		return nil, err
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	var found *NetworkInterface
	for _, item := range p.Interfaces {
		if strings.ToLower(strings.TrimSpace(item.Domain)) != domain {
			continue
		}
		for _, alias := range item.MACAddresses {
			canonical, err := NormalizeMAC(alias)
			if err != nil {
				continue
			}
			if canonical != mac {
				continue
			}
			if found != nil {
				return nil, ErrAmbiguousIdentity
			}
			copy := item
			found = &copy
		}
	}
	return found, nil
}
