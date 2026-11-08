package portpoll

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"yanetviz/internal/plan"
)

func TestPortConfigRejectsForeignOrAmbiguousBindings(t *testing.T) {
	document := plan.Seed()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "password"), []byte("test-password"), 0600); err != nil {
		t.Fatal(err)
	}
	source := Source{ID: "switch", Device: document.Ports[0].Device, Address: "192.0.2.1", Username: "monitor", PasswordFile: "password", Ports: map[string]string{"ether1": document.Ports[0].ID}}
	path := filepath.Join(dir, "monitor.json")
	save := func(sources []Source) {
		raw, err := json.Marshal(Config{Sources: sources})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save([]Source{source})
	config, err := LoadConfig(path, document)
	if err != nil || config.Sources[0].PoE != "auto" || config.PollSeconds != 60 || config.Sources[0].endpoint != "https://192.0.2.1/rest/" {
		t.Fatalf("invalid secure defaults: %v", err)
	}
	source.PoE = "unsupported"
	save([]Source{source})
	if _, err := LoadConfig(path, document); err != nil {
		t.Fatal("explicit unsupported policy was rejected")
	}
	source.PoE = "invalid-policy"
	save([]Source{source})
	if _, err := LoadConfig(path, document); err == nil {
		t.Fatal("unknown PoE policy was accepted")
	}
	source.PoE = "auto"
	duplicate := source
	duplicate.ID = "other-source"
	save([]Source{source, duplicate})
	if _, err := LoadConfig(path, document); err == nil {
		t.Fatal("accepted competing sources for one device")
	}
	source.Ports["ether2"] = document.Ports[0].ID
	save([]Source{source})
	if _, err := LoadConfig(path, document); err == nil {
		t.Fatal("bound one physical terminal to two native ports")
	}
	delete(source.Ports, "ether2")
	document.Ports = append(document.Ports, plan.Port{ID: "foreign-port", Device: document.Devices[0].ID, Name: "foreign", PoEMode: "unknown"})
	source.Ports["ether1"] = "foreign-port"
	save([]Source{source})
	if _, err := LoadConfig(path, document); err == nil {
		t.Fatal("accepted a port belonging to another device")
	}
}
