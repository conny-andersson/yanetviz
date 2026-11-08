package plan

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

func sampleDHCP(p Plan) DHCPObservation {
	return DHCPObservation{Interface: p.Interfaces[0].ID, Domain: "home", Source: p.Devices[5].ID, Server: "lan", MACAddress: "02:11:22:33:44:55", ClientID: "01:Ab:00:FF", ClientIDField: "active-client-id", Address: "192.0.2.10", Hostname: "observed-name", Status: "bound", FirstSeen: "2026-11-08T10:00:00Z", LastSeen: "2026-11-08T10:00:00Z"}
}

func openIdentityStore(t *testing.T) (*Store, Plan) {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "plan.db"), identityPlan())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return store, document
}

func TestObservationsSurvivePlanEditsWithoutChangingDocumentation(t *testing.T) {
	store, document := openIdentityStore(t)
	observation := sampleDHCP(document)
	if err := store.RecordDHCPObservations([]DHCPObservation{observation}); err != nil {
		t.Fatal(err)
	}
	unchanged, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(document, unchanged) {
		t.Fatal("collection modified the plan or revision")
	}
	document.Devices[4].Name = "My chosen name"
	document.Devices[4].Floor = "entrance"
	if err := store.Save(document); err != nil {
		t.Fatal(err)
	}
	actual, err := store.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, []DHCPObservation{observation}) {
		t.Fatalf("plan edit lost observation: %#v", actual)
	}
	if err := store.RecordDHCPObservations([]DHCPObservation{}); err != nil {
		t.Fatal(err)
	}
	actual, err = store.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(actual) != 1 {
		t.Fatal("empty collection erased observation")
	}
}

func TestOutOfOrderSamplesAndClientIDsRemainSeparateEvidence(t *testing.T) {
	store, document := openIdentityStore(t)
	original := sampleDHCP(document)
	newer := original
	newer.LastSeen = "2026-11-08T11:00:00Z"
	newer.Address = "192.0.2.11"
	older := original
	older.FirstSeen = "2026-11-08T09:00:00Z"
	older.LastSeen = older.FirstSeen
	older.Address = "192.0.2.9"
	for _, record := range []DHCPObservation{original, newer, older} {
		if err := store.RecordDHCPObservations([]DHCPObservation{record}); err != nil {
			t.Fatal(err)
		}
	}
	actual, err := store.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(actual) != 1 || actual[0].Address != newer.Address || actual[0].FirstSeen != older.FirstSeen || actual[0].LastSeen != newer.LastSeen || actual[0].ClientID != original.ClientID {
		t.Fatalf("incorrect observation reconciliation: %#v", actual)
	}
	changedID := newer
	changedID.ClientID = "  opaque-ID  "
	if err := store.RecordDHCPObservations([]DHCPObservation{changedID}); err != nil {
		t.Fatal(err)
	}
	actual, err = store.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(actual) != 2 || actual[1].ClientID != "  opaque-ID  " {
		t.Fatal("client ID history or opaque representation was lost")
	}
}

func TestInvalidObservationBatchIsAtomic(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*DHCPObservation)
	}{
		{"unowned MAC", func(o *DHCPObservation) { o.MACAddress = "02:11:22:33:44:66" }},
		{"unknown interface", func(o *DHCPObservation) { o.Interface = "missing" }},
		{"unknown source", func(o *DHCPObservation) { o.Source = "missing" }},
		{"different domain", func(o *DHCPObservation) { o.Domain = "guest" }},
		{"invalid IP", func(o *DHCPObservation) { o.Address = "not-an-IP" }},
		{"invalid times", func(o *DHCPObservation) { o.LastSeen = "2026-11-07T10:00:00Z" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, document := openIdentityStore(t)
			good := sampleDHCP(document)
			bad := good
			tc.mutate(&bad)
			if err := store.RecordDHCPObservations([]DHCPObservation{good, bad}); err == nil {
				t.Fatal("invalid observation accepted")
			}
			actual, err := store.DHCPObservations()
			if err != nil {
				t.Fatal(err)
			}
			if len(actual) != 0 {
				t.Fatal("invalid batch partially committed")
			}
		})
	}
}

