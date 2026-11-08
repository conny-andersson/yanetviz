package wifi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"yanetviz/internal/plan"
)

const radioMenu = "interface/wifi/radio"
const interfaceMenu = "interface/wifi"
const registrationMenu = "interface/wifi/registration-table"

type row map[string]string

// Collector joins non-secret radio inventory and serializes requests through
// its polling worker. Unknown mappings remain explicit rather than guessed.
type Collector struct {
	source Source
	client *http.Client

	radios     []plan.WiFiRadio
	interfaces map[string]string
}

// NewCollector uses verified TLS and refuses redirects, including HTTPS to HTTP.
func NewCollector(source Source) *Collector {
	return &Collector{source: source, client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("redirects are not allowed") },
	}}
}

func (c *Collector) read(ctx context.Context, menu, properties string) ([]row, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.source.endpoint+menu+"?.proplist="+url.QueryEscape(properties), nil)
	if err != nil {
		return nil, fmt.Errorf("invalid management endpoint")
	}
	request.SetBasicAuth(c.source.Username, c.source.password)
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connection failed or timed out; check management access and TLS")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication failed; check the source credentials")
	case http.StatusForbidden:
		return nil, fmt.Errorf("permission denied; allow read access to WiFi CAPsMAN")
	case http.StatusBadRequest, http.StatusNotFound:
		return nil, fmt.Errorf("WiFi CAPsMAN menu unsupported; this collector requires /interface/wifi")
	default:
		return nil, fmt.Errorf("management request failed (HTTP %d)", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		return nil, fmt.Errorf("incomplete or oversized management response")
	}
	var rows []row
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil || len(rows) > 10000 {
		return nil, fmt.Errorf("invalid or incomplete management response")
	}
	return rows, nil
}

func (c *Collector) inventory(ctx context.Context) error {
	radios, err := c.read(ctx, radioMenu, "radio-mac,interface,local,cap")
	if err != nil {
		return err
	}
	interfaces, err := c.read(ctx, interfaceMenu, "name,radio-mac,master-interface")
	if err != nil {
		return err
	}
	observed := []plan.WiFiRadio{}
	names := map[string]string{}
	for _, radio := range radios {
		mac, err := plan.NormalizeMAC(radio["radio-mac"])
		if err != nil || radio["interface"] == "" {
			return fmt.Errorf("incomplete radio inventory")
		}
		ap := c.source.Radios[mac]
		if radio["local"] == "true" && ap == "" {
			ap = c.source.Device
		}
		observed = append(observed, plan.WiFiRadio{MAC: mac, InterfaceName: radio["interface"], CAP: radio["cap"], AP: ap})
		if previous := names[radio["interface"]]; previous != "" && previous != mac {
			return fmt.Errorf("ambiguous radio interface mapping")
		}
		names[radio["interface"]] = mac
	}
	// Virtual SSIDs inherit their documented master radio, never a name prefix.
	for range len(interfaces) + 1 {
		changed := false
		for _, item := range interfaces {
			name := item["name"]
			if name == "" {
				return fmt.Errorf("incomplete Wi-Fi interface inventory")
			}
			mac := item["radio-mac"]
			if mac != "" && mac != "00:00:00:00:00:00" {
				normalized, err := plan.NormalizeMAC(mac)
				if err != nil {
					return fmt.Errorf("invalid interface radio MAC")
				}
				mac = normalized
			} else {
				mac = names[item["master-interface"]]
			}
			if mac != "" && names[name] != mac {
				names[name] = mac
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	c.radios = observed
	c.interfaces = names
	return nil
}

// Poll returns a complete snapshot only after all registration rows decode. Any
// transport, permission or parse failure must be recorded as health, not absence.
func (c *Collector) Poll(ctx context.Context, now time.Time) (plan.WiFiSnapshot, error) {
	snapshot := plan.WiFiSnapshot{Source: c.source.ID, Device: c.source.Device, Domain: c.source.Domain, Menu: registrationMenu, LastAttempt: now.UTC().Format(time.RFC3339Nano), Radios: []plan.WiFiRadio{}, Registrations: []plan.WiFiRegistration{}}
	rows, err := c.read(ctx, registrationMenu, "mac-address,interface,authorized,ssid,band,signal,rx-rate,tx-rate")
	if err != nil {
		return snapshot, err
	}
	// Refresh the joins with every snapshot: a controller may reuse an interface
	// name after provisioning. Cached names must never select a different AP.
	if err := c.inventory(ctx); err != nil {
		return snapshot, err
	}

	snapshot.Radios = append(snapshot.Radios, c.radios...)
	owners := map[string]string{}
	for _, radio := range c.radios {
		owners[radio.MAC] = radio.AP
	}
	seen := map[string]bool{}
	for _, item := range rows {
		mac, err := plan.NormalizeMAC(item["mac-address"])
		if err != nil || item["interface"] == "" || (item["authorized"] != "true" && item["authorized"] != "false") {
			return snapshot, fmt.Errorf("incomplete or invalid Wi-Fi registration")
		}
		key := mac + "/" + item["interface"]
		if seen[key] {
			return snapshot, fmt.Errorf("duplicate Wi-Fi registration; snapshot is incomplete")
		}
		seen[key] = true
		radio := c.interfaces[item["interface"]]
		record := plan.WiFiRegistration{Authorized: item["authorized"] == "true", MAC: mac, InterfaceName: item["interface"], RadioMAC: radio, AP: owners[radio], SSID: item["ssid"], Band: item["band"]}
		if item["signal"] != "" {
			signal, err := strconv.Atoi(item["signal"])
			if err != nil || signal < -200 || signal > 100 {
				return snapshot, fmt.Errorf("invalid Wi-Fi signal measurement")
			}
			record.Signal = &signal
		}
		record.RxRate, err = parseRate(item["rx-rate"])
		if err != nil {
			return snapshot, err
		}
		record.TxRate, err = parseRate(item["tx-rate"])
		if err != nil {
			return snapshot, err
		}
		snapshot.Registrations = append(snapshot.Registrations, record)
	}
	return snapshot, nil
}

func parseRate(raw string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	rate, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || rate < 0 {
		return nil, fmt.Errorf("invalid Wi-Fi link rate")
	}
	return &rate, nil
}
