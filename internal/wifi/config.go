// Package wifi collects read-only RouterOS WiFi CAPsMAN evidence.
package wifi

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"yanetviz/internal/plan"
	"yanetviz/internal/routeros"
)

// Config keeps management endpoints and credential references outside the plan.
// This increment supports the modern /interface/wifi CAPsMAN tables.
type Config struct {
	PollSeconds int      `json:"pollSeconds"`
	Sources     []Source `json:"sources"`
}

// Source explicitly authorizes one endpoint, identity domain and radio mappings.
// Passwords are read from a separate file and never returned by the application.
type Source struct {
	ID           string            `json:"id"`
	Device       string            `json:"device"`
	Domain       string            `json:"domain"`
	Address      string            `json:"address"`
	Transport    string            `json:"transport"`
	Username     string            `json:"username"`
	PasswordFile string            `json:"passwordFile"`
	Radios       map[string]string `json:"radios"`
	password     string
	endpoint     string
}

// LoadConfig validates explicit sources against the documented devices. Relative
// password file paths resolve against the configuration directory. HTTPS is the
// default; HTTP requires an explicit per-source rest-http choice.
func LoadConfig(path string, document plan.Plan) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open Wi-Fi config: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("invalid Wi-Fi configuration JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return config, fmt.Errorf("expected one Wi-Fi configuration document")
	}
	if config.PollSeconds == 0 {
		config.PollSeconds = 60
	}
	if config.PollSeconds < 5 || config.PollSeconds > 60 || len(config.Sources) == 0 || len(config.Sources) > 8 {
		return config, fmt.Errorf("configure 1–8 Wi-Fi sources and a poll interval of 5–60 seconds")
	}
	devices := map[string]bool{}
	for _, device := range document.Devices {
		devices[device.ID] = true
	}
	ids := map[string]bool{}
	for i := range config.Sources {
		source := &config.Sources[i]
		source.Domain = strings.ToLower(strings.TrimSpace(source.Domain))
		if source.ID == "" || len(source.ID) > 100 || ids[source.ID] || !devices[source.Device] || source.Domain == "" || source.Username == "" {
			return config, fmt.Errorf("Wi-Fi source %d needs a unique ID, documented device, domain and username", i+1)
		}
		ids[source.ID] = true
		// Reuse plan identity validation rather than accepting incompatible domains.
		probe := document
		probe.Interfaces = []plan.NetworkInterface{{ID: "wifi-domain-probe", Device: source.Device, Name: "Wi-Fi", Domain: source.Domain, MACAddresses: []string{}}}
		if err := probe.Validate(); err != nil {
			return config, fmt.Errorf("Wi-Fi source %q has an invalid identity domain", source.ID)
		}
		if source.Transport == "" {
			source.Transport = "rest-https"
		}
		source.endpoint, err = routeros.Endpoint(source.Address, source.Transport)
		if err != nil {
			return config, fmt.Errorf("source %q: %w", source.ID, err)
		}
		source.password, err = routeros.Password(source.PasswordFile, path)
		if err != nil {
			return config, fmt.Errorf("source %q: %w", source.ID, err)
		}
		mappings := map[string]string{}
		for mac, device := range source.Radios {
			canonical, err := plan.NormalizeMAC(mac)
			if err != nil || !devices[device] || mappings[canonical] != "" {
				return config, fmt.Errorf("source %q: invalid or duplicate radio mapping", source.ID)
			}
			mappings[canonical] = device
		}
		source.Radios = mappings
	}
	return config, nil
}
