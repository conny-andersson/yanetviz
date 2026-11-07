package plan

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
)

// ErrConflict means the submitted revision no longer matches persisted state.
var ErrConflict = errors.New("the plan changed in another tab; reload before saving")

// Store persists manual documentation in one SQLite database.
type Store struct{ db *sql.DB }

// Open creates the schema and seeds an empty database. Existing data is retained.
func Open(path string, seed Plan) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(cause error) (*Store, error) {
		return nil, errors.Join(cause, db.Close())
	}
	var schemaVersion int
	if err := db.QueryRow("PRAGMA user_version").Scan(&schemaVersion); err != nil {
		return fail(err)
	}
	if schemaVersion > 1 {
		return fail(fmt.Errorf("database schema %d is newer than this application supports", schemaVersion))
	}
	_, err = db.Exec(`PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS metadata (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL, revision INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS floors (id TEXT PRIMARY KEY, payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS cabinets (id TEXT PRIMARY KEY, floor TEXT NOT NULL REFERENCES floors(id), payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS devices (id TEXT PRIMARY KEY, floor TEXT REFERENCES floors(id), cabinet TEXT REFERENCES cabinets(id), payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS ports (id TEXT PRIMARY KEY, device TEXT NOT NULL REFERENCES devices(id), payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS cables (id TEXT PRIMARY KEY, a_device TEXT NOT NULL REFERENCES devices(id), b_device TEXT NOT NULL REFERENCES devices(id), a_port TEXT REFERENCES ports(id), b_port TEXT REFERENCES ports(id), payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS pass_throughs (id TEXT PRIMARY KEY, a TEXT NOT NULL REFERENCES ports(id), b TEXT NOT NULL REFERENCES ports(id), payload TEXT NOT NULL);
 PRAGMA user_version=1;`)
	if err != nil {
		return fail(err)
	}
	s := &Store{db}
	var n int
	if err = db.QueryRow("SELECT count(*) FROM metadata").Scan(&n); err != nil {
		return fail(err)
	}
	if n == 0 {
		if err = seed.Validate(); err != nil {
			return fail(err)
		}
		if err = s.save(seed, true); err != nil {
			return fail(err)
		}
	}
	return s, nil
}

// Close releases the database connection.
func (s *Store) Close() error { return s.db.Close() }
func readRows[T any](tx *sql.Tx, table string) ([]T, error) {
	rows, err := tx.Query("SELECT payload FROM " + table + " ORDER BY rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v T
		if err = json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Load reads a consistent plan, including its current revision.
func (s *Store) Load() (Plan, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Plan{}, err
	}
	defer tx.Rollback()
	p := Plan{}
	if err = tx.QueryRow("SELECT version,revision FROM metadata WHERE id=1").Scan(&p.Version, &p.Revision); err != nil {
		return p, err
	}
	if p.Floors, err = readRows[Floor](tx, "floors"); err != nil {
		return p, err
	}
	if p.Cabinets, err = readRows[Cabinet](tx, "cabinets"); err != nil {
		return p, err
	}
	if p.Devices, err = readRows[Device](tx, "devices"); err != nil {
		return p, err
	}
	if p.Ports, err = readRows[Port](tx, "ports"); err != nil {
		return p, err
	}
	if p.Cables, err = readRows[Cable](tx, "cables"); err != nil {
		return p, err
	}
	if p.PassThroughs, err = readRows[PassThrough](tx, "pass_throughs"); err != nil {
		return p, err
	}
	return p, tx.Commit()
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Save validates and replaces documentation atomically. A matching revision is
// required, and a successful save advances the persisted revision by one.
func (s *Store) Save(p Plan) error {
	return s.save(p, false)
}

func (s *Store) save(p Plan, initial bool) error {
	if err := p.Validate(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if initial {
		_, err = tx.Exec("INSERT INTO metadata VALUES(1,1,1)")
	} else {
		var result sql.Result
		result, err = tx.Exec("UPDATE metadata SET revision=revision+1 WHERE id=1 AND revision=?", p.Revision)
		if err == nil {
			var n int64
			n, err = result.RowsAffected()
			if n == 0 && err == nil {
				return ErrConflict
			}
		}
	}
	if err != nil {
		return err
	}
	for _, t := range []string{"pass_throughs", "cables", "ports", "devices", "cabinets", "floors"} {
		if _, err = tx.Exec("DELETE FROM " + t); err != nil {
			return err
		}
	}
	// Descriptive fields remain JSON while foreign keys enforce entity ownership.
	insert := func(query string, entity any, columns ...any) error {
		encoded, err := json.Marshal(entity)
		if err != nil {
			return fmt.Errorf("encode plan entity: %w", err)
		}
		columns = append(columns, string(encoded))
		_, err = tx.Exec(query, columns...)
		return err
	}
	for _, v := range p.Floors {
		if err = insert("INSERT INTO floors VALUES(?,?)", v, v.ID); err != nil {
			return err
		}
	}
	for _, v := range p.Cabinets {
		if err = insert("INSERT INTO cabinets VALUES(?,?,?)", v, v.ID, v.Floor); err != nil {
			return err
		}
	}
	for _, v := range p.Devices {
		if err = insert("INSERT INTO devices VALUES(?,?,?,?)", v, v.ID, nullable(v.Floor), nullable(v.Cabinet)); err != nil {
			return err
		}
	}
	for _, v := range p.Ports {
		if err = insert("INSERT INTO ports VALUES(?,?,?)", v, v.ID, v.Device); err != nil {
			return err
		}
	}
	for _, v := range p.Cables {
		if err = insert("INSERT INTO cables VALUES(?,?,?,?,?,?)", v, v.ID, v.A.Device, v.B.Device, nullable(v.A.Port), nullable(v.B.Port)); err != nil {
			return err
		}
	}
	for _, v := range p.PassThroughs {
		if err = insert("INSERT INTO pass_throughs VALUES(?,?,?,?)", v, v.ID, v.A, v.B); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit plan: %w", err)
	}
	return nil
}
