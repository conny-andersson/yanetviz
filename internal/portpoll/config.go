// Package portpoll collects read-only physical Ethernet and PoE observations.
package portpoll

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"yanetviz/internal/plan"
	"yanetviz/internal/routeros"
)

// Config explicitly enables physical-port monitoring for known devices.
type Config struct {
	PollSeconds int      `json:"pollSeconds"`
	Sources     []Source `json:"sources"`
}

// Source binds RouterOS default interface names to documented port UUIDs. These
// mappings survive friendly-name changes without relying on mutable row IDs.
type Source struct {
	PoE          string            `json:"poe,omitempty"`
	ID           string            `json:"id"`
	Device       string            `json:"device"`
	Address      string            `json:"address"`
	Transport    string            `json:"transport"`
	Username     string            `json:"username"`
	PasswordFile string            `json:"passwordFile"`
	Ports        map[string]string `json:"ports"`
	endpoint     string
	password     string
}

// LoadConfig validates ownership and credentials without making network requests.
func LoadConfig(path string, p plan.Plan) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open port monitoring config: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("invalid port monitoring configuration JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return config, fmt.Errorf("expected one port monitoring configuration document")
	}
	if config.PollSeconds == 0 {
		config.PollSeconds = 60
	}
	if config.PollSeconds < 5 || config.PollSeconds > 60 || len(config.Sources) == 0 || len(config.Sources) > 8 {
		return config, fmt.Errorf("configure 1–8 port sources and a poll interval of 5–60 seconds")
	}
	devices := map[string]bool{}
	ports := map[string]plan.Port{}
	for _, device := range p.Devices {
		devices[device.ID] = true
	}
	for _, port := range p.Ports {
		ports[port.ID] = port
	}
	ids := map[string]bool{}
	owners := map[string]bool{}
	for i := range config.Sources {
		source := &config.Sources[i]
		if source.ID == "" || len(source.ID) > 100 || ids[source.ID] || !devices[source.Device] || owners[source.Device] || source.Username == "" {
			return config, fmt.Errorf("port source %d needs a unique ID, documented device and username; use one source per device", i+1)
		}
		if source.PoE == "" {
			source.PoE = "auto"
		}
		if source.PoE != "auto" && source.PoE != "disabled" && source.PoE != "unsupported" {
			return config, fmt.Errorf("source %q: poe must be auto, disabled or unsupported", source.ID)
		}
		ids[source.ID] = true
		owners[source.Device] = true
		source.endpoint, err = routeros.Endpoint(source.Address, source.Transport)
		if err != nil {
			return config, fmt.Errorf("source %q: %w", source.ID, err)
		}
		source.password, err = routeros.Password(source.PasswordFile, path)
		if err != nil {
			return config, fmt.Errorf("source %q: %w", source.ID, err)
		}
		used := map[string]bool{}
		for name, id := range source.Ports {
			port, known := ports[id]
			if name == "" || len(name) > 200 || !known || port.Device != source.Device || used[id] {
				return config, fmt.Errorf("source %q: port bindings must uniquely reference that device's documented ports", source.ID)
			}
			used[id] = true
		}
	}
	return config, nil
}