func TestIdentityChangesPruneOnlyDependentEvidence(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Plan)
	}{
		{"interface deleted", func(p *Plan) { p.Interfaces = nil }},
		{"MAC removed", func(p *Plan) { p.Interfaces[0].MACAddresses = nil }},
		{"domain changed", func(p *Plan) { p.Interfaces[0].Domain = "guest" }},
		{"source removed", func(p *Plan) { p.Devices = append(p.Devices[:5], p.Devices[6:]...) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, document := openIdentityStore(t)
			record := sampleDHCP(document)
			if err := store.RecordDHCPObservations([]DHCPObservation{record}); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&document)
			if err := store.Save(document); err != nil {
				t.Fatal(err)
			}
			actual, err := store.DHCPObservations()
			if err != nil {
				t.Fatal(err)
			}
			if len(actual) != 0 {
				t.Fatal("orphaned observation retained")
			}
		})
	}
}

func TestSchemaOneUpgradePreservesExistingPlan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	store, err := Open(path, Seed())
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	before.Devices[0].Name = "Legacy documented router"
	if err := store.Save(before); err != nil {
		t.Fatal(err)
	}
	before, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("DROP TABLE dhcp_observations; DROP TABLE interfaces; PRAGMA user_version=1;"); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(path, Seed())
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	after, err := upgraded.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("schema upgrade changed existing documentation")
	}
	var version int
	if err := upgraded.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 4 {
		t.Fatalf("schema did not upgrade: %d", version)
	}
}

func TestObservationsAndInterfaceIdentitiesSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.db")
	store, err := Open(path, identityPlan())
	if err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	record := sampleDHCP(document)
	if err := store.RecordDHCPObservations([]DHCPObservation{record}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path, Seed())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored, err := reopened.Load()
	if err != nil {
		t.Fatal(err)
	}
	records, err := reopened.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, document) || !reflect.DeepEqual(records, []DHCPObservation{record}) {
		t.Fatal("interface identities or observations changed after restart")
	}
}

func TestSharedDHCPClientIDDoesNotMergeDifferentInterfaces(t *testing.T) {
	store, document := openIdentityStore(t)
	second := document.Interfaces[0]
	second.ID = NewID()
	second.Device = document.Devices[6].ID
	second.MACAddresses = []string{"02:11:22:33:44:77"}
	document.Interfaces = append(document.Interfaces, second)
	if err := store.Save(document); err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	first := sampleDHCP(document)
	another := first
	another.Interface = second.ID
	another.MACAddress = second.MACAddresses[0]
	if err := store.RecordDHCPObservations([]DHCPObservation{first, another}); err != nil {
		t.Fatal(err)
	}
	records, err := store.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Interface == records[1].Interface {
		t.Fatal("shared client ID caused identities to merge")
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(document, after) {
		t.Fatal("collection merged or edited the plan")
	}
}

func TestPortReferenceSurvivesReopenAndVersionTwoUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version-two.db")
	document := identityPlan()
	physical := Port{ID: NewID(), Device: document.Interfaces[0].Device, Name: "ether1", PoEMode: "unknown"}
	document.Ports = append(document.Ports, physical)
	document.Interfaces[0].Port = physical.ID
	store, err := Open(path, document)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	record := sampleDHCP(before)
	if err := store.RecordDHCPObservations([]DHCPObservation{record}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("ALTER TABLE interfaces DROP COLUMN port; PRAGMA user_version=2;"); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(path, Seed())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	after, err := restored.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("port reference or documentation changed during schema upgrade")
	}
	evidence, err := restored.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(evidence, []DHCPObservation{record}) {
		t.Fatal("DHCP evidence changed during schema upgrade")
	}
	var reference string
	if err := restored.db.QueryRow("SELECT port FROM interfaces WHERE id=?", document.Interfaces[0].ID).Scan(&reference); err != nil {
		t.Fatal(err)
	}
	if reference != physical.ID {
		t.Fatal("port foreign-key column was not backfilled")
	}
	if _, err := restored.db.Exec("UPDATE interfaces SET port='missing' WHERE id=?", document.Interfaces[0].ID); err == nil {
		t.Fatal("database accepted a dangling port foreign key")
	}
	after.Interfaces[0].Port = ""
	after.Ports = after.Ports[:len(after.Ports)-1]
	if err := restored.Save(after); err != nil {
		t.Fatal(err)
	}
	evidence, err = restored.DHCPObservations()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(evidence, []DHCPObservation{record}) {
		t.Fatal("unlinking a deleted port erased DHCP evidence")
	}
}
