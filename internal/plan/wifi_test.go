package plan

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestWiFiEvidenceSurvivesFailureReopenAndPlanEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wifi.db")
	p := Seed()
	p.Interfaces = []NetworkInterface{{ID: "test-interface", Device: p.Devices[4].ID, Name: "Wi-Fi", Domain: "home", MACAddresses: []string{"02:AA:BB:CC:DD:01"}}}
	store, err := Open(path, p)
	if err != nil {
		t.Fatal(err)
	}
	p, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	snapshot := WiFiSnapshot{Source: "controller", Device: p.Devices[0].ID, Domain: "home", LastAttempt: now.Format(time.RFC3339Nano), Radios: []WiFiRadio{{MAC: "02:11:22:33:44:55", AP: p.Devices[5].ID}}, Registrations: []WiFiRegistration{{MAC: "02:AA:BB:CC:DD:01", InterfaceName: "wifi1", AP: p.Devices[5].ID}}}
	if err := store.RecordWiFiSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, p) {
		t.Fatal("polling changed the physical plan")
	}
	// Failure must retain the last successful registration, even with partial rows.
	failure := snapshot
	failure.LastAttempt = now.Add(15 * time.Second).Format(time.RFC3339Nano)
	failure.Error = "connection failed"
	failure.Registrations = nil
	if err := store.RecordWiFiSnapshot(failure); err != nil {
		t.Fatal(err)
	}
	records, err := store.WiFiSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(records[0].Registrations) != 1 || !records[0].Registrations[0].Present || records[0].LastSuccess != snapshot.LastAttempt {
		t.Fatal("failed read erased complete snapshot")
	}
	enabled := map[string]bool{"controller": true}
	view := ReconcileWiFi(p, records, now.Add(20*time.Second), now.Add(-time.Second), enabled)
	if view.Clients[0].State != "associated" || view.Clients[0].Device != p.Devices[4].ID || view.Sources[0].State != "error" {
		t.Fatalf("wrong fresh/health view: %#v", view)
	}
	if ReconcileWiFi(p, records, now.Add(120*time.Second), now.Add(-time.Second), enabled).Clients[0].State != "associated" {
		t.Fatal("60-second polling observations became stale between successful polls")
	}
	if ReconcileWiFi(p, records, now.Add(180*time.Second), now.Add(-time.Second), enabled).Clients[0].State != "stale" {
		t.Fatal("failed evidence did not expire")
	}
	// A complete empty snapshot establishes absence and retains last-seen evidence.
	empty := snapshot
	empty.LastAttempt = now.Add(60 * time.Second).Format(time.RFC3339Nano)
	empty.Registrations = []WiFiRegistration{}
	if err := store.RecordWiFiSnapshot(empty); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordWiFiSnapshot(snapshot); err != nil {
		t.Fatal(err)
	} // Late response ignored.
	p.Devices[4].Name = "User-owned name"
	if err := store.Save(p); err != nil {
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
	records, err = store.WiFiSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	view = ReconcileWiFi(p, records, now.Add(65*time.Second), now.Add(-time.Second), enabled)
	if view.Clients[0].State != "not-associated" || view.Clients[0].LastSeen != snapshot.LastAttempt {
		t.Fatal("complete absence did not preserve last seen")
	}
	if ReconcileWiFi(p, records, now.Add(65*time.Second), now.Add(62*time.Second), enabled).Clients[0].State != "stale" {
		t.Fatal("restart incorrectly restored live status")
	}
	p.Interfaces = nil
	if ReconcileWiFi(p, records, now.Add(65*time.Second), now.Add(-time.Second), enabled).Clients[0].Device != "" {
		t.Fatal("removed MAC identity remained matched")
	}
}

func TestWiFiUnmatchedDomainsConflictsAndUnknownAPs(t *testing.T) {
	p := Seed()
	p.Interfaces = []NetworkInterface{{ID: "wifi-client", Device: p.Devices[4].ID, Name: "Wi-Fi", Domain: "home", MACAddresses: []string{"02:AA:BB:CC:DD:01"}}}
	now := time.Now().UTC()
	snapshot := WiFiSnapshot{Source: "controller", Device: p.Devices[0].ID, Domain: "guest", LastSuccess: now.Format(time.RFC3339Nano), Registrations: []WiFiRegistration{{MAC: "02:AA:BB:CC:DD:01", InterfaceName: "wifi1", AP: "missing-ap", Present: true}}}
	enabled := map[string]bool{"controller": true}
	view := ReconcileWiFi(p, []WiFiSnapshot{snapshot}, now, now.Add(-time.Second), enabled)
	if view.Clients[0].Device != "" || view.Clients[0].AP != "" {
		t.Fatal("unresolved domain or AP was guessed")
	}
	snapshot.Domain = "home"
	snapshot.Registrations = append(snapshot.Registrations, WiFiRegistration{MAC: "02:AA:BB:CC:DD:01", InterfaceName: "wifi2", AP: p.Devices[5].ID, Present: true})
	view = ReconcileWiFi(p, []WiFiSnapshot{snapshot}, now, now.Add(-time.Second), enabled)
	if len(view.Clients) != 2 || view.Clients[0].State != "conflict" || view.Clients[1].State != "conflict" {
		t.Fatal("simultaneous registrations selected an arbitrary AP")
	}
}

func TestSchemaThreeUpgradePreservesRoutesAndDHCP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	store, err := Open(path, identityPlan())
	if err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	document.Cables[0].Sections = []RouteSection{{Floor: document.Floors[0].ID, Exit: &RoutePoint{.2, .7}, Confirmed: true}}
	if err := store.Save(document); err != nil {
		t.Fatal(err)
	}
	document.Revision++
	observation := sampleDHCP(document)
	if err := store.RecordDHCPObservations([]DHCPObservation{observation}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("DROP TABLE wifi_sources; PRAGMA user_version=3;"); err != nil {
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
	evidence, err := upgraded.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, document) || !reflect.DeepEqual(evidence, []DHCPObservation{observation}) {
		t.Fatal("schema upgrade lost manual routes or DHCP evidence")
	}
	snapshots, err := upgraded.WiFiSnapshots()
	if err != nil || len(snapshots) != 0 {
		t.Fatalf("new observation table not empty: %v", err)
	}
}
