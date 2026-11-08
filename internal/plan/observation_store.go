package plan

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DHCPObservations reads collected evidence separately from the exported plan.
func (s *Store) DHCPObservations() ([]DHCPObservation, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	records, err := readRows[DHCPObservation](tx, "dhcp_observations")
	if err != nil {
		return nil, err
	}
	return records, tx.Commit()
}

// RecordDHCPObservations commits a validated batch without modifying names,
// placement or plan revision. Each distinct source/server/MAC/client-ID tuple
// retains its own first/last-seen interval. Older samples cannot replace newer IP
// or hostname evidence, and an empty batch never declares clients absent.
func (s *Store) RecordDHCPObservations(records []DHCPObservation) error {
	if len(records) > 10000 {
		return fmt.Errorf("%w: batch exceeds 10000 observations", ErrInvalidDHCPObservation)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	document, err := loadPlan(tx)
	if err != nil {
		return err
	}
	normalized := make([]DHCPObservation, len(records))
	for index, record := range records {
		record.Domain = strings.ToLower(strings.TrimSpace(record.Domain))
		record.MACAddress, err = NormalizeMAC(record.MACAddress)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidDHCPObservation, err)
		}
		if record.FirstSeen == "" {
			record.FirstSeen = record.LastSeen
		}
		if err := record.validate(document); err != nil {
			return fmt.Errorf("%w: record %d: %v", ErrInvalidDHCPObservation, index+1, err)
		}
		first, _ := time.Parse(time.RFC3339Nano, record.FirstSeen)
		last, _ := time.Parse(time.RFC3339Nano, record.LastSeen)
		record.FirstSeen = first.UTC().Format(time.RFC3339Nano)
		record.LastSeen = last.UTC().Format(time.RFC3339Nano)
		normalized[index] = record
	}
	for _, record := range normalized {
		key := []any{record.Interface, record.Source, record.Server, record.MACAddress, record.ClientID}
		var raw string
		err := tx.QueryRow("SELECT payload FROM dhcp_observations WHERE interface_id=? AND source_id=? AND server=? AND mac=? AND client_id=?", key...).Scan(&raw)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil {
			var previous DHCPObservation
			if err := json.Unmarshal([]byte(raw), &previous); err != nil {
				return err
			}
			previousFirst, err := time.Parse(time.RFC3339Nano, previous.FirstSeen)
			if err != nil {
				return err
			}
			previousLast, err := time.Parse(time.RFC3339Nano, previous.LastSeen)
			if err != nil {
				return err
			}
			first, _ := time.Parse(time.RFC3339Nano, record.FirstSeen)
			last, _ := time.Parse(time.RFC3339Nano, record.LastSeen)
			earliest := record.FirstSeen
			if previousFirst.Before(first) {
				earliest = previous.FirstSeen
			}
			if previousLast.After(last) {
				record = previous
			}
			record.FirstSeen = earliest
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO dhcp_observations VALUES(?,?,?,?,?,?)
   ON CONFLICT(interface_id,source_id,server,mac,client_id) DO UPDATE SET payload=excluded.payload`, append(key, string(encoded))...)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Full-plan replacement must retain evidence for surviving identities, and
// remove evidence only when its owner, source, domain or MAC alias was removed.
func pruneOrphanedObservations(tx *sql.Tx, p Plan) error {
	records, err := readRows[DHCPObservation](tx, "dhcp_observations")
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.validate(p) == nil {
			continue
		}
		_, err = tx.Exec("DELETE FROM dhcp_observations WHERE interface_id=? AND source_id=? AND server=? AND mac=? AND client_id=?", record.Interface, record.Source, record.Server, record.MACAddress, record.ClientID)
		if err != nil {
			return err
		}
	}
	return nil
}
