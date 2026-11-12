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
	"yanetviz/internal/portpoll"
	"yanetviz/internal/wifi"
)

type server struct {
	portEnabled map[string]bool
	portPending []plan.PortSnapshot
	wifiStarted time.Time
	wifiEnabled map[string]bool
	wifiPending []plan.WiFiSnapshot
	store       *plan.Store
	password    [32]byte
	auth        bool
	publicView  bool
	secure      bool
	mu          sync.Mutex
	sessions    map[string]time.Time
	writes      sync.Mutex
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
	wifiPath := flag.String("wifi-config", "", "Optional WiFi CAPsMAN collection configuration JSON")
	portPath := flag.String("monitor-config", "", "Optional Ethernet link and PoE monitoring configuration JSON")
	publicView := flag.Bool("public-view", false, "Allow viewing plans and observations without login; editing still requires login")
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
	var wifiConfig wifi.Config
	if *wifiPath != "" {
		document, err := store.Load()
		if err != nil {
			return err
		}
		wifiConfig, err = wifi.LoadConfig(*wifiPath, document)
		if err != nil {
			return err
		}
	}
	var portConfig portpoll.Config
	if *portPath != "" {
		document, err := store.Load()
		if err != nil {
			return err
		}
		portConfig, err = portpoll.LoadConfig(*portPath, document)
		if err != nil {
			return err
		}
	}
	s := &server{store: store, auth: password != "", password: sha256.Sum256([]byte(password)), sessions: map[string]time.Time{}, secure: *secure, publicView: *publicView}
	s.wifiStarted = time.Now()
	s.wifiEnabled = map[string]bool{}
	for _, source := range wifiConfig.Sources {
		s.wifiEnabled[source.ID] = true
		s.wifiPending = append(s.wifiPending, plan.WiFiSnapshot{Source: source.ID, Device: source.Device, Domain: source.Domain})
	}
	s.portEnabled = map[string]bool{}
	for _, source := range portConfig.Sources {
		s.portEnabled[source.ID] = true
		for _, channel := range []string{"link", "poe"} {
			capability := ""
			if channel == "poe" && source.PoE != "auto" {
				capability = source.PoE
			}
			s.portPending = append(s.portPending, plan.PortSnapshot{Source: source.ID, Device: source.Device, Channel: channel, Capability: capability})
		}
	}
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
	mux.HandleFunc("/api/cable-labels/preview", s.protect(s.previewCableLabels))
	mux.HandleFunc("/api/export", s.protect(s.export))
	mux.HandleFunc("/api/observations/dhcp", s.protect(s.dhcpObservations))
	mux.HandleFunc("/api/observations/live", s.protect(s.liveObservations))
	mux.HandleFunc("/api/observations/events", s.protect(s.liveEvents))
	mux.HandleFunc("/api/observations/ports", s.protect(s.portObservations))
	mux.HandleFunc("/api/observations/ports/events", s.protect(s.portEvents))
	mux.HandleFunc("/api/observations/wifi", s.protect(s.wifiObservations))
	mux.HandleFunc("/api/observations/wifi/events", s.protect(s.wifiEvents))
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
	srv.BaseContext = func(_ net.Listener) context.Context { return ctx }
	collectionCtx, cancelCollection := context.WithCancel(ctx)
	collectionDone := make(chan struct{})
	go func() {
		defer close(collectionDone)
		var collectors sync.WaitGroup
		collectors.Add(2)
		go func() {
			defer collectors.Done()
			wifi.Run(collectionCtx, wifiConfig, store)
		}()
		go func() {
			defer collectors.Done()
			portpoll.Run(collectionCtx, portConfig, store)
		}()
		collectors.Wait()
	}()
	defer func() {
		cancelCollection()
		<-collectionDone
	}()
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
