package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

func (s *server) authorized(r *http.Request) bool {
	if !s.auth {
		return true
	}
	c, err := r.Cookie("yanetviz_session")
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.sessions[c.Value]
	return ok && time.Now().Before(until)
}

// permissions describes application capabilities independently of login identity.
// Future user roles can resolve to the same view/edit capabilities.
type permissions struct {
	View bool `json:"view"`
	Edit bool `json:"edit"`
}

func (s *server) permissions(r *http.Request) permissions {
	edit := s.authorized(r)
	return permissions{View: s.publicView || edit, Edit: edit}
}

func (s *server) protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		access := s.permissions(r)
		allowed := access.Edit
		// All protected GET/HEAD routes read saved documentation or observations.
		// Other methods, including import validation and label previews, require edit.
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			allowed = access.View
		}
		if !allowed {
			http.Error(w, "sign in required", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
func (s *server) session(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		output(w, http.StatusOK, struct {
			Authenticated bool        `json:"authenticated"`
			LoginRequired bool        `json:"loginRequired"`
			Permissions   permissions `json:"permissions"`
		}{s.authorized(r), s.auth, s.permissions(r)})
	case "POST":
		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		hash := sha256.Sum256([]byte(body.Password))
		if s.auth && subtle.ConstantTimeCompare(hash[:], s.password[:]) != 1 {
			time.Sleep(250 * time.Millisecond)
			http.Error(w, "incorrect password", 401)
			return
		}
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			http.Error(w, "session unavailable", 500)
			return
		}
		value := hex.EncodeToString(token)
		s.mu.Lock()
		for k, t := range s.sessions {
			if time.Now().After(t) {
				delete(s.sessions, k)
			}
		}
		s.sessions[value] = time.Now().Add(12 * time.Hour)
		s.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "yanetviz_session", Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.secure, MaxAge: 43200})
		output(w, 200, map[string]bool{"authenticated": true})
	case "DELETE":
		if c, err := r.Cookie("yanetviz_session"); err == nil {
			s.mu.Lock()
			delete(s.sessions, c.Value)
			s.mu.Unlock()
		}
		http.SetCookie(w, &http.Cookie{Name: "yanetviz_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteStrictMode})
		w.WriteHeader(204)
	default:
		w.WriteHeader(405)
	}
}
