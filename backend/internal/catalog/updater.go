// Package catalog — Phase 6 catalog updater (spec §26.3).
//
// The updater fetches a manifest.json from a configurable URL, verifies
// every YAML file's SHA-256 against the manifest, validates each one
// against spec §13.8, and atomically swaps them into the in-memory
// catalog. Failure of any single step rolls back to the previous set.
//
// Storage layout on disk:
//
//	<data_dir>/catalog-cache/
//	├── manifest.json
//	└── definitions/
//	    ├── example-public-html.yml
//	    └── example-torznab.yml
//
// The cached copy is the durable record: on next boot, if the manifest
// is present and parses, we prefer it over the embedded copy.
package catalog

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/local/easysearch/backend/internal/catalog/builtin"
	"github.com/local/easysearch/backend/internal/model"
)

// UpdaterConfig configures the catalog updater.
type UpdaterConfig struct {
	ManifestURL string        // "" disables online updates
	HTTPClient  *http.Client  // nil → http.DefaultClient
	CacheDir    string        // where to write downloaded manifest + YAML
	Logger      *slog.Logger
	Now         func() time.Time // injectable clock for tests
	// OnDefinitionActivated, when non-nil, is called once per (definitionID,
	// newVersion) pair right after a new manifest is activated. Use it to
	// bump definition_version on installed_indexers rows.
	OnDefinitionActivated func(definitionID, newVersion string) error
}

// Updater fetches, verifies, and activates catalog updates.
type Updater struct {
	cfg     UpdaterConfig
	catalog *Catalog

	mu     sync.Mutex
	active *activeManifest // most recent activated state
}

type activeManifest struct {
	Version   string
	Source    string // "embedded" | "cached" | "remote"
	Activated time.Time
	Entries   []builtin.ManifestEntry
}

