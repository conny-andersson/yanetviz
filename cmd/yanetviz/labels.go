package main

import (
	"encoding/json"
	"io"
	"net/http"
	"yanetviz/internal/plan"
)

// previewCableLabels accepts the working copy so unsaved names and rack positions
// can be reviewed. It is read-only and does not change the saved plan revision.
func (s *server) previewCableLabels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var document plan.Plan
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		http.Error(w, "invalid plan JSON or request too large", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "expected one JSON document", http.StatusBadRequest)
		return
	}
	document = document.NormalizeIdentities()
	if err := document.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	output(w, http.StatusOK, document.PreviewCableLabels())
}
