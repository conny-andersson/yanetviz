package plan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CableLabelPreview is a proposed label or an actionable reason why it cannot be
// generated. Generation never changes the plan or fills in missing endpoints.
type CableLabelPreview struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
	Error string `json:"error,omitempty"`
}

var numericPortName = regexp.MustCompile(`(?i)^(?:ether|port)?([0-9]+)$`)
var sfpPortName = regexp.MustCompile(`(?i)^(?:sfp(?:\+|plus)?|sfp-sfpplus)([0-9]+)$`)
var sfpConnector = regexp.MustCompile(`(?i)^sfp(?:\+|28|56)?$`)

// SFP cages have their own numbering namespace, so ether1 and sfp1 remain
// distinguishable as P01 and Psfp01. An override changes the number, not its type.
func labelPortToken(port Port) (string, error) {
	name := strings.TrimSpace(port.Name)
	sfpMatch := sfpPortName.FindStringSubmatch(name)
	prefix := "P"
	if sfpMatch != nil || sfpConnector.MatchString(strings.TrimSpace(port.Connector)) {
		prefix = "Psfp"
	}
	number := 0
	if port.LabelNumber != nil {
		number = *port.LabelNumber
	} else {
		match := numericPortName.FindStringSubmatch(name)
		if sfpMatch != nil {
			match = sfpMatch
		}
		if port.Side != "" {
			match = numericPortName.FindStringSubmatch(strings.TrimSpace(port.Position))
		}
		if match != nil {
			parsed, err := strconv.Atoi(match[1])
			if err == nil {
				number = parsed
			}
		}
	}
	if number < 1 || number > 9999 {
		return "", fmt.Errorf("port %q needs a cable label number (1–9999)", port.Name)
	}
	suffix := ""
	if port.Side == "front" {
		suffix = "F"
	}
	if port.Side == "rear" {
		suffix = "R"
	}
	return fmt.Sprintf("%s%02d%s", prefix, number, suffix), nil
}

func labelName(short, full string) string {
	if value := strings.TrimSpace(short); value != "" {
		return value
	}
	return strings.TrimSpace(full)
}

// PreviewCableLabels formats user-authored endpoint information in A-to-B order.
// Call Validate first. Missing or ambiguous information prevents generation for
// that cable while allowing independent, fully documented cables to be labeled.
func (p Plan) PreviewCableLabels() []CableLabelPreview {
	devices := make(map[string]Device, len(p.Devices))
	cabinets := make(map[string]Cabinet, len(p.Cabinets))
	ports := make(map[string]Port, len(p.Ports))
	tokens := make(map[string]string, len(p.Ports))
	problems := make(map[string]string)
	portTokenCounts := make(map[string]int)
	for _, device := range p.Devices {
		devices[device.ID] = device
	}
	for _, cabinet := range p.Cabinets {
		cabinets[cabinet.ID] = cabinet
	}
	for _, port := range p.Ports {
		ports[port.ID] = port
		token, err := labelPortToken(port)
		if err != nil {
			problems[port.ID] = err.Error()
			continue
		}
		tokens[port.ID] = token
		portTokenCounts[port.Device+"/"+token]++
	}
	endpointLabels := make(map[string]string, len(p.Ports))
	endpointCounts := make(map[string]int)
	for _, port := range p.Ports {
		if problems[port.ID] != "" {
			continue
		}
		device := devices[port.Device]
		token := tokens[port.ID]
		if portTokenCounts[port.Device+"/"+token] > 1 {
			problems[port.ID] = fmt.Sprintf("%s: %s is ambiguous; assign unique cable label numbers to its ports", device.Name, token)
			continue
		}
		prefix := labelName(device.ShortName, device.Name)
		if device.Cabinet != "" {
			cabinet := cabinets[device.Cabinet]
			if device.RackUnit == nil {
				problems[port.ID] = fmt.Sprintf("%s: starting rack unit in %s is unknown", device.Name, cabinet.Name)
				continue
			}
			prefix = fmt.Sprintf("%s:U%02d", labelName(cabinet.ShortName, cabinet.Name), *device.RackUnit)
		}
		label := prefix + ":" + token
		endpointLabels[port.ID] = label
		endpointCounts[label]++
	}
	formatEndpoint := func(endpoint Endpoint) (string, error) {
		device, known := devices[endpoint.Device]
		if !known {
			return "", fmt.Errorf("endpoint device is unknown")
		}
		if endpoint.Port == "" {
			return "", fmt.Errorf("%s: endpoint port is unknown", device.Name)
		}
		port, known := ports[endpoint.Port]
		if !known || port.Device != endpoint.Device {
			return "", fmt.Errorf("%s: endpoint port is unresolved", device.Name)
		}
		if problem := problems[port.ID]; problem != "" {
			return "", fmt.Errorf("%s", problem)
		}
		label := endpointLabels[port.ID]
		if endpointCounts[label] > 1 {
			return "", fmt.Errorf("endpoint label %q is shared; use distinct cabinet/equipment short names or port numbers", label)
		}
		return label, nil
	}
	existingLabels := make(map[string][]string)
	for _, cable := range p.Cables {
		if label := strings.TrimSpace(cable.Label); label != "" {
			existingLabels[label] = append(existingLabels[label], cable.ID)
		}
	}
	previews := make([]CableLabelPreview, 0, len(p.Cables))
	for _, cable := range p.Cables {
		preview := CableLabelPreview{ID: cable.ID}
		a, aerr := formatEndpoint(cable.A)
		b, berr := formatEndpoint(cable.B)
		switch {
		case aerr != nil:
			preview.Error = "Endpoint A: " + aerr.Error()
		case berr != nil:
			preview.Error = "Endpoint B: " + berr.Error()
		default:
			preview.Label = a + "-" + b
			for _, otherID := range existingLabels[preview.Label] {
				if otherID != cable.ID {
					preview.Error = "Proposed label is already used by another cable; resolve that label first"
					preview.Label = ""
					break
				}
			}
		}
		previews = append(previews, preview)
	}
	return previews
}
