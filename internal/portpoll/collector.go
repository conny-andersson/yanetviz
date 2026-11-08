package portpoll

import (
	"context"
	"errors"
	"fmt"
	"time"
	"yanetviz/internal/plan"
	"yanetviz/internal/routeros"
)

const ethernetMenu = "interface/ethernet"
const poeMenu = "interface/ethernet/poe"

// Collector reads separate link/PoE channels using one fresh native-port inventory.
type Collector struct {
	metadata    *plan.DeviceInventory
	source      Source
	client      *routeros.Client
	inventory   []routeros.Row
	inventoryAt time.Time
}

// NewCollector creates a read-only collector; requests share their endpoint gate
// with Wi-Fi collection so the same device never receives overlapping requests.
func NewCollector(source Source) *Collector {
	return &Collector{source: source, client: routeros.NewClient(source.endpoint, source.Username, source.password)}
}

func (c *Collector) base(channel string, now time.Time) plan.PortSnapshot {
	return plan.PortSnapshot{Inventory: c.metadata, Source: c.source.ID, Device: c.source.Device, Channel: channel, LastAttempt: now.UTC().Format(time.RFC3339Nano), Ports: []plan.PortReading{}}
}

func (c *Collector) readInventory(ctx context.Context, now time.Time) error {
	if c.inventoryAt.Equal(now) {
		return nil
	}
	rows, err := c.client.Read(ctx, ethernetMenu, ".id,name,default-name,mac-address,disabled")
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	names := map[string]bool{}
	defaults := map[string]bool{}
	for _, row := range rows {
		if row[".id"] == "" || row["name"] == "" || ids[row[".id"]] || names[row["name"]] {
			return fmt.Errorf("incomplete or ambiguous Ethernet inventory")
		}
		ids[row[".id"]] = true
		names[row["name"]] = true
		key := nativeKey(row)
		if defaults[key] {
			return fmt.Errorf("ambiguous default interface names")
		}
		defaults[key] = true
	}
	c.inventory = rows
	c.inventoryAt = now
	return nil
}

func nativeKey(row routeros.Row) string {
	if row["default-name"] != "" {
		return row["default-name"]
	}
	return row["name"]
}

func (c *Collector) reading(row routeros.Row) (plan.PortReading, error) {
	reading := plan.PortReading{Port: c.source.Ports[nativeKey(row)], Name: row["name"], DefaultName: row["default-name"]}
	if raw := row["mac-address"]; raw != "" {
		mac, err := plan.NormalizeMAC(raw)
		if err != nil {
			return reading, fmt.Errorf("invalid Ethernet MAC address")
		}
		reading.MAC = mac
	}
	disabled, err := parseBool(row["disabled"])
	reading.Disabled = disabled
	return reading, err
}

func inventoryIDs(rows []routeros.Row) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row[".id"])
	}
	return ids
}

func monitorByName(rows []routeros.Row) (map[string]routeros.Row, error) {
	indexed := map[string]routeros.Row{}
	for _, row := range rows {
		name := row["name"]
		if name == "" || indexed[name] != nil {
			return nil, fmt.Errorf("incomplete or ambiguous monitor response")
		}
		indexed[name] = row
	}
	return indexed, nil
}

func unavailable(err error) bool {
	var request *routeros.RequestError
	return errors.As(err, &request) && (request.Status == 400 || request.Status == 404)
}

// PollLink accepts a sample only when every inventoried interface is represented.
// Configured speed is never substituted for a missing negotiated link rate.
func (c *Collector) PollLink(ctx context.Context, now time.Time) (plan.PortSnapshot, error) {
	snapshot := c.base("link", now)
	if err := c.readInventory(ctx, now); err != nil {
		return snapshot, err
	}
	rows, err := c.client.Monitor(ctx, ethernetMenu, inventoryIDs(c.inventory))
	if unavailable(err) {
		snapshot.Capability = "unavailable"
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	indexed, err := monitorByName(rows)
	if err != nil || len(indexed) != len(c.inventory) {
		return snapshot, fmt.Errorf("incomplete Ethernet monitor snapshot")
	}
	for _, item := range c.inventory {
		row := indexed[item["name"]]
		if row == nil || row["status"] == "" {
			return snapshot, fmt.Errorf("missing Ethernet link status")
		}
		reading, err := c.reading(item)
		if err != nil {
			return snapshot, err
		}
		reading.Link = "unknown"
		switch row["status"] {
		case "link-ok":
			reading.Link = "up"
		case "no-link":
			reading.Link = "down"
		}
		if reading.Link == "up" {
			reading.RateMbps, err = parseRate(row["rate"])
			if err != nil {
				return snapshot, err
			}
			reading.FullDuplex, err = parseBool(row["full-duplex"])
			if err != nil {
				return snapshot, err
			}
		}
		snapshot.Ports = append(snapshot.Ports, reading)
	}
	snapshot.Capability = "supported"
	return snapshot, nil
}

// PollPoE reads configuration and actual measurements separately. Missing fields
// remain unknown, and a missing menu is a capability limitation rather than zero.
func (c *Collector) PollPoE(ctx context.Context, now time.Time) (plan.PortSnapshot, error) {
	snapshot := c.base("poe", now)
	if c.source.PoE == "disabled" || c.source.PoE == "unsupported" {
		snapshot.Capability = c.source.PoE
		return snapshot, nil
	}
	configured, err := c.client.Read(ctx, poeMenu, ".id,name,poe-out,poe-voltage")
	if unavailable(err) {
		snapshot.Capability = "unavailable"
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	if err := c.readInventory(ctx, now); err != nil {
		return snapshot, err
	}
	seen := map[string]bool{}
	for _, row := range configured {
		if row[".id"] == "" || row["name"] == "" || seen[row[".id"]] {
			return snapshot, fmt.Errorf("incomplete PoE configuration inventory")
		}
		seen[row[".id"]] = true
	}
	rows, err := c.client.Monitor(ctx, poeMenu, inventoryIDs(configured))
	if unavailable(err) {
		snapshot.Capability = "unavailable"
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	indexed, err := monitorByName(rows)
	if err != nil || len(indexed) != len(configured) {
		return snapshot, fmt.Errorf("incomplete PoE monitor snapshot")
	}
	native := map[string]routeros.Row{}
	for _, item := range c.inventory {
		native[item["name"]] = item
	}
	for _, item := range configured {
		if item[".id"] == "" || item["name"] == "" || native[item["name"]] == nil || indexed[item["name"]] == nil {
			return snapshot, fmt.Errorf("PoE interface mapping changed; retry collection")
		}
		reading, err := c.reading(native[item["name"]])
		if err != nil {
			return snapshot, err
		}
		status, err := parsePoE(item, indexed[item["name"]])
		if err != nil {
			return snapshot, err
		}
		reading.PoE = status
		snapshot.Ports = append(snapshot.Ports, reading)
	}
	snapshot.Capability = "supported"
	return snapshot, nil
}
