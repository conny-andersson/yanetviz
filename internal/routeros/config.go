package routeros

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Endpoint validates one configured host, defaulting to verified HTTPS. HTTP
// requires explicit rest-http; user-info, paths and query strings are rejected.
func Endpoint(address, transport string) (string, error) {
	scheme := "https"
	if transport == "rest-http" {
		scheme = "http"
	} else if transport != "" && transport != "rest-https" {
		return "", fmt.Errorf("transport must be rest-https or rest-http")
	}
	target, err := url.Parse(scheme + "://" + address)
	if err != nil || target.Hostname() == "" || target.User != nil || target.Path != "" || target.RawQuery != "" || target.Fragment != "" || strings.Contains(address, "/") {
		return "", fmt.Errorf("use a management host or host:port")
	}
	return target.String() + "/rest/", nil
}

// Password reads a separate credential file relative to its startup config.
func Password(path, configPath string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("passwordFile is required")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(configPath), path)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot read password file")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(raw) > 4096 {
		return "", fmt.Errorf("invalid password file")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	if password == "" {
		return "", fmt.Errorf("invalid password file")
	}
	return password, nil
}
