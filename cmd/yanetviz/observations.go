package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"yanetviz/internal/plan"
)

func (s *server) dhcpObservations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		records, err := s.store.DHCPObservations()
		if err != nil {
			http.Error(w, "could not load DHCP observations", http.StatusInternalServerError)
			return
		}
		output(w, http.StatusOK, records)
	case http.MethodPost:
		var records []plan.DHCPObservation
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&records); err != nil || records == nil {
			http.Error(w, "expected a DHCP observation array", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			http.Error(w, "expected one JSON document", http.StatusBadRequest)
			return
		}
		if err := s.store.RecordDHCPObservations(records); err != nil {
			if errors.Is(err, plan.ErrInvalidDHCPObservation) {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				log.Printf("record DHCP observations: %v", err)
				http.Error(w, "could not save DHCP observations", http.StatusInternalServerError)
			}
			return
		}
		output(w, http.StatusOK, map[string]int{"recorded": len(records)})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
