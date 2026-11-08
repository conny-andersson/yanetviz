package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"yanetviz/internal/plan"
)

func TestPortAPIAndCombinedStreamPreserveManualPlan(t *testing.T) {
	s := testServer(t)
	document, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	s.wifiStarted = now.Add(-time.Second)
	s.portEnabled = map[string]bool{"switch": true}
	reading := plan.PortReading{Port: document.Ports[0].ID, Name: document.Ports[0].Name, DefaultName: "ether1", Link: "up"}
	snapshot := plan.PortSnapshot{Source: "switch", Device: document.Ports[0].Device, Channel: "link", Capability: "supported", LastAttempt: now.Format(time.RFC3339Nano), Ports: []plan.PortReading{reading}}
	if err := s.store.RecordPortSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	s.liveObservations(response, httptest.NewRequest(http.MethodGet, "/api/observations/live", nil))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var view liveView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Ports.Readings) != 1 || view.Ports.Readings[0].State != "fresh" || view.Ports.Readings[0].Port != reading.Port {
		t.Fatal("port binding or current state missing")
	}
	for _, private := range []string{"password", "username", "passwordFile", "endpoint"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatal("private configuration entered public observations")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := httptest.NewRecorder()
	s.liveEvents(stream, httptest.NewRequest(http.MethodGet, "/api/observations/events", nil).WithContext(ctx))
	if !strings.HasPrefix(stream.Body.String(), "data: ") || !strings.Contains(stream.Body.String(), `"wifi":`) || !strings.Contains(stream.Body.String(), `"ports":`) {
		t.Fatal("combined stream did not send complete initial views")
	}
	after, err := s.store.Load()
	if err != nil || !reflect.DeepEqual(after, document) {
		t.Fatal("observations changed the plan or revision")
	}
	s.auth = true
	denied := httptest.NewRecorder()
	s.protect(s.portObservations)(denied, httptest.NewRequest(http.MethodGet, "/api/observations/ports", nil))
	if denied.Code != 401 {
		t.Fatal("port observations bypassed authentication")
	}
}

func TestConfiguredPoEOverrideTakesPrecedenceOverHistoricalErrors(t *testing.T) {
	s := testServer(t)
	p, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := plan.PortSnapshot{Source: "ap", Device: p.Devices[0].ID, Channel: "poe", LastAttempt: time.Now().UTC().Format(time.RFC3339Nano), Error: "authentication failed"}
	if err := s.store.RecordPortSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	s.portEnabled = map[string]bool{"ap": true}
	s.portPending = []plan.PortSnapshot{{Source: "ap", Device: snapshot.Device, Channel: "poe", Capability: "unsupported"}}
	view, err := s.portView()
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Sources) != 1 || view.Sources[0].State != "unsupported" || view.Sources[0].Error != "" {
		t.Fatal("known hardware limitation was presented as a collection failure")
	}
}
