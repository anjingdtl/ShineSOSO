package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/local/easysearch/backend/internal/model"
)

func mkDef(id, version, content string) model.ImportedDefinition {
	sum := sha256.Sum256([]byte(content))
	return model.ImportedDefinition{
		ID:       id,
		Version:  version,
		Content:  content,
		Checksum: hex.EncodeToString(sum[:]),
	}
}

func TestImportedDefinitionRepo_CRUD(t *testing.T) {
	s := openTestStore(t)
	repo := NewImportedDefinitionRepo(s)

	d := mkDef("example-public", "1.0.0", "name: Example\nid: example-public\n")
	if err := repo.Create(d); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.Get(d.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Checksum == "" {
		t.Errorf("checksum should be set")
	}
	if got.Content != d.Content {
		t.Errorf("content mismatch")
	}

	// Duplicate id
	err = repo.Create(d)
	if !errors.Is(err, ErrDuplicate) {
		t.Errorf("want ErrDuplicate, got %v", err)
	}

	all, err := repo.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 || all[0].ID != d.ID {
		t.Errorf("list wrong: %v", all)
	}

	// Delete
	if err := repo.Delete(d.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(d.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
	if err := repo.Delete(d.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing should return ErrNotFound")
	}
}

func TestImportedDefinitionRepo_BlankChecksumGetsFilled(t *testing.T) {
	s := openTestStore(t)
	repo := NewImportedDefinitionRepo(s)

	d := model.ImportedDefinition{
		ID:      "id1",
		Version: "1.0.0",
		Content: "abc",
		// Checksum intentionally omitted
	}
	if err := repo.Create(d); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get("id1")
	if got.Checksum == "" {
		t.Errorf("blank checksum should be filled in Create")
	}
	want := sha256.Sum256([]byte("abc"))
	if got.Checksum != hex.EncodeToString(want[:]) {
		t.Errorf("checksum wrong: got %s", got.Checksum)
	}
}
