package main

import (
	"net/http"
	"time"
	"yanetviz/internal/plan"
)

func (s *server) portView() (plan.PortView, error) {
	document, err := s.store.Load()
	if err != nil {
		return plan.PortView{}, err
	}
	snapshots, err := s.store.PortSnapshots()
	if err != nil {
		return plan.PortView{}, err
	}
	for _, pending := range s.portPending {
		found := false
		for i, snapshot := range snapshots {
			if snapshot.Source == pending.Source && snapshot.Channel == pending.Channel {
				if pending.Capability == "unsupported" || pending.Capability == "disabled" {
					snapshots[i].Capability = pending.Capability
					snapshots[i].Error = ""
				}
				found = true
				break
			}
		}
		if !found {
			snapshots = append(snapshots, pending)
		}
	}
	return plan.ReconcilePorts(document, snapshots, time.Now(), s.wifiStarted, s.portEnabled), nil
}

func (s *server) portObservations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	view, err := s.portView()
	if err != nil {
		http.Error(w, "could not load link and PoE observations", 500)
		return
	}
	output(w, http.StatusOK, view)
}

func (s *server) portEvents(w http.ResponseWriter, r *http.Request) {
	s.observationEvents(w, r, func() (any, error) { return s.portView() })
}
