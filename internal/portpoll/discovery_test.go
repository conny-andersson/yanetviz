package portpoll

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfiguredPoESupportSkipsOnlyPoERequests(t *testing.T) {
	for _, policy := range []string{"unsupported", "disabled"} {
		t.Run(policy, func(t *testing.T) {
			var linkRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "poe") {
					t.Error("configured PoE override still sent a device request")
				}
				linkRequests.Add(1)
				if r.URL.Path == "/rest/interface/ethernet" {
					w.Write([]byte(`[{".id":"*1","name":"ether1"}]`))
					return
				}
				w.Write([]byte(`[{"name":"ether1","status":"link-ok","rate":"1Gbps"}]`))
			}))
			defer server.Close()
			collector := NewCollector(Source{PoE: policy, endpoint: server.URL + "/rest/"})
			now := time.Now()
			power, err := collector.PollPoE(context.Background(), now)
			if err != nil || power.Capability != policy || linkRequests.Load() != 0 {
				t.Fatal("PoE override was not applied before network access")
			}
			link, err := collector.PollLink(context.Background(), now)
			if err != nil || link.Capability != "supported" || linkRequests.Load() != 2 {
				t.Fatalf("PoE policy disabled healthy link polling: %v", err)
			}
		})
	}
}

func TestDiscoveryReadsOnlyModelAndRouterOSProperties(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet {
			t.Error("hardware discovery attempted a mutation")
		}
		switch r.URL.Path {
		case "/rest/system/resource":
			if r.URL.Query().Get(".proplist") != "board-name,architecture-name,version" {
				t.Error("unexpected resource properties")
			}
			w.Write([]byte(`{"board-name":"wAP","architecture-name":"mipsbe","version":"7.20.1"}`))
		case "/rest/system/routerboard":
			if r.URL.Query().Get(".proplist") != "model" {
				t.Error("discovery requested serial numbers or other unnecessary properties")
			}
			w.Write([]byte(`[{"model":"TEST-wAP"}]`))
		default:
			t.Error("unexpected discovery request")
		}
	}))
	defer server.Close()
	collector := NewCollector(Source{endpoint: server.URL + "/rest/"})
	now := time.Now()
	collector.Discover(context.Background(), now)
	if collector.metadata == nil || collector.metadata.Model != "TEST-wAP" || collector.metadata.OSVersion != "7.20.1" || collector.metadata.Architecture != "mipsbe" {
		t.Fatalf("hardware properties were not decoded: %#v", collector.metadata)
	}
	collector.Discover(context.Background(), now.Add(time.Minute))
	if requests.Load() != 2 {
		t.Fatal("hardware inventory was refreshed more than once in five minutes")
	}
	collector.Discover(context.Background(), now.Add(5*time.Minute))
	if requests.Load() != 4 {
		t.Fatal("hardware inventory did not refresh after five minutes")
	}
}
