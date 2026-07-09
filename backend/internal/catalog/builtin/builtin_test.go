package builtin_test

import (
	"testing"

	"github.com/local/easysearch/backend/internal/catalog/builtin"
	"github.com/local/easysearch/backend/internal/catalog"
)

func TestReadManifest_parsesEmbedded(t *testing.T) {
	m, err := builtin.ReadManifest()
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if m.Schema != 1 {
		t.Errorf("schema=%d want 1", m.Schema)
	}
	if m.Version == "" {
		t.Errorf("version empty")
	}
	if len(m.Definitions) == 0 {
		t.Errorf("no definitions")
	}
}

func TestVerifyChecksum_passesForAllEmbedded(t *testing.T) {
	m, err := builtin.ReadManifest()
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	for _, e := range m.Definitions {
		ok, err := builtin.VerifyChecksum(e.ID)
		if err != nil {
			t.Errorf("verify %s: %v", e.ID, err)
			continue
		}
		if !ok {
			t.Errorf("checksum mismatch for %s", e.ID)
		}
	}
}

func TestDefinitionYAML_parsesAndValidates(t *testing.T) {
	m, err := builtin.ReadManifest()
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	for _, e := range m.Definitions {
		raw, err := builtin.DefinitionYAML(e.ID)
		if err != nil {
			t.Errorf("read %s: %v", e.ID, err)
			continue
		}
		def, err := catalog.LoadDefinition(raw, e.File)
		if err != nil {
			t.Errorf("parse %s: %v", e.ID, err)
			continue
		}
		if err := catalog.Validate(def); err != nil {
			t.Errorf("validate %s: %v", e.ID, err)
		}
	}
}

func TestAllYAMLs_returnsMap(t *testing.T) {
	m, err := builtin.AllYAMLs()
	if err != nil {
		t.Fatalf("AllYAMLs: %v", err)
	}
	if len(m) == 0 {
		t.Fatalf("empty")
	}
}

func TestSortedIDs_stableOrder(t *testing.T) {
	a, _ := builtin.SortedIDs()
	b, _ := builtin.SortedIDs()
	if len(a) != len(b) {
		t.Fatalf("len mismatch")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("non-stable order at %d", i)
		}
	}
}