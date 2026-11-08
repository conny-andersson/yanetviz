package plan

import "fmt"

// Rack dimensions are optional to keep legacy plans and partially documented
// equipment usable. A mounting position requires a known device height, but a
// cabinet can still have an unknown total height while its contents are mapped.
func (p Plan) validateRackMounting() error {
	cabinets := make(map[string]Cabinet, len(p.Cabinets))
	for _, cabinet := range p.Cabinets {
		if !validRackValue(cabinet.HeightU) {
			return fmt.Errorf("cabinet %q: height must be a whole number from 1 to 1000 U, or unknown", cabinet.Name)
		}
		cabinets[cabinet.ID] = cabinet
	}

	occupied := make(map[string]map[int]string)
	for _, device := range p.Devices {
		if !validRackValue(device.HeightU) || !validRackValue(device.RackUnit) {
			return fmt.Errorf("equipment %q: height and starting unit must be whole numbers from 1 to 1000, or unassigned", device.Name)
		}
		if device.RackUnit == nil {
			continue
		}
		if device.Cabinet == "" {
			return fmt.Errorf("equipment %q: assign a cabinet before setting its rack unit", device.Name)
		}
		if device.HeightU == nil {
			return fmt.Errorf("equipment %q: document its height before assigning a rack unit", device.Name)
		}

		cabinet := cabinets[device.Cabinet]
		lastUnit := *device.RackUnit + *device.HeightU - 1
		if cabinet.HeightU != nil && lastUnit > *cabinet.HeightU {
			return fmt.Errorf("equipment %q occupies U%d–U%d, beyond cabinet %q (%d U)", device.Name, *device.RackUnit, lastUnit, cabinet.Name, *cabinet.HeightU)
		}
		if occupied[cabinet.ID] == nil {
			occupied[cabinet.ID] = make(map[int]string)
		}
		for unit := *device.RackUnit; unit <= lastUnit; unit++ {
			if other, used := occupied[cabinet.ID][unit]; used {
				return fmt.Errorf("cabinet %q: %q and %q overlap at U%d", cabinet.Name, other, device.Name, unit)
			}
			occupied[cabinet.ID][unit] = device.Name
		}
	}
	return nil
}

func validRackValue(value *int) bool {
	return value == nil || (*value >= 1 && *value <= 1000)
}
