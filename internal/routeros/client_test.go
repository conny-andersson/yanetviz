package routeros

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollectorsShareOneRequestGatePerEndpoint(t *testing.T) {
	var active atomic.Int32
	var overlap atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if active.Add(1) > 1 {
			overlap.Store(true)
		}
		defer active.Add(-1)
		time.Sleep(5 * time.Millisecond)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()
	clients := []*Client{NewClient(server.URL+"/rest/", "monitor", "test-password"), NewClient(server.URL+"/rest/", "monitor", "test-password")}
	var workers sync.WaitGroup
	for _, client := range clients {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := client.Read(context.Background(), "interface/ethernet", "name"); err != nil {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
	if overlap.Load() {
		t.Fatal("collectors overlapped requests on the same device")
	}
	if _, err := clients[0].Monitor(context.Background(), "interface/ethernet/poe/power-cycle", []string{"*1"}); err == nil {
		t.Fatal("transport allowed a power-changing command")
	}
}
