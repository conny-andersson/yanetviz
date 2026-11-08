package plan

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// PortSnapshots reads retained evidence independently of the manual plan.
func (s *Store) PortSnapshots() ([]PortSnapshot, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	snapshots, err := readRows[PortSnapshot](tx, "port_sources")
	if err != nil {
		return nil, err
	}
	return snapshots, tx.Commit()
}

// RecordPortSnapshot commits one complete channel. Failed/unavailable samples
// retain prior readings. Missing inventory means unknown status, never link-down.
// Late results cannot overwrite newer evidence. Plan revisions are unaffected.
func (s *Store) RecordPortSnapshot(next PortSnapshot) error {
	if err := next.validate(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw string
	previous := PortSnapshot{}
	err = tx.QueryRow("SELECT payload FROM port_sources WHERE source=? AND channel=?", next.Source, next.Channel).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		if err := json.Unmarshal([]byte(raw), &previous); err != nil {
			return err
		}
		before, _ := time.Parse(time.RFC3339Nano, previous.LastAttempt)
		after, _ := time.Parse(time.RFC3339Nano, next.LastAttempt)
		if !after.After(before) {
			return nil
		}
	}
	// Inventory refresh health is independent from link/PoE channel health.
	if previous.Device == next.Device || previous.Device == "" {
		next.Inventory = mergeInventory(previous.Inventory, next.Inventory)
	}
	if next.Error != "" || oneOf(next.Capability, "unavailable", "unsupported", "disabled") {
		if previous.Source == "" {
			previous.Source = next.Source
			previous.Device = next.Device
			previous.Channel = next.Channel
		}
		if previous.Device == next.Device {
			previous.Inventory = next.Inventory
		}
		previous.LastAttempt = next.LastAttempt
		previous.Error = next.Error
		if next.Capability != "" {
			previous.Capability = next.Capability
		}
		next = previous
	} else {
		next.LastSuccess = next.LastAttempt
		keys := map[string]bool{}
		for i := range next.Ports {
			next.Ports[i].Present = true
			next.Ports[i].LastSeen = next.LastSuccess
			keys[next.Ports[i].nativeKey()] = true
		}
		if previous.Device == next.Device {
			for _, reading := range previous.Ports {
				if !keys[reading.nativeKey()] {
					reading.Present = false
					next.Ports = append(next.Ports, reading)
				}
			}
		}
	}
	if len(next.Ports) > 2000 {
		return fmt.Errorf("retained port snapshot exceeds 2000 interfaces")
	}
	encoded, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if len(encoded) > 8<<20 {
		return fmt.Errorf("port snapshot exceeds 8 MiB")
	}
	if _, err := tx.Exec("INSERT INTO port_sources VALUES(?,?,?) ON CONFLICT(source,channel) DO UPDATE SET payload=excluded.payload", next.Source, next.Channel, string(encoded)); err != nil {
		return err
	}
	return tx.Commit()
}

func mergeInventory(previous, next *DeviceInventory) *DeviceInventory {
	if next == nil {
		return previous
	}
	if previous == nil {
		return next
	}
	oldAttempt, _ := time.Parse(time.RFC3339Nano, previous.LastAttempt)
	newAttempt, _ := time.Parse(time.RFC3339Nano, next.LastAttempt)
	if newAttempt.Before(oldAttempt) {
		return previous
	}
	if next.LastSuccess != "" {
		retained := *next
		if retained.Model == "" && retained.Error != "" {
			retained.Model = previous.Model
		}
		return &retained
	}
	retained := *previous
	retained.LastAttempt = next.LastAttempt
	retained.Error = next.Error
	return &retained
}
