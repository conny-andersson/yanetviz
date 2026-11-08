package plan

import (
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestDocumentationSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.db")
	store, err := Open(path, Seed())
	if err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	document.Devices[0].Name = "Basement router"
	document.Cabinets[0].HeightU = rackUnits(12)
	document.Cabinets[0].ShortName = "R01"
	document.Devices[0].ShortName = "rt01"
	document.Ports[0].LabelNumber = rackUnits(25)
	document.Devices[0].HeightU = rackUnits(2)
	document.Devices[0].RackUnit = rackUnits(3)
	document.Devices[4].Floor = "entrance"
	document.Devices[4].X, document.Devices[4].Y = .23, .71
	if err := store.Save(document); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path, Seed())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	actual, err := reopened.Load()
	if err != nil {
		t.Fatal(err)
	}
	document.Revision++
	if !reflect.DeepEqual(document, actual) {
		t.Fatalf("documentation changed after reopening\nwant: %#v\ngot: %#v", document, actual)
	}
}

func TestStaleEditDoesNotOverwriteSavedPlan(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "plan.db"), Seed())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	stale, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	first.Devices[0].Name = "First tab edit"
	if err := store.Save(first); err != nil {
		t.Fatal(err)
	}
	stale.Devices[0].Name = "Stale tab edit"
	if err := store.Save(stale); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	actual, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if actual.Devices[0].Name != "First tab edit" || actual.Revision != first.Revision+1 {
		t.Fatal("stale edit overwrote saved documentation")
	}
}

func TestConcurrentEditsHaveOneWinner(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "plan.db"), Seed())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var writers sync.WaitGroup
	for range 2 {
		writers.Add(1)
		go func() { defer writers.Done(); results <- store.Save(document) }()
	}
	writers.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("got %d successful saves and %d conflicts", successes, conflicts)
	}
}
