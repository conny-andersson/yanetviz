package portpoll

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"yanetviz/internal/routeros"
)

func TestLinkAndPoEMeasurementsUseExplicitPhysicalBindings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/interface/ethernet":
			if r.Method != "GET" || r.URL.Query().Get(".proplist") == "" {
				t.Error("unbounded or mutating inventory request")
			}
			json.NewEncoder(w).Encode([]routeros.Row{{".id": "*1", "name": "friendly-uplink", "default-name": "ether1", "disabled": "false", "mac-address": "02:11:22:33:44:55"}, {".id": "*2", "name": "ether2", "default-name": "ether2", "disabled": "true"}})
		case "/rest/interface/ethernet/poe":
			json.NewEncoder(w).Encode([]routeros.Row{{".id": "*1", "name": "friendly-uplink", "poe-out": "forced-on", "poe-voltage": "auto"}, {".id": "*2", "name": "ether2", "poe-out": "auto-on"}})
		case "/rest/interface/ethernet/monitor", "/rest/interface/ethernet/poe/monitor":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			once, ok := body["once"]
			if r.Method != "POST" || !ok || once != "" || body["numbers"] != "*1,*2" || len(body) != 2 {
				t.Error("monitor was not a fixed one-shot read")
			}
			if strings.Contains(r.URL.Path, "/poe/") {
				json.NewEncoder(w).Encode([]routeros.Row{{"name": "friendly-uplink", "poe-out": "forced-on", "poe-out-status": "powered-on", "poe-out-voltage": "47.1", "poe-out-current": "94", "poe-out-power": "4.4"}, {"name": "ether2", "poe-out-status": "waiting-for-load"}})
			} else {
				json.NewEncoder(w).Encode([]routeros.Row{{"name": "friendly-uplink", "status": "link-ok", "rate": "2.5Gbps", "full-duplex": "true"}, {"name": "ether2", "status": "no-link", "rate": "1Gbps"}})
			}
		default:
			t.Error("unexpected management request")
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	collector := NewCollector(Source{ID: "switch", Device: "switch-device", endpoint: server.URL + "/rest/", Ports: map[string]string{"ether1": "physical-port-1", "ether2": "physical-port-2"}})
	now := time.Now()
	links, err := collector.PollLink(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if links.Ports[0].Port != "physical-port-1" || *links.Ports[0].RateMbps != 2500 || !*links.Ports[0].FullDuplex {
		t.Fatal("native name binding or link units changed")
	}
	if links.Ports[1].Link != "down" || links.Ports[1].RateMbps != nil || links.Ports[1].FullDuplex != nil || !*links.Ports[1].Disabled {
		t.Fatal("configured/absent speed was presented as negotiated link rate")
	}
	power, err := collector.PollPoE(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	poe := power.Ports[0].PoE
	if power.Ports[0].Port != "physical-port-1" || poe.Mode != "forced" || poe.VendorMode != "forced-on" || poe.Status != "powered-on" || *poe.VoltageV != 47.1 || *poe.CurrentMA != 94 || *poe.PowerW != 4.4 {
		t.Fatalf("wrong PoE normalization: %#v", poe)
	}
	if power.Ports[1].PoE.PowerW != nil || power.Ports[1].PoE.VoltageV != nil || power.Ports[1].PoE.Mode != "auto" {
		t.Fatal("unknown measurements became zero or auto implied powered")
	}
}

func TestIncompleteLinkAndUnavailablePoEAreDistinct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/interface/ethernet":
			w.Write([]byte(`[{".id":"*1","name":"ether1"}]`))
		case "/rest/interface/ethernet/monitor":
			w.Write([]byte(`[]`))
		case "/rest/interface/ethernet/poe":
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	collector := NewCollector(Source{endpoint: server.URL + "/rest/"})
	now := time.Now()
	if _, err := collector.PollLink(context.Background(), now); err == nil {
		t.Fatal("accepted partial monitor list as complete")
	}
	snapshot, err := collector.PollPoE(context.Background(), now)
	if err != nil || snapshot.Capability != "unavailable" {
		t.Fatal("unavailable PoE menu was treated as an empty powered-port list")
	}
}

func TestPortMetricUnitsAndMissingValues(t *testing.T) {
	for _, test := range []struct {
		raw, unit string
		want      float64
	}{{"54.2V", "V", 54.2}, {"449mA", "mA", 449}, {"24.3W", "W", 24.3}, {"0", "W", 0}} {
		actual, err := measurement(test.raw, test.unit)
		if err != nil || actual == nil || *actual != test.want {
			t.Fatalf("wrong metric normalization: %s", test.raw)
		}
	}
	for _, raw := range []string{"NaN", "+Inf", "-1", "not-a-number"} {
		if _, err := measurement(raw, "W"); err == nil {
			t.Fatalf("accepted invalid measurement %s", raw)
		}
	}
	value, err := measurement("", "W")
	if err != nil || value != nil {
		t.Fatal("missing wattage became zero")
	}
}