// NewUpdater wires an Updater against a Catalog. CacheDir is created
// on first Update; pass an empty string to disable on-disk caching
// (useful for tests).
func NewUpdater(c *Catalog, cfg UpdaterConfig) *Updater {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.CacheDir == "" {
		cfg.CacheDir = filepath.Join(os.TempDir(), "easysearch-catalog-cache")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Updater{cfg: cfg, catalog: c}
}

// UpdateReport is what Update returns and what the API surfaces to the UI.
type UpdateReport struct {
	Before  string             `json:"before"`  // version before update
	After   string             `json:"after"`   // version after update
	Source  string             `json:"source"`  // where the new manifest came from
	Changed []ChangedDefinition `json:"changed"` // definitions that differ from the old set
	Added   []string           `json:"added"`   // ids not present before
	Removed []string           `json:"removed"` // ids present before but missing now
}

// ChangedDefinition describes a single definition whose bytes differ
// between the old and new manifests.
type ChangedDefinition struct {
	ID          string `json:"id"`
	OldVersion  string `json:"oldVersion,omitempty"`
	NewVersion  string `json:"newVersion,omitempty"`
	OldSHA256   string `json:"oldSha256,omitempty"`
	NewSHA256   string `json:"newSha256,omitempty"`
}

// Sentinel errors.
var (
	ErrManifestUnchanged = errors.New("catalog: manifest is unchanged from the active version")
	ErrManifestInvalid   = errors.New("catalog: manifest failed validation")
	ErrChecksumMismatch  = errors.New("catalog: definition sha256 mismatch")
	ErrDefinitionInvalid = errors.New("catalog: downloaded definition failed spec §13.8 validation")
	ErrManifestHTTP      = errors.New("catalog: manifest fetch failed")
)

// Activate loads a manifest and definitions from a local directory
// (cache or test fixture), verifies checksums, validates each YAML,
// and swaps them into the catalog. On any failure the previous active
// state remains untouched.
func (u *Updater) Activate(fsys fs.FS, dir string) (*UpdateReport, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.activateLocked(fsys, dir, "cache")
}

// ActivateAsEmbedded is the boot path used by ActivateEmbedded — it
// tags the activated state with source="embedded" so the API can
// label it correctly.
func (u *Updater) ActivateAsEmbedded() (*UpdateReport, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.activateLocked(builtin.FS(), "", "embedded")
}

// activateLocked is the body of Activate without the mutex acquisition.
// Callers that already hold the mutex use this directly to avoid
// self-deadlock. The `source` label is recorded in the active state so
// the API and UI can tell where the manifest came from.
func (u *Updater) activateLocked(fsys fs.FS, dir string, source string) (*UpdateReport, error) {
	oldVersion := ""
	if u.active != nil {
		oldVersion = u.active.Version
	}

	manifestRaw, err := fs.ReadFile(fsys, joinFS(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m builtin.Manifest
	if err := json.Unmarshal(manifestRaw, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrManifestInvalid, err)
	}
	if m.Schema != 1 {
		return nil, fmt.Errorf("%w: schema %d", ErrManifestInvalid, m.Schema)
	}

	// Validate each YAML against the declared SHA-256.
	loaded := make(map[string]model.IndexerDefinition, len(m.Definitions))
	newEntries := make([]builtin.ManifestEntry, 0, len(m.Definitions))
	for _, e := range m.Definitions {
		full := joinFS(dir, e.File)
		raw, err := fs.ReadFile(fsys, full)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", full, err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != e.SHA256 {
			return nil, fmt.Errorf("%w: %s (manifest says %s, got %s)",
				ErrChecksumMismatch, e.ID, e.SHA256, hex.EncodeToString(sum[:]))
		}
		def, err := LoadDefinition(raw, e.File)
		if err != nil {
			return nil, fmt.Errorf("%w: parse %s: %v", ErrDefinitionInvalid, e.ID, err)
		}
		if err := Validate(def); err != nil {
			return nil, fmt.Errorf("%w: validate %s: %v", ErrDefinitionInvalid, e.ID, err)
		}
		loaded[def.ID] = def
		newEntries = append(newEntries, e)
	}

	// Optional Ed25519 signature gate, layered on top of SHA-256.
	// When EASYSEARCH_CATALOG_PUBKEY is empty (the default), this
	// step is a no-op and behaviour is unchanged from prior versions.
	if pubB64 := os.Getenv("EASYSEARCH_CATALOG_PUBKEY"); pubB64 != "" {
		pub, err := base64.StdEncoding.DecodeString(pubB64)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("updater: invalid EASYSEARCH_CATALOG_PUBKEY: %w", err)
		}
		signing, err := m.SigningBytes()
		if err != nil {
			return nil, fmt.Errorf("updater: marshal for signing: %w", err)
		}
		if !Verify(signing, m.Signature, pub) {
			return nil, errors.New("updater: catalog signature verification failed")
		}
	}

	// Compute the diff before we touch state.
	oldEntries := map[string]builtin.ManifestEntry{}
	if u.active != nil {
		for _, e := range u.active.Entries {
			oldEntries[e.ID] = e
		}
	}
	changed, added, removed := diffEntries(oldEntries, newEntries)

	// Swap atomically: register new definitions FIRST so callers reading
	// the catalog never see an empty window. We never remove the old
	// entries until after the new ones are in place; if same id exists,
	// RegisterDefinition overwrites — desired behavior.
	report := &UpdateReport{
		Before:  oldVersion,
		After:   m.Version,
		Source:  source,
		Changed: changed,
		Added:   added,
		Removed: removed,
	}
	for _, d := range loaded {
		u.catalog.RegisterDefinition(d)
	}

	u.active = &activeManifest{
		Version:   m.Version,
		Source:    source,
		Activated: u.cfg.Now().UTC(),
		Entries:   newEntries,
	}

	// Fire the version-bump hook for every activated definition so the
	// installed_indexers table stays in sync with the catalog. Failures
	// here don't roll back the activation — the version bump is a
	// bookkeeping update and we'll retry on the next update.
	if u.cfg.OnDefinitionActivated != nil {
		for _, e := range newEntries {
			if err := u.cfg.OnDefinitionActivated(e.ID, e.Version); err != nil {
				u.cfg.Logger.Warn("bump definition_version failed",
					"definition_id", e.ID,
					"new_version", e.Version,
					"err", err,
				)
			}
		}
	}

	return report, nil
}

// ActivateEmbedded is the no-network boot path: install the embedded
// manifest if no active state is recorded yet (idempotent on re-run).
func (u *Updater) ActivateEmbedded() (*UpdateReport, error) {
	u.mu.Lock()
	if u.active != nil {
		u.mu.Unlock()
		return &UpdateReport{
			Before: u.active.Version,
			After:  u.active.Version,
			Source: u.active.Source,
		}, ErrManifestUnchanged
	}
	u.mu.Unlock()
	return u.ActivateAsEmbedded()
}

