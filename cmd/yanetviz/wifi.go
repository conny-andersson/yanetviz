package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"time"
	"yanetviz/internal/plan"
)

func (s *server) wifiView() (plan.WiFiView, error) {
	document, err := s.store.Load()
	if err != nil {
		return plan.WiFiView{}, err
	}
	snapshots, err := s.store.WiFiSnapshots()
	if err != nil {
		return plan.WiFiView{}, err
	}
	for _, source := range s.wifiPending {
		found := false
		for _, snapshot := range snapshots {
			if snapshot.Source == source.Source {
				found = true
				break
			}
		}
		if !found {
			snapshots = append(snapshots, source)
		}
	}
	return plan.ReconcileWiFi(document, snapshots, time.Now(), s.wifiStarted, s.wifiEnabled), nil
}

func (s *server) wifiObservations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	view, err := s.wifiView()
	if err != nil {
		http.Error(w, "could not load Wi-Fi observations", 500)
		return
	}
	output(w, http.StatusOK, view)
}

// wifiEvents sends derived observation snapshots only; it never reloads the
// user's working plan. Reconnecting starts with a complete current view.
func (s *server) wifiEvents(w http.ResponseWriter, r *http.Request) {
	s.observationEvents(w, r, func() (any, error) { return s.wifiView() })
}

// observationEvents streams read-only snapshots without replacing pending plans.
func (s *server) observationEvents(w http.ResponseWriter, r *http.Request, read func() (any, error)) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	// Recheck authentication on reconnect, rather than keeping expired sessions.
	lifetime := time.NewTimer(time.Minute)
	defer lifetime.Stop()
	var previous any
	for {
		view, err := read()
		if err != nil {
			log.Printf("load stream observations: %v", err)
			return
		}
		// Streaming extends its write deadline only while the client receives data.
		if err := controller.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return
		}
		if !reflect.DeepEqual(previous, view) {
			raw, err := json.Marshal(view)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
				return
			}
			previous = view
		} else if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-lifetime.C:
			return
		case <-ticker.C:
		}
	}
}
