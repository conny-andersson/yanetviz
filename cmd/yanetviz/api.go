package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"yanetviz"
	"yanetviz/internal/plan"
)

func (s *server) plan(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		p, err := s.store.Load()
		if err != nil {
			http.Error(w, "could not load plan", 500)
			return
		}
		output(w, 200, p)
	case "PUT":
		var p plan.Plan
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			http.Error(w, "invalid plan JSON or request too large", 400)
			return
		}
		if err := dec.Decode(new(any)); err != io.EOF {
			http.Error(w, "expected one JSON document", 400)
			return
		}
		p = p.NormalizeIdentities()
		if err := p.Validate(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		s.writes.Lock()
		defer s.writes.Unlock()
		if err := s.store.Save(p); err != nil {
			if errors.Is(err, plan.ErrConflict) {
				http.Error(w, err.Error(), 409)
			} else {
				log.Printf("save plan: %v", err)
				http.Error(w, "could not save plan", 500)
			}
			return
		}
		p.Revision++
		output(w, 200, p)
	default:
		w.WriteHeader(405)
	}
}
func (s *server) export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	document, err := s.store.Load()
	if err != nil {
		http.Error(w, "could not load export", http.StatusInternalServerError)
		return
	}
	var archive bytes.Buffer
	if err := writeExport(&archive, document); err != nil {
		log.Printf("export plan: %v", err)
		http.Error(w, "could not create export", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="yanetviz-plan.zip"`)
	if _, err := archive.WriteTo(w); err != nil {
		log.Printf("send export: %v", err)
	}
}

// Build the complete archive before sending headers, so an export failure cannot
// masquerade as a successful download.
func writeExport(destination io.Writer, document plan.Plan) error {
	archive := zip.NewWriter(destination)
	planFile, err := archive.Create("plan.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(planFile).Encode(document); err != nil {
		return err
	}
	included := map[string]bool{}
	for _, floor := range document.Floors {
		if !strings.HasPrefix(floor.Background, "/floorplans/") {
			continue
		}
		name := strings.TrimPrefix(floor.Background, "/")
		if included[name] {
			continue
		}
		included[name] = true
		content, err := yanetviz.Assets.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read floor plan: %w", err)
		}
		file, err := archive.Create(name)
		if err != nil {
			return err
		}
		if _, err := file.Write(content); err != nil {
			return err
		}
	}
	readme, err := archive.Create("README.txt")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(readme, "Yanetviz documentation export. plan.json contains the complete manual plan; uploaded raster backgrounds are embedded as data URLs. Bundled floor plans are included in floorplans/. To restore, select plan.json using Import plan in the editor. No credentials or live observations are included.")
	if err != nil {
		return err
	}
	return archive.Close()
}

// validate checks an imported plan without changing persisted documentation.
func (s *server) validate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var p plan.Plan
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		http.Error(w, "invalid plan JSON or request too large", 400)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "expected one JSON document", 400)
		return
	}
	p = p.NormalizeIdentities()
	if err := p.Validate(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	output(w, http.StatusOK, p)
}
