package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"yanetviz/internal/plan"
)

func testServer(t *testing.T) *server {
	t.Helper()
	store, err := plan.Open(filepath.Join(t.TempDir(), "test.db"), plan.Seed())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return &server{store: store, sessions: map[string]time.Time{}}
}

func TestValidationDoesNotPersistImport(t *testing.T) {
	s := testServer(t)
	before, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	candidate.Devices[0].Name = "Preview only"
	body, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	s.validate(response, httptest.NewRequest(http.MethodPost, "/api/validate", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("validation returned %d: %s", response.Code, response.Body.String())
	}
	after, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("import preview changed saved documentation")
	}
}

func TestExportContainsRestorablePlanAndBackgrounds(t *testing.T) {
	document := plan.Seed()
	document.Interfaces = []plan.NetworkInterface{{ID: plan.NewID(), Device: document.Devices[4].ID, Name: "Wi-Fi", Domain: "home", MACAddresses: []string{"02:11:22:33:44:55"}}}
	physical := plan.Port{ID: plan.NewID(), Device: document.Devices[4].ID, Name: "ether1", PoEMode: "unknown"}
	document.Ports = append(document.Ports, physical)
	document.Interfaces[0].Port = physical.ID
	cabinetHeight, equipmentHeight, startingUnit := 12, 2, 3
	document.Cabinets[0].HeightU = &cabinetHeight
	document.Cabinets[0].ShortName = "R01"
	document.Devices[0].ShortName = "rt01"
	labelNumber := 25
	document.Ports[0].LabelNumber = &labelNumber
	document.Devices[0].HeightU = &equipmentHeight
	document.Devices[0].RackUnit = &startingUnit
	var buffer bytes.Buffer
	if err := writeExport(&buffer, document); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		files[file.Name] = content
	}
	var restored plan.Plan
	if err := json.Unmarshal(files["plan.json"], &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, document) {
		t.Fatal("export changed the plan")
	}
	for _, floor := range document.Floors {
		if len(files[floor.Background[1:]]) == 0 {
			t.Fatalf("background %s missing", floor.Background)
		}
	}
	if len(files) != len(document.Floors)+2 {
		t.Fatal("unexpected files in documentation export")
	}
}

func TestPasswordLoginProtectsPlanAndCreatesPrivateSession(t *testing.T) {
	s := testServer(t)
	s.auth = true
	s.password = sha256.Sum256([]byte("test administrator password"))
	request := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	denied := httptest.NewRecorder()
	s.protect(s.plan)(denied, request)
	if denied.Code != http.StatusUnauthorized {
		t.Fatal("plan accessible without session")
	}
	bad := httptest.NewRecorder()
	s.session(bad, httptest.NewRequest(http.MethodPost, "/api/session", bytes.NewBufferString(`{"password":"wrong"}`)))
	if bad.Code != http.StatusUnauthorized {
		t.Fatal("incorrect password accepted")
	}
	login := httptest.NewRecorder()
	s.session(login, httptest.NewRequest(http.MethodPost, "/api/session", bytes.NewBufferString(`{"password":"test administrator password"}`)))
	if login.Code != http.StatusOK {
		t.Fatal("valid password rejected")
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie flags missing")
	}
	request.AddCookie(cookies[0])
	allowed := httptest.NewRecorder()
	s.protect(s.plan)(allowed, request)
	if allowed.Code != http.StatusOK {
		t.Fatal("valid session rejected")
	}
	logout := httptest.NewRecorder()
	request.Method = http.MethodDelete
	s.session(logout, request)
	if s.authorized(request) {
		t.Fatal("session survived logout")
	}
}

func TestLabelPreviewUsesWorkingCopyWithoutSaving(t *testing.T) {
	s := testServer(t)
	before, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	candidate.Cabinets[0].ShortName = "WorkingRack"
	body, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	s.previewCableLabels(response, httptest.NewRequest(http.MethodPost, "/api/cable-labels/preview", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", response.Code, response.Body.String())
	}
	var previews []plan.CableLabelPreview
	if err := json.Unmarshal(response.Body.Bytes(), &previews); err != nil {
		t.Fatal(err)
	}
	if len(previews) != len(candidate.Cables) {
		t.Fatal("missing per-cable label results")
	}
	after, err := s.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("label preview saved working copy changes")
	}
}
