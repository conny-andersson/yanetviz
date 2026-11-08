package wifi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"yanetviz/internal/plan"
)

func TestConfigRequiresExplicitHTTPAndValidPhysicalMappings(t *testing.T) {
	document := plan.Seed()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "password"), []byte("not-a-real-credential\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source := Source{ID: "controller", Device: document.Devices[0].ID, Domain: " HOME ", Address: "192.0.2.1", Username: "monitor", PasswordFile: "password", Radios: map[string]string{"02-11-22-33-44-55": document.Devices[4].ID}}
	path := filepath.Join(dir, "wifi.json")
	save := func(source Source) {
		raw, err := json.Marshal(Config{Sources: []Source{source}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save(source)
	config, err := LoadConfig(path, document)
	if err != nil {
		t.Fatal(err)
	}
	configured := config.Sources[0]
	if configured.endpoint != "https://192.0.2.1/rest/" || configured.Domain != "home" || configured.password != "not-a-real-credential" || config.PollSeconds != 15 {
		t.Fatal("secure defaults or credential-file resolution failed")
	}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "not-a-real-credential") {
		t.Fatal("password entered serializable configuration")
	}
	source.Transport = "rest-http"
	save(source)
	config, err = LoadConfig(path, document)
	if err != nil || config.Sources[0].endpoint != "http://192.0.2.1/rest/" {
		t.Fatal("explicit HTTP choice was not honored")
	}
	for _, address := range []string{"user:password@192.0.2.1", "192.0.2.1/rest", "192.0.2.1?query=1", "https://192.0.2.1"} {
		source.Address = address
		save(source)
		if _, err := LoadConfig(path, document); err == nil {
			t.Fatalf("accepted invalid management host %q", address)
		}
	}
	source.Address = "192.0.2.1"
	source.Radios["02-11-22-33-44-55"] = "nonexistent-device"
	save(source)
	if _, err := LoadConfig(path, document); err == nil {
		t.Fatal("accepted an unknown physical AP")
	}
}
