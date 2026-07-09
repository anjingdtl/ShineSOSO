package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/local/easysearch/backend/internal/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sampleIndexer(name string) model.InstalledIndexer {
	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	return model.InstalledIndexer{
		ID:                uuid.NewString(),
		DefinitionID:      "example-public",
		Name:              name,
		Enabled:           true,
		BaseURL:           "https://example.com",
		DefinitionVersion: "1.0.0",
		Status:            "unknown",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func TestIndexerRepoCreateAndGet(t *testing.T) {
	s := openTestStore(t)
	repo := NewIndexerRepo(s)

	in := sampleIndexer("alpha")
	if err := repo.Create(in); err != nil {
		t.Fatalf("create: %v", err)
	}
	out, err := repo.Get(in.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if out.Name != "alpha" || out.BaseURL != "https://example.com" {
		t.Errorf("got %+v", out)
	}
	if out.Status != "unknown" {
		t.Errorf("status: got %q want unknown", out.Status)
	}
}

func TestIndexerRepoList(t *testing.T) {
	s := openTestStore(t)
	repo := NewIndexerRepo(s)

	a := sampleIndexer("alpha")
	b := sampleIndexer("beta")
	_ = repo.Create(a)
	_ = repo.Create(b)

	all, err := repo.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("len=%d want 2", len(all))
	}
}

func TestIndexerRepoUniqueName(t *testing.T) {
	s := openTestStore(t)
	repo := NewIndexerRepo(s)
	a := sampleIndexer("alpha")
	b := sampleIndexer("alpha") // same name
	if err := repo.Create(a); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err := repo.Create(b)
	if err == nil {
		t.Errorf("expected duplicate-name error, got nil")
	}
}

func TestIndexerRepoUpdate(t *testing.T) {
	s := openTestStore(t)
	repo := NewIndexerRepo(s)
	a := sampleIndexer("alpha")
	_ = repo.Create(a)

	a.Enabled = false
	a.BaseURL = "https://new.example.com"
	a.Status = "healthy"
	if err := repo.Update(a); err != nil {
		t.Fatalf("update: %v", err)
	}

	out, _ := repo.Get(a.ID)
	if out.Enabled {
		t.Error("enabled should be false")
	}
	if out.BaseURL != "https://new.example.com" {
		t.Errorf("baseURL=%s", out.BaseURL)
	}
	if out.Status != "healthy" {
		t.Errorf("status=%s", out.Status)
	}
	if out.UpdatedAt.Before(a.CreatedAt) {
		t.Errorf("updated_at should not regress (created=%s updated=%s)", a.CreatedAt, out.UpdatedAt)
	}
}

func TestIndexerRepoDelete(t *testing.T) {
	s := openTestStore(t)
	repo := NewIndexerRepo(s)
	a := sampleIndexer("alpha")
	_ = repo.Create(a)
	if err := repo.Delete(a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := repo.Get(a.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestIndexerRepoListEnabledOrdered(t *testing.T) {
	s := openTestStore(t)
	repo := NewIndexerRepo(s)

	a := sampleIndexer("alpha")
	a.Enabled = false
	b := sampleIndexer("beta")
	b.Enabled = true
	c := sampleIndexer("gamma")
	c.Enabled = true
	_ = repo.Create(a)
	_ = repo.Create(b)
	_ = repo.Create(c)

	enabled, err := repo.ListEnabled()
	if err != nil {
		t.Fatalf("list enabled: %v", err)
	}
	if len(enabled) != 2 {
		t.Errorf("len=%d want 2", len(enabled))
	}
	// Stable order: by created_at (insertion order in this test).
	if enabled[0].Name != "beta" || enabled[1].Name != "gamma" {
		t.Errorf("order: %s %s", enabled[0].Name, enabled[1].Name)
	}
}

func TestIndexerRepoSetStatus(t *testing.T) {
	s := openTestStore(t)
	repo := NewIndexerRepo(s)
	a := sampleIndexer("alpha")
	_ = repo.Create(a)

	now := time.Date(2020, 1, 1, 13, 0, 0, 0, time.UTC)
	if err := repo.SetStatus(a.ID, "healthy", &now, nil, 250); err != nil {
		t.Fatalf("set status: %v", err)
	}
	out, _ := repo.Get(a.ID)
	if out.Status != "healthy" {
		t.Errorf("status=%s", out.Status)
	}
	if out.ResponseTimeMs != 250 {
		t.Errorf("responseTimeMs=%d", out.ResponseTimeMs)
	}
	if out.LastCheckedAt == nil || !out.LastCheckedAt.Equal(now) {
		t.Errorf("lastCheckedAt=%v want %s", out.LastCheckedAt, now)
	}
}

func TestIndexerRepoRecordHealthEvent(t *testing.T) {
	s := openTestStore(t)
	repo := NewIndexerRepo(s)
	a := sampleIndexer("alpha")
	_ = repo.Create(a)

	if err := repo.RecordHealthEvent(a.ID, "healthy", 250, "", ""); err != nil {
		t.Fatalf("record event: %v", err)
	}
	if err := repo.RecordHealthEvent(a.ID, "error", 0, "INDEXER_TIMEOUT", "boom"); err != nil {
		t.Fatalf("record event 2: %v", err)
	}
	events, err := repo.ListHealthEvents(a.ID, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len=%d want 2", len(events))
	}
	if events[0].Status != "error" {
		t.Errorf("first status=%s", events[0].Status)
	}
	if events[0].ErrorCode != "INDEXER_TIMEOUT" {
		t.Errorf("first code=%s", events[0].ErrorCode)
	}
	if events[1].Status != "healthy" {
		t.Errorf("second status=%s", events[1].Status)
	}
}