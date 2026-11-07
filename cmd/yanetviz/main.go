package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"yanetviz"
	"yanetviz/internal/plan"
)

type server struct {
	store    *plan.Store
	password [32]byte
	auth     bool
	secure   bool
	mu       sync.Mutex
	sessions map[string]time.Time
	writes   sync.Mutex
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	addr := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	data := flag.String("data", "data", "Persistent data directory")
	secure := flag.Bool("secure-cookie", false, "Require HTTPS for login cookies (enable behind an HTTPS proxy)")
	flag.Parse()
	password := os.Getenv("YANETVIZ_PASSWORD")
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if password == "" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("set YANETVIZ_PASSWORD before listening outside loopback")
	}
	if err = os.MkdirAll(*data, 0700); err != nil {
		return err
	}
	store, err := plan.Open(filepath.Join(*data, "yanetviz.db"), plan.Seed())
	if err != nil {
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()
	s := &server{store: store, auth: password != "", password: sha256.Sum256([]byte(password)), sessions: map[string]time.Time{}, secure: *secure}
	web, err := fs.Sub(yanetviz.Assets, "web/dist")
	if err != nil {
		return fmt.Errorf("open bundled frontend: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(web)))
	mux.HandleFunc("/floorplans/", s.protect(http.FileServer(http.FS(yanetviz.Assets)).ServeHTTP))
	mux.HandleFunc("/api/session", s.session)
	mux.HandleFunc("/api/plan", s.protect(s.plan))
	mux.HandleFunc("/api/validate", s.protect(s.validate))
	mux.HandleFunc("/api/export", s.protect(s.export))
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if _, err := store.Load(); err != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.Write([]byte("ok"))
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if r.Header.Get("X-Yanetviz") != "1" {
				http.Error(w, "missing request header", 403)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
				http.Error(w, "cross-origin edits are forbidden", 403)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
	srv := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			log.Printf("HTTP shutdown: %v", err)
		}
	}()
	log.Printf("Yanetviz: http://%s · data: %s", *addr, *data)
	if err = srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
func output(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}
