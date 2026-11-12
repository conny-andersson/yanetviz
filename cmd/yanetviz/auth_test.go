package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestViewAndEditPermissions(t *testing.T) {
	for _, scenario := range []struct {
		name                           string
		auth, publicView, validSession bool
		want                           permissions
	}{
		{"local without login", false, false, false, permissions{true, true}},
		{"private anonymous", true, false, false, permissions{false, false}},
		{"public anonymous", true, true, false, permissions{true, false}},
		{"private administrator", true, false, true, permissions{true, true}},
		{"public administrator", true, true, true, permissions{true, true}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := testServer(t)
			s.auth, s.publicView = scenario.auth, scenario.publicView
			s.sessions["valid"] = time.Now().Add(time.Hour)
			s.sessions["expired"] = time.Now().Add(-time.Second)
			request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
			token := "expired"
			if scenario.validSession {
				token = "valid"
			}
			request.AddCookie(&http.Cookie{Name: "yanetviz_session", Value: token})
			response := httptest.NewRecorder()
			s.session(response, request)
			var session struct {
				Permissions permissions `json:"permissions"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
				t.Fatal(err)
			}
			if session.Permissions != scenario.want {
				t.Fatalf("permissions: %+v, want %+v", session.Permissions, scenario.want)
			}
			for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
				request.Method = method
				response := httptest.NewRecorder()
				called := false
				s.protect(func(w http.ResponseWriter, r *http.Request) {
					called = true
					w.WriteHeader(http.StatusNoContent)
				})(response, request)
				want := scenario.want.Edit
				if method == http.MethodGet || method == http.MethodHead {
					want = scenario.want.View
				}
				if called != want {
					t.Errorf("%s: handler called=%v, want %v", method, called, want)
				}
				if !want && response.Code != http.StatusUnauthorized {
					t.Errorf("%s: status %d", method, response.Code)
				}
			}
		})
	}
}

func TestAnonymousViewCannotChangePlan(t *testing.T) {
	s := testServer(t)
	s.auth, s.publicView = true, true
	before, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct {
		method, path string
		handler      http.HandlerFunc
	}{
		{http.MethodPut, "/api/plan", s.plan},
		{http.MethodPost, "/api/validate", s.validate},
		{http.MethodPost, "/api/cable-labels/preview", s.previewCableLabels},
	} {
		response := httptest.NewRecorder()
		s.protect(route.handler)(response, httptest.NewRequest(route.method, route.path, bytes.NewReader(beforeJSON)))
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s allowed anonymous edit operation: %d", route.path, response.Code)
		}
	}
	after, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeJSON, afterJSON) {
		t.Fatal("anonymous requests changed saved documentation")
	}
}
