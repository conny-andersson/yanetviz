package main

import (
	"net/http"
	"yanetviz/internal/plan"
)

type liveView struct {
	WiFi  plan.WiFiView `json:"wifi"`
	Ports plan.PortView `json:"ports"`
}

func (s *server) liveView() (liveView, error) {
	wifi, err := s.wifiView()
	if err != nil {
		return liveView{}, err
	}
	ports, err := s.portView()
	if err != nil {
		return liveView{}, err
	}
	return liveView{WiFi: wifi, Ports: ports}, nil
}

func (s *server) liveObservations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	view, err := s.liveView()
	if err != nil {
		http.Error(w, "could not load observations", 500)
		return
	}
	output(w, 200, view)
}

func (s *server) liveEvents(w http.ResponseWriter, r *http.Request) {
	s.observationEvents(w, r, func() (any, error) { return s.liveView() })
}
