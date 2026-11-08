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

func TestWiFiAPIAndEventsDoNotChangePlanOrExposeCredentials(t *testing.T) {
	s := testServer(t)
	document, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	s.wifiStarted = now.Add(-time.Second)
	s.wifiEnabled = map[string]bool{"controller": true}
	snapshot := plan.WiFiSnapshot{Source: "controller", Device: document.Devices[0].ID, Domain: "home", LastAttempt: now.Format(time.RFC3339Nano), Registrations: []plan.WiFiRegistration{{MAC: "02:AA:BB:CC:DD:01", InterfaceName: "wifi1", AP: document.Devices[4].ID}}}
	if err := s.store.RecordWiFiSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	s.wifiObservations(response, httptest.NewRequest(http.MethodGet, "/api/observations/wifi", nil))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var view plan.WiFiView
	if err := json.Unmarshal(response.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Clients) != 1 || view.Clients[0].Device != "" || view.Clients[0].State != "associated" {
		t.Fatal("unmatched client was dropped or guessed")
	}
	if strings.Contains(response.Body.String(), "password") || strings.Contains(response.Body.String(), "address") {
		t.Fatal("source configuration leaked into API")
	}
	// A stream sends a current snapshot immediately, then exits on cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := httptest.NewRecorder()
	s.wifiEvents(stream, httptest.NewRequest(http.MethodGet, "/api/observations/wifi/events", nil).WithContext(ctx))
	if !strings.HasPrefix(stream.Body.String(), "data: ") || stream.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatal("missing complete initial stream snapshot")
	}
	after, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, document) {
		t.Fatal("observation API changed documentation")
	}
	s.auth = true
	denied := httptest.NewRecorder()
	s.protect(s.wifiObservations)(denied, httptest.NewRequest(http.MethodGet, "/api/observations/wifi", nil))
	if denied.Code != 401 {
		t.Fatal("Wi-Fi evidence bypassed authentication")
	}
}
