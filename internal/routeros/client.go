// Package routeros provides bounded, read-only management requests. Monitor POSTs
// are explicitly allowlisted one-shot reads, never arbitrary RouterOS commands.
package routeros

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Row preserves vendor string values until a collector normalizes their units.
type Row map[string]string

// RequestError describes failure without leaking credentials or response bodies.
type RequestError struct {
	Status  int
	Message string
}

func (e *RequestError) Error() string { return e.Message }

// Client verifies TLS and refuses redirects. Clients targeting the same endpoint
// share a request gate, including clients belonging to different collectors.
type Client struct {
	endpoint string
	username string
	password string
	http     *http.Client
	gate     chan struct{}
}

var gates sync.Map

// NewClient accepts an endpoint already validated by startup configuration.
func NewClient(endpoint, username, password string) *Client {
	gate := make(chan struct{}, 1)
	shared, _ := gates.LoadOrStore(strings.ToLower(endpoint), gate)
	return &Client{endpoint: endpoint, username: username, password: password, gate: shared.(chan struct{}), http: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("redirects are not allowed") }}}
}

// Read requests only the properties required by the calling collector.
func (c *Client) Read(ctx context.Context, menu, properties string) ([]Row, error) {
	return c.request(ctx, http.MethodGet, menu+"?.proplist="+url.QueryEscape(properties), nil)
}

// Monitor reads one Ethernet or PoE sample for the supplied inventory IDs. No
// generic command execution, configuration mutation or power cycling is exposed.
func (c *Client) Monitor(ctx context.Context, menu string, numbers []string) ([]Row, error) {
	if menu != "interface/ethernet" && menu != "interface/ethernet/poe" {
		return nil, fmt.Errorf("monitor command is not allowed")
	}
	if len(numbers) == 0 {
		return []Row{}, nil
	}
	payload, err := json.Marshal(map[string]string{"numbers": strings.Join(numbers, ","), "once": ""})
	if err != nil {
		return nil, err
	}
	return c.request(ctx, http.MethodPost, menu+"/monitor", payload)
}

func (c *Client) request(ctx context.Context, method, path string, body []byte) ([]Row, error) {
	raw, err := c.rawRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	var rows []Row
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil || len(rows) > 10000 {
		return nil, fmt.Errorf("invalid or incomplete management response")
	}
	return rows, nil
}

// ReadResource reads allowlisted hardware/software properties only. RouterOS
// versions return singleton resources either as objects or one-element arrays.
func (c *Client) ReadResource(ctx context.Context, menu string) (Row, error) {
	properties := ""
	switch menu {
	case "system/resource":
		properties = "board-name,architecture-name,version"
	case "system/routerboard":
		properties = "model"
	default:
		return nil, fmt.Errorf("resource menu is not allowed")
	}
	raw, err := c.rawRequest(ctx, http.MethodGet, menu+"?.proplist="+url.QueryEscape(properties), nil)
	if err != nil {
		return nil, err
	}
	var object Row
	if err := json.Unmarshal(raw, &object); err == nil && object != nil {
		return object, nil
	}
	var rows []Row
	if err := json.Unmarshal(raw, &rows); err == nil && len(rows) == 1 && rows[0] != nil {
		return rows[0], nil
	}
	return nil, fmt.Errorf("invalid device resource response")
}

func (c *Client) rawRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-c.gate }()
	request, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("invalid management endpoint")
	}
	request.SetBasicAuth(c.username, c.password)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connection failed or timed out; check management access and TLS")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message := fmt.Sprintf("management request failed (HTTP %d)", response.StatusCode)
		switch response.StatusCode {
		case 401:
			message = "authentication failed; check source credentials"
		case 403:
			message = "permission denied; allow read and monitor access"
		case 400, 404:
			message = "requested management menu or monitor command is unavailable"
		}
		return nil, &RequestError{Status: response.StatusCode, Message: message}
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		return nil, fmt.Errorf("incomplete or oversized management response")
	}
	return raw, nil
}