// Fetch downloads a remote manifest + every YAML it references,
// writes them under CacheDir (atomic per file), then calls Activate.
// The CacheDir layout mirrors the embedded one so Activate is
// agnostic to source.
func (u *Updater) Fetch(ctx context.Context) (*UpdateReport, error) {
	if u.cfg.ManifestURL == "" {
		return nil, errors.New("catalog: no ManifestURL configured")
	}
	if err := os.MkdirAll(filepath.Join(u.cfg.CacheDir, "definitions"), 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.cfg.ManifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build manifest request: %w", err)
	}
	resp, err := u.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrManifestHTTP, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%w: status %d: %s", ErrManifestHTTP, resp.StatusCode, body)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read manifest body: %w", err)
	}

	// Quick sanity parse to know which files to fetch before we write
	// anything to disk.
	var m builtin.Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrManifestInvalid, err)
	}
	if m.Schema != 1 {
		return nil, fmt.Errorf("%w: schema %d", ErrManifestInvalid, m.Schema)
	}

	// Stage the new manifest in a temp file and rename atomically.
	manifestPath := filepath.Join(u.cfg.CacheDir, "manifest.json")
	if err := writeFileAtomic(manifestPath, body, 0o644); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	// Fetch each definition. We tolerate failure of any single file
	// here ONLY if the manifest already has that file cached — but
	// since this is the fetch path, we fail hard on any individual
	// failure to keep semantics simple (and avoid half-installed state).
	for _, e := range m.Definitions {
		if !strings.HasPrefix(e.File, "definitions/") {
			return nil, fmt.Errorf("%w: file %q must start with definitions/", ErrManifestInvalid, e.File)
		}
		defURL, err := resolveRelative(u.cfg.ManifestURL, e.File)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", e.File, err)
		}
		if err := u.fetchOne(ctx, defURL, filepath.Join(u.cfg.CacheDir, e.File), e.SHA256); err != nil {
			// Roll back: remove the staged manifest so the next
			// boot still sees the previous good state.
			_ = os.Remove(manifestPath)
			return nil, fmt.Errorf("fetch %s: %w", e.File, err)
		}
	}

	// Activate from the now-populated cache dir.
	return u.activateLocked(os.DirFS(u.cfg.CacheDir), "", "remote")
}

// ActiveVersion returns the currently-active manifest version + source
// (or empty string if none active yet).
func (u *Updater) ActiveVersion() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.active == nil {
		return ""
	}
	return u.active.Version
}

// ActiveSource returns the origin of the active manifest (one of
// "embedded", "cache", "remote") so the UI can label it.
func (u *Updater) ActiveSource() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.active == nil {
		return ""
	}
	return u.active.Source
}

// fetchOne downloads a single YAML, checks its SHA-256 against expected,
// and writes it atomically.
func (u *Updater) fetchOne(ctx context.Context, url, destPath, expectedSHA string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := u.cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxDefinitionBytes))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != expectedSHA {
		return fmt.Errorf("%w: declared %s got %s", ErrChecksumMismatch, expectedSHA, hex.EncodeToString(sum[:]))
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(destPath, body, 0o644)
}

// writeFileAtomic writes data to path via a sibling temp file + rename,
// so partial files never become visible.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// joinFS joins path segments using the forward-slash separator that
// io/fs requires. filepath.Join uses the OS separator which on Windows
// yields "definitions\\foo.yml" — invalid for fs.FS. A "." prefix
// segment (the root indicator) is dropped because io/fs roots must
// not begin with "./".
func joinFS(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		cleaned = append(cleaned, p)
	}
	return strings.Join(cleaned, "/")
}

// resolveRelative joins a base URL with a relative reference, preserving
// the base's query/fragment.
func resolveRelative(baseURL, ref string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	r, err := base.Parse(ref)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}

// diffEntries returns (changed, added, removed) between two manifests.
// `changed` is sorted by ID for stable output.
func diffEntries(oldEntries map[string]builtin.ManifestEntry, newEntries []builtin.ManifestEntry) ([]ChangedDefinition, []string, []string) {
	newMap := make(map[string]builtin.ManifestEntry, len(newEntries))
	for _, e := range newEntries {
		newMap[e.ID] = e
	}
	var changed []ChangedDefinition
	var added []string
	for _, e := range newEntries {
		old, ok := oldEntries[e.ID]
		if !ok {
			added = append(added, e.ID)
			continue
		}
		if old.Version != e.Version || old.SHA256 != e.SHA256 {
			changed = append(changed, ChangedDefinition{
				ID:         e.ID,
				OldVersion: old.Version,
				NewVersion: e.Version,
				OldSHA256:  old.SHA256,
				NewSHA256:  e.SHA256,
			})
		}
	}
	var removed []string
	for id := range oldEntries {
		if _, ok := newMap[id]; !ok {
			removed = append(removed, id)
		}
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].ID < changed[j].ID })
	sort.Strings(added)
	sort.Strings(removed)
	return changed, added, removed
}