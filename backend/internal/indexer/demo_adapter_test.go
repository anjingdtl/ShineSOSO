package indexer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/local/easysearch/backend/internal/model"
)

func TestDemoAdapter_Alpha(t *testing.T) {
	a := newDemoAdapter("demo-alpha", "示例 A (内置)")
	if a.ID() != "demo-alpha" {
		t.Errorf("ID want demo-alpha got %q", a.ID())
	}
	if a.Name() != "示例 A (内置)" {
		t.Errorf("Name want 示例 A (内置) got %q", a.Name())
	}
	tr := a.Test(context.Background())
	if !tr.OK {
		t.Errorf("alpha Test should succeed, got %+v", tr)
	}
	if tr.StatusCode != 200 {
		t.Errorf("alpha Test status want 200 got %d", tr.StatusCode)
	}
}

func TestDemoAdapter_Beta(t *testing.T) {
	a := newDemoAdapter("demo-beta", "示例 B (内置)")
	tr := a.Test(context.Background())
	if !tr.OK {
		t.Errorf("beta Test should succeed, got %+v", tr)
	}
}

func TestDemoAdapter_Gamma_AlwaysFails(t *testing.T) {
	a := newDemoAdapter("demo-gamma", "示例 C (故意失败)")
	tr := a.Test(context.Background())
	if tr.OK {
		t.Errorf("gamma Test should fail, got %+v", tr)
	}
	if tr.ErrorCode == "" {
		t.Errorf("gamma Test should have an error code, got %+v", tr)
	}
	_, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err == nil {
		t.Error("gamma Search should return error")
	}
}

func TestDemoAdapter_Alpha_ReturnsThreeResults(t *testing.T) {
	a := newDemoAdapter("demo-alpha", "示例 A (内置)")
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "matrix"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("want 3 results, got %d", len(res))
	}
	// Each result should have an indexer id and name set.
	for _, r := range res {
		if r.IndexerID != "demo-alpha" {
			t.Errorf("IndexerID want demo-alpha got %q", r.IndexerID)
		}
		if r.IndexerName != "示例 A (内置)" {
			t.Errorf("IndexerName want 示例 A (内置) got %q", r.IndexerName)
		}
	}
	// First should be a magnet link.
	if !strings.HasPrefix(res[0].MagnetURL, "magnet:") {
		t.Errorf("first result should have magnet, got %q", res[0].MagnetURL)
	}
	// Second should have a torrent URL.
	if !strings.HasSuffix(res[1].TorrentURL, ".torrent") {
		t.Errorf("second result should have torrent, got %q", res[1].TorrentURL)
	}
	// Third should have a direct URL.
	if !strings.HasSuffix(res[2].DirectURL, ".zip") {
		t.Errorf("third result should have direct, got %q", res[2].DirectURL)
	}
}

func TestDemoAdapter_SearchRespectsDelay(t *testing.T) {
	a := newDemoAdapter("demo-alpha", "")
	a.delay = 50 * time.Millisecond
	start := time.Now()
	_, _ = a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("expected >= 50ms delay, got %v", elapsed)
	}
}

func TestDemoAdapter_UnknownID(t *testing.T) {
	a := newDemoAdapter("demo-unknown", "")
	// Should still be constructable and functional; just no preset
	// delay/fail behavior.
	if a.ID() != "demo-unknown" {
		t.Errorf("ID should be preserved")
	}
	tr := a.Test(context.Background())
	if !tr.OK {
		t.Errorf("unknown demo should succeed by default, got %+v", tr)
	}
}