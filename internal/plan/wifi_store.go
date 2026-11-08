package plan

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// WiFiSnapshots reads retained source snapshots independently of the manual plan.
func (s *Store) WiFiSnapshots() ([]WiFiSnapshot, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	snapshots, err := readRows[WiFiSnapshot](tx, "wifi_sources")
	if err != nil {
		return nil, err
	}
	return snapshots, tx.Commit()
}

// RecordWiFiSnapshot atomically replaces a complete successful snapshot, retaining
// absent clients' last evidence. A failed attempt updates health only. Neither
// path changes the manual plan or revision; out-of-order attempts are ignored.
func (s *Store) RecordWiFiSnapshot(next WiFiSnapshot) error {
	if err := next.validate(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	previous := WiFiSnapshot{}
	var raw string
	err = tx.QueryRow("SELECT payload FROM wifi_sources WHERE source=?", next.Source).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		if err := json.Unmarshal([]byte(raw), &previous); err != nil {
			return err
		}
		oldTime, _ := time.Parse(time.RFC3339Nano, previous.LastAttempt)
		newTime, _ := time.Parse(time.RFC3339Nano, next.LastAttempt)
		if !newTime.After(oldTime) {
			return nil
		}
	}
	if next.Error != "" {
		// Failed reads cannot reattribute retained evidence after config changes.
		if previous.Source == "" {
			previous.Source = next.Source
			previous.Device = next.Device
			previous.Domain = next.Domain
		}
		previous.LastAttempt = next.LastAttempt
		previous.Error = next.Error
		next = previous
	} else {
		next.LastSuccess = next.LastAttempt
		keys := map[string]bool{}
		for i := range next.Registrations {
			next.Registrations[i].Present = true
			next.Registrations[i].LastSeen = next.LastSuccess
			record := next.Registrations[i]
			keys[record.MAC+"/"+record.InterfaceName] = true
		}
		// Domain changes must not reattribute historical observations to a new LAN.
		if previous.Domain == next.Domain && previous.Device == next.Device {
			for _, record := range previous.Registrations {
				if !keys[record.MAC+"/"+record.InterfaceName] {
					record.Present = false
					next.Registrations = append(next.Registrations, record)
				}
			}
		}
	}
	if len(next.Registrations) > 10000 {
		return fmt.Errorf("retained Wi-Fi observations exceed 10000 records")
	}
	encoded, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if len(encoded) > 8<<20 {
		return fmt.Errorf("Wi-Fi snapshot exceeds 8 MiB")
	}
	if _, err := tx.Exec("INSERT INTO wifi_sources VALUES(?,?) ON CONFLICT(source) DO UPDATE SET payload=excluded.payload", next.Source, string(encoded)); err != nil {
		return err
	}
	return tx.Commit()
}
