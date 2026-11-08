package wifi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollectorUsesRadioIdentityAndReadOnlyProperties(t *testing.T) {
	ap := "physical-ap"
	radio := "02:11:22:33:44:55"
	requests := []string{}
	var mu sync.Mutex
	master := "radio-one"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, r.URL.Path)
		if r.Method != http.MethodGet {
			t.Error("collector attempted a router mutation")
		}
		username, password, ok := r.BasicAuth()
		if !ok || username != "monitor" || password != "private-password" {
			t.Error("missing source credentials")
		}
		properties := r.URL.Query().Get(".proplist")
		if properties == "" || strings.Contains(properties, "security") {
			t.Error("collector requested unbounded configuration")
		}
		switch r.URL.Path {
		case "/rest/" + registrationMenu:
			json.NewEncoder(w).Encode([]row{{"mac-address": "02:AA:BB:CC:DD:01", "interface": "guest", "authorized": "true", "rx-rate": "54000000", "signal": "-62"}, {"mac-address": "02:AA:BB:CC:DD:02", "interface": "guest", "authorized": "false"}})
		case "/rest/" + radioMenu:
			json.NewEncoder(w).Encode([]row{{"radio-mac": radio, "interface": master, "cap": "cap-identity"}})
		case "/rest/" + interfaceMenu:
			json.NewEncoder(w).Encode([]row{{"name": master, "radio-mac": radio}, {"name": "guest", "master-interface": master}})
		default:
			t.Errorf("unexpected router path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	source := Source{ID: "controller", Device: "router", Domain: "home", Username: "monitor", password: "private-password", endpoint: server.URL + "/rest/", Radios: map[string]string{radio: ap}}
	collector := NewCollector(source)
	now := time.Now()
	snapshot, err := collector.Poll(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Registrations) != 2 || snapshot.Registrations[0].AP != ap || snapshot.Registrations[0].RadioMAC != radio {
		t.Fatalf("wrong physical mapping: %#v", snapshot.Registrations)
	}
	if !snapshot.Registrations[0].Authorized || snapshot.Registrations[1].Authorized {
		t.Fatal("registration authorization was discarded")
	}
	if *snapshot.Registrations[0].RxRate != 54000000 || *snapshot.Registrations[0].Signal != -62 || snapshot.Registrations[1].Signal != nil || snapshot.Registrations[1].RxRate != nil {
		t.Fatal("metrics lost their units or unknown values")
	}
	// Renaming/provisioning must refresh joins rather than rely on interface names.
	mu.Lock()
	master = "new-radio-name"
	mu.Unlock()
	if _, err := collector.Poll(context.Background(), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 6 {
		t.Fatal("radio joins were not refreshed")
	}
}

func TestCollectorRejectsIncompleteSnapshotsAndNeverFollowsRedirects(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"empty complete", `[]`, 200},
		{"null", `null`, 200},
		{"object", `{"error":400}`, 200},
		{"truncated", `[{"mac-address":`, 200},
		{"missing authorization", `[{"mac-address":"02:AA:BB:CC:DD:01","interface":"wifi1"}]`, 200},
		{"authentication", `private server response`, 401},
		{"permission", `private server response`, 403},
		{"unsupported", `private server response`, 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/rest/"+registrationMenu {
					w.WriteHeader(test.status)
					w.Write([]byte(test.body))
					return
				}
				w.Write([]byte(`[]`))
			}))
			defer server.Close()
			snapshot, err := NewCollector(Source{endpoint: server.URL + "/rest/"}).Poll(context.Background(), time.Now())
			if (err == nil) != (test.name == "empty complete") {
				t.Fatalf("unexpected snapshot result: %#v, %v", snapshot, err)
			}
			if err != nil && strings.Contains(err.Error(), "private server response") {
				t.Fatal("router response leaked into health")
			}
		})
	}
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Store(true) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	_, err := NewCollector(Source{endpoint: source.URL + "/rest/"}).Poll(context.Background(), time.Now())
	if err == nil || redirected.Load() {
		t.Fatal("collector followed a redirect")
	}
}
