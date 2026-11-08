package plan

import (
	"fmt"
	"time"
)

// WiFiRegistration is radio evidence, not a physical connection or placement.
// Rates are bits per second; nil metrics explicitly mean unavailable.
type WiFiRegistration struct {
	Authorized    bool   `json:"authorized"`
	MAC           string `json:"mac"`
	InterfaceName string `json:"interfaceName"`
	RadioMAC      string `json:"radioMac,omitempty"`
	AP            string `json:"ap,omitempty"`
	SSID          string `json:"ssid,omitempty"`
	Band          string `json:"band,omitempty"`
	Signal        *int   `json:"signal,omitempty"`
	RxRate        *int64 `json:"rxRate,omitempty"`
	TxRate        *int64 `json:"txRate,omitempty"`
	LastSeen      string `json:"lastSeen"`
	Present       bool   `json:"present"`
}

// WiFiRadio describes the controller inventory and an explicit physical AP mapping.
type WiFiRadio struct {
	MAC           string `json:"mac"`
	InterfaceName string `json:"interfaceName"`
	CAP           string `json:"cap,omitempty"`
	AP            string `json:"ap,omitempty"`
}

// WiFiSnapshot retains the last complete source snapshot and collection health.
// Credentials and router configuration must never enter this structure.
type WiFiSnapshot struct {
	Source        string             `json:"source"`
	Device        string             `json:"device"`
	Domain        string             `json:"domain"`
	Menu          string             `json:"menu,omitempty"`
	LastAttempt   string             `json:"lastAttempt"`
	LastSuccess   string             `json:"lastSuccess,omitempty"`
	Error         string             `json:"error,omitempty"`
	Radios        []WiFiRadio        `json:"radios"`
	Registrations []WiFiRegistration `json:"registrations"`
}

// WiFiClient reconciles a registration with current manual interface identities.
// Unknown identities and APs remain unresolved, including after plan edits.
type WiFiClient struct {
	WiFiRegistration
	Source    string `json:"source"`
	Domain    string `json:"domain"`
	Device    string `json:"device,omitempty"`
	Interface string `json:"interface,omitempty"`
	State     string `json:"state"`
}

// WiFiSourceView exposes health without endpoint credentials or configuration.
type WiFiSourceView struct {
	Source      string      `json:"source"`
	Device      string      `json:"device"`
	Menu        string      `json:"menu,omitempty"`
	LastAttempt string      `json:"lastAttempt,omitempty"`
	LastSuccess string      `json:"lastSuccess,omitempty"`
	Error       string      `json:"error,omitempty"`
	State       string      `json:"state"`
	Radios      []WiFiRadio `json:"radios"`
}

// WiFiView is a derived observation view, excluded from portable plan exports.
type WiFiView struct {
	Sources []WiFiSourceView `json:"sources"`
	Clients []WiFiClient     `json:"clients"`
}

// ReconcileWiFi matches exact domain/MAC aliases and evaluates freshness at read
// time. Restored snapshots remain stale until this process completes a poll.
func ReconcileWiFi(p Plan, snapshots []WiFiSnapshot, now, started time.Time, enabled map[string]bool) WiFiView {
	view := WiFiView{Sources: []WiFiSourceView{}, Clients: []WiFiClient{}}
	devices := map[string]bool{}
	for _, device := range p.Devices {
		devices[device.ID] = true
	}
	for _, snapshot := range snapshots {
		success, err := time.Parse(time.RFC3339Nano, snapshot.LastSuccess)
		fresh := err == nil && success.After(started) && !success.After(now) && now.Sub(success) < ObservationMaxAge && enabled[snapshot.Source] && devices[snapshot.Device]
		sourceState := "stale"
		if fresh {
			sourceState = "fresh"
		}
		if snapshot.Error != "" {
			sourceState = "error"
		}
		if !enabled[snapshot.Source] {
			sourceState = "disabled"
		}
		radios := append([]WiFiRadio{}, snapshot.Radios...)
		for i := range radios {
			if !devices[radios[i].AP] {
				radios[i].AP = ""
			}
		}
		view.Sources = append(view.Sources, WiFiSourceView{
			Source:      snapshot.Source,
			Device:      snapshot.Device,
			Menu:        snapshot.Menu,
			LastAttempt: snapshot.LastAttempt,
			LastSuccess: snapshot.LastSuccess,
			Error:       snapshot.Error,
			State:       sourceState,
			Radios:      radios,
		})
		// Keep all simultaneous registrations to expose conflicts. For absent clients,
		// present only the most recent evidence, rather than every historical radio.
		present := map[string]bool{}
		latest := map[string]time.Time{}
		for _, record := range snapshot.Registrations {
			if record.Present {
				present[record.MAC] = true
			}
			seen, _ := time.Parse(time.RFC3339Nano, record.LastSeen)
			if seen.After(latest[record.MAC]) {
				latest[record.MAC] = seen
			}
		}
		absentShown := map[string]bool{}
		for _, record := range snapshot.Registrations {
			seen, _ := time.Parse(time.RFC3339Nano, record.LastSeen)
			if !record.Present && (present[record.MAC] || !seen.Equal(latest[record.MAC]) || absentShown[record.MAC]) {
				continue
			}
			if !record.Present {
				absentShown[record.MAC] = true
			}
			client := WiFiClient{WiFiRegistration: record, Source: snapshot.Source, Domain: snapshot.Domain, State: "stale"}
			if !devices[client.AP] {
				client.AP = ""
			}
			if fresh {
				client.State = "associated"
				if !record.Present {
					client.State = "not-associated"
				}
			}
			identity, matchErr := p.MatchInterface(snapshot.Domain, record.MAC)
			if matchErr == nil && identity != nil {
				client.Device = identity.Device
				client.Interface = identity.ID
			}
			view.Clients = append(view.Clients, client)
		}
	}
	groups := map[string][]int{}
	for i, client := range view.Clients {
		if client.State == "associated" {
			key := client.Domain + "/" + client.MAC
			groups[key] = append(groups[key], i)
		}
	}
	for _, indices := range groups {
		sources := map[string]int{}
		aps := map[string]bool{}
		conflict := false
		for _, i := range indices {
			client := view.Clients[i]
			sources[client.Source]++
			if sources[client.Source] > 1 {
				conflict = true
			}
			if client.AP != "" {
				aps[client.AP] = true
			}
		}
		if conflict || len(aps) > 1 {
			for _, i := range indices {
				view.Clients[i].State = "conflict"
			}
		}
	}
	return view
}

func (snapshot WiFiSnapshot) validate() error {
	if snapshot.Source == "" || len(snapshot.Source) > 100 || snapshot.Device == "" || !identityDomain.MatchString(snapshot.Domain) {
		return fmt.Errorf("invalid Wi-Fi source identity")
	}
	if _, err := time.Parse(time.RFC3339Nano, snapshot.LastAttempt); err != nil {
		return fmt.Errorf("invalid Wi-Fi attempt time")
	}
	if len(snapshot.Registrations) > 10000 || len(snapshot.Radios) > 2000 {
		return fmt.Errorf("Wi-Fi snapshot exceeds size limit")
	}
	if snapshot.Error != "" {
		return nil
	}
	seen := map[string]bool{}
	for _, record := range snapshot.Registrations {
		mac, err := NormalizeMAC(record.MAC)
		key := record.MAC + "/" + record.InterfaceName
		if err != nil || mac != record.MAC || record.InterfaceName == "" || seen[key] {
			return fmt.Errorf("invalid or duplicate Wi-Fi registration")
		}
		seen[key] = true
	}
	return nil
}
