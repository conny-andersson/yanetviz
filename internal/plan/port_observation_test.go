package plan

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPortFailuresMissingInventoryAndReopenPreserveDocumentation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ports.db")
	store, err := Open(path, Seed())
	if err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 8, 14, 0, 0, 0, time.UTC)
	device := document.Ports[0].Device
	reading := PortReading{Port: document.Ports[0].ID, Name: document.Ports[0].Name, DefaultName: "ether1", Link: "up"}
	link := PortSnapshot{Source: "switch", Device: device, Channel: "link", Capability: "supported", LastAttempt: now.Format(time.RFC3339Nano), Ports: []PortReading{reading}}
	if err := store.RecordPortSnapshot(link); err != nil {
		t.Fatal(err)
	}
	power := link
	power.Channel = "poe"
	power.Ports = []PortReading{{Port: reading.Port, Name: reading.Name, DefaultName: "ether1", PoE: &ObservedPoE{Mode: "forced", VendorMode: "forced-on", Status: "powered-on"}}}
	if err := store.RecordPortSnapshot(power); err != nil {
		t.Fatal(err)
	}
	power.LastAttempt = now.Add(15 * time.Second).Format(time.RFC3339Nano)
	power.Error = "permission denied"
	power.Ports = nil
	if err := store.RecordPortSnapshot(power); err != nil {
		t.Fatal(err)
	}
	// Healthy link updates continue even when PoE is failing.
	link.LastAttempt = now.Add(200 * time.Second).Format(time.RFC3339Nano)
	if err := store.RecordPortSnapshot(link); err != nil {
		t.Fatal(err)
	}
	snapshots, err := store.PortSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	view := ReconcilePorts(document, snapshots, now.Add(204*time.Second), now.Add(-time.Second), map[string]bool{"switch": true})
	if view.Readings[0].State != "fresh" || view.Readings[1].State != "stale" || view.Readings[1].PoE.Status != "powered-on" || view.Readings[1].PoE.PowerW != nil {
		t.Fatalf("channel health or missing metrics lost: %#v", view)
	}
	actual, err := store.Load()
	if err != nil || !reflect.DeepEqual(actual, document) {
		t.Fatal("polling edited manual documentation or its revision")
	}
	// Complete missing inventory explicitly invalidates current status, not cabling.
	link.LastAttempt = now.Add(240 * time.Second).Format(time.RFC3339Nano)
	link.Ports = []PortReading{}
	if err := store.RecordPortSnapshot(link); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path, Seed())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	snapshots, err = store.PortSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	view = ReconcilePorts(document, snapshots, now.Add(244*time.Second), now.Add(-time.Second), map[string]bool{"switch": true})
	if view.Readings[0].State != "missing" || view.Readings[0].Link != "up" || view.Readings[0].LastSeen != now.Add(200*time.Second).Format(time.RFC3339Nano) {
		t.Fatal("missing inventory was converted to link-down or lost historical evidence")
	}
	restarted := ReconcilePorts(document, snapshots, now.Add(244*time.Second), now.Add(240*time.Second), map[string]bool{"switch": true})
	if restarted.Readings[0].State != "stale" {
		t.Fatal("reopened observations incorrectly claim live status")
	}
	document.Ports[0].Device = document.Devices[5].ID
	unmatched := ReconcilePorts(document, snapshots, now.Add(244*time.Second), now.Add(-time.Second), map[string]bool{"switch": true})
	if unmatched.Readings[0].Port != "" {
		t.Fatal("reassigned documentation retained a foreign observation binding")
	}
}

func TestSchemaFourUpgradePreservesPlanDHCPAndWiFi(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	store, err := Open(path, identityPlan())
	if err != nil {
		t.Fatal(err)
	}
	p, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	p.Cables[0].Sections = []RouteSection{{Floor: p.Floors[0].ID, Exit: &RoutePoint{.2, .7}, Confirmed: true}}
	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}
	p.Revision++
	dhcp := sampleDHCP(p)
	if err := store.RecordDHCPObservations([]DHCPObservation{dhcp}); err != nil {
		t.Fatal(err)
	}
	wifi := WiFiSnapshot{Source: "controller", Device: p.Devices[0].ID, Domain: "home", LastAttempt: "2026-11-08T14:00:00Z", Registrations: []WiFiRegistration{{MAC: "02:AA:BB:CC:DD:01", InterfaceName: "wifi1"}}}
	if err := store.RecordWiFiSnapshot(wifi); err != nil {
		t.Fatal(err)
	}
	before, err := store.WiFiSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("DROP TABLE port_sources; PRAGMA user_version=4;"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(path, Seed())
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	actual, err := upgraded.Load()
	if err != nil {
		t.Fatal(err)
	}
	after, err := upgraded.WiFiSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := upgraded.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, p) || !reflect.DeepEqual(after, before) || !reflect.DeepEqual(evidence, []DHCPObservation{dhcp}) {
		t.Fatal("schema-4 upgrade changed physical plan, routes or retained observations")
	}
}

func TestConfiguredUnsupportedRetainsInventoryAndHistoricalReadings(t *testing.T) {
	store, p := openIdentityStore(t)
	now := time.Now().UTC()
	snapshot := PortSnapshot{Source: "ap", Device: p.Ports[0].Device, Channel: "poe", Capability: "supported", LastAttempt: now.Format(time.RFC3339Nano), Ports: []PortReading{{Name: "ether1", Port: p.Ports[0].ID, PoE: &ObservedPoE{Mode: "auto", Status: "powered-on"}}}, Inventory: &DeviceInventory{Model: "known-model", OSVersion: "7.20.1", LastAttempt: now.Format(time.RFC3339Nano), LastSuccess: now.Format(time.RFC3339Nano)}}
	if err := store.RecordPortSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Capability = "unsupported"
	snapshot.LastAttempt = now.Add(time.Minute).Format(time.RFC3339Nano)
	snapshot.Ports = nil
	snapshot.Inventory = &DeviceInventory{LastAttempt: snapshot.LastAttempt, Error: "authentication failed"}
	if err := store.RecordPortSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	rows, err := store.PortSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Inventory.Model != "known-model" || rows[0].Inventory.OSVersion != "7.20.1" || rows[0].Inventory.Error != "authentication failed" || len(rows[0].Ports) != 1 {
		t.Fatal("failed discovery or policy change erased historical evidence")
	}
	view := ReconcilePorts(p, rows, now.Add(time.Minute), now.Add(-time.Second), map[string]bool{"ap": true})
	if view.Sources[0].State != "unsupported" || view.Readings[0].State != "stale" {
		t.Fatal("configured lack of support was presented as live power or a collection error")
	}
	actual, err := store.Load()
	if err != nil || !reflect.DeepEqual(actual, p) {
		t.Fatal("discovery changed manual documentation")
	}
}
