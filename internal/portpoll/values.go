package portpoll

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"yanetviz/internal/plan"
	"yanetviz/internal/routeros"
)

func parseBool(raw string) (*bool, error) {
	if raw == "" {
		return nil, nil
	}
	value := false
	switch raw {
	case "true", "yes":
		value = true
	case "false", "no":
	default:
		return nil, fmt.Errorf("invalid interface boolean")
	}
	return &value, nil
}

func measurement(raw, unit string) (*float64, error) {
	if raw == "" {
		return nil, nil
	}
	number := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), unit))
	value, err := strconv.ParseFloat(number, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return nil, fmt.Errorf("invalid port measurement")
	}
	return &value, nil
}

func parseRate(raw string) (*float64, error) {
	if raw == "" {
		return nil, nil
	}
	for _, unit := range []struct {
		suffix string
		factor float64
	}{{"Tbps", 1e6}, {"Gbps", 1000}, {"Mbps", 1}, {"Kbps", .001}, {"kbps", .001}, {"bps", .000001}} {
		if strings.HasSuffix(raw, unit.suffix) {
			value, err := measurement(strings.TrimSuffix(raw, unit.suffix), "")
			if err != nil || value == nil {
				return nil, fmt.Errorf("invalid negotiated link rate")
			}
			result := *value * unit.factor
			if math.IsInf(result, 0) {
				return nil, fmt.Errorf("invalid negotiated link rate")
			}
			return &result, nil
		}
	}
	return nil, fmt.Errorf("unknown negotiated link-rate unit")
}

func parsePoE(configured, observed routeros.Row) (*plan.ObservedPoE, error) {
	vendor := observed["poe-out"]
	if vendor == "" {
		vendor = configured["poe-out"]
	}
	status := &plan.ObservedPoE{VendorMode: vendor, Mode: "unknown", VoltageSelection: configured["poe-voltage"], Status: observed["poe-out-status"]}
	switch {
	case vendor == "auto-on" || vendor == "auto":
		status.Mode = "auto"
	case strings.HasPrefix(vendor, "forced-on"):
		status.Mode = "forced"
	case vendor == "off":
		status.Mode = "off"
	}
	if status.Status == "" {
		status.Status = "unknown"
	}
	var err error
	status.VoltageV, err = measurement(observed["poe-out-voltage"], "V")
	if err != nil {
		return nil, err
	}
	status.CurrentMA, err = measurement(observed["poe-out-current"], "mA")
	if err != nil {
		return nil, err
	}
	status.PowerW, err = measurement(observed["poe-out-power"], "W")
	if err != nil {
		return nil, err
	}
	return status, nil
}
