// Package indexer — declarative adapter (Phase 5 YAML, spec §12.3,
// JSON / XML added in Phase 8).
//
// The declarative adapter turns a YAML IndexerDefinition into a live
// IndexerAdapter: it composes the search URL with the restricted
// template engine, GETs through the indexer.Client, parses the response
// in HTML / JSON / XML form, and produces model.SearchResult values.
//
// Torznab is handled by its own torznab.go adapter; the declarative
// factory still rejects "torznab" so callers route to the right one.
package indexer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/local/easysearch/backend/internal/model"
)

// ErrFormatUnsupported is returned when a definition asks for a
// response format the current adapter can't handle.
var ErrFormatUnsupported = errors.New("declarative adapter: response format not supported in this build")

// declarativeAdapter implements IndexerAdapter from a model.IndexerDefinition.
type declarativeAdapter struct {
	id       string
	def      model.IndexerDefinition
	installed model.InstalledIndexer
}

func (a *declarativeAdapter) ID() string { return a.installed.ID }

// Test performs a minimal GET to confirm the URL is well-formed and
// the host returns non-empty HTML. We do NOT run a real query here;
// spec §16 says health probes should not pollute analytics.
func (a *declarativeAdapter) Test(ctx context.Context) TestResult {
	start := time.Now()
	u, err := a.buildURL("", "", "", "1")
	if err != nil {
		return TestResult{OK: false, DurationMs: ms(start), ErrorCode: "URL_BUILD", ErrorMessage: err.Error()}
	}
	resp, err := a.fetch(ctx, u.String())
	if err != nil {
		return TestResult{OK: false, DurationMs: ms(start), ErrorCode: "FETCH", ErrorMessage: err.Error()}
	}
	defer DiscardAndClose(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TestResult{
			OK: false, StatusCode: resp.StatusCode, DurationMs: ms(start),
			ErrorCode: "HTTP", ErrorMessage: fmt.Sprintf("status %d", resp.StatusCode),
		}
	}
	// Sniff a few bytes; we don't bother parsing for the probe.
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if len(buf) == 0 {
		return TestResult{OK: false, StatusCode: resp.StatusCode, DurationMs: ms(start), ErrorCode: "EMPTY"}
	}
	return TestResult{OK: true, StatusCode: resp.StatusCode, DurationMs: ms(start)}
}

// Search runs a single user query and returns the raw results.
func (a *declarativeAdapter) Search(ctx context.Context, q model.SearchQuery) ([]model.SearchResult, error) {
	page := "1"
	u, err := a.buildURL(q.Keyword, q.Category, "", page)
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}
	resp, err := a.fetch(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer DiscardAndClose(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &StatusCodeError{StatusCode: resp.StatusCode, URL: resp.Request.URL}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	switch a.def.Result.Format {
	case "", "html":
		return a.runHTML(body)
	case "json":
		return a.runJSON(body, a.def.Result)
	case "xml":
		return a.runXML(body, a.def.Result)
	}
	return nil, ErrFormatUnsupported
}

// runHTML parses an HTML response body and feeds each matching row
// through the existing extractRow pipeline.
func (a *declarativeAdapter) runHTML(body []byte) ([]model.SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	out := make([]model.SearchResult, 0)
	rows := doc.Find(a.def.Result.Rows.Selector)
	if a.def.Result.Rows.Selector == "" {
		// If no row selector, treat the doc root as a single row.
		rows = doc.Find("body")
	}
	rows.Each(func(_ int, row *goquery.Selection) {
		res, ok := a.extractRow(row, a.installed.ID, a.installed.Name)
		if !ok {
			return
		}
		out = append(out, res)
	})
	return out, nil
}

func (a *declarativeAdapter) buildURL(keyword, category, categoryID, page string) (*url.URL, error) {
	def := a.def.Search
	if def.Method == "" {
		def.Method = "GET"
	}
	if def.Method != "GET" && def.Method != "POST" {
		return nil, fmt.Errorf("unsupported method %q", def.Method)
	}

	data := TemplateData{
		Indexer: IndexerData{BaseURL: a.installed.BaseURL},
		Query: QueryData{
			Keyword:    keyword,
			Category:   category,
			CategoryID: categoryID,
			Page:       page,
		},
	}

	// Render the query string values, joining key=value pairs.
	q := url.Values{}
	for k, v := range def.Query {
		rendered, err := RenderTemplate(v, data)
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", k, err)
		}
		// For an empty render we still emit the key so sites that
		// require the parameter even empty don't choke.
		q.Set(k, rendered)
	}

	// Build the final URL: base + path + query.
	base, err := url.Parse(a.installed.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("base url: %w", err)
	}
	ref, err := url.Parse(def.Path)
	if err != nil {
		return nil, fmt.Errorf("path: %w", err)
	}
	final := base.ResolveReference(ref)
	if len(q) > 0 {
		final.RawQuery = q.Encode()
	}
	if def.Method == "POST" {
		// Mutating POST: the body comes from search.body; the URL
		// itself carries no query string in this branch.
		final.RawQuery = ""
	}
	return final, nil
}

func (a *declarativeAdapter) fetch(ctx context.Context, target string) (*http.Response, error) {
	if a.def.Search.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(a.def.Search.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	c := currentClient()
	if c == nil {
		return nil, errors.New("declarative adapter: no HTTP client configured")
	}
	return c.Get(ctx, target)
}

// withClient is called by the factory so the per-adapter c is set in
// package state during construction. It's simplistic — the indexer
// engine uses a single client per process — but avoids plumbing a
// per-adapter pointer through every API path.

func (a *declarativeAdapter) extractRow(row *goquery.Selection, indexerID, indexerName string) (model.SearchResult, bool) {
	res := model.SearchResult{
		IndexerID:   indexerID,
		IndexerName: indexerName,
		ID:          fmt.Sprintf("%s:%s:%d", indexerID, "0", rowIndex(row)),
	}

	for fieldName, f := range a.def.Result.Fields {
		raw := readField(row, f)
		if raw == "" {
			if f.Required {
				return res, false
			}
			continue
		}
		// Implicit resolve_url: if the field is declared ResolveURL, append
		// resolve_url to the filter pipeline rather than require the author
		// to write it explicitly (matches spec §13.6 ergonomics).
		filters := f.Filters
		if f.ResolveURL {
			filters = append(filters, "resolve_url")
		}
		val, err := ApplyFiltersByLayout(raw, filters, a.installed.BaseURL, f.DateLayouts)
		if err != nil {
			// Filter failures on optional fields are silent.
			if !f.Required {
				continue
			}
			return res, false
		}
		assignField(&res, fieldName, val)
	}

	// Spec §13.4: a result without any download entry (magnet/torrent/
	// direct) cannot be displayed as downloadable. If the YAML didn't
	// yield any of them we keep the row for the UI to mark as detail-only.
	if res.MagnetURL == "" && res.TorrentURL == "" && res.DirectURL == "" {
		if res.InfoHash != "" {
			// hash alone is enough; magnet is implicitly reconstructable
			res.MagnetURL = "magnet:?xt=urn:btih:" + strings.ToLower(res.InfoHash)
		}
	}
	if res.Title == "" {
		return res, false
	}
	// NormalizeTitle for dedup — kept lazy: defer to search pipeline.
	res.NormalizedTitle = strings.ToLower(strings.TrimSpace(res.Title))
	return res, true
}

// readField pulls text or attribute from a field's selector, scoped to row.
func readField(row *goquery.Selection, f model.FieldDefinition) string {
	if f.Selector == "" {
		return ""
	}
	sel := row.Find(f.Selector)
	if sel.Length() == 0 {
		return ""
	}
	switch strings.ToLower(f.Value) {
	case "text":
		return strings.TrimSpace(sel.First().Text())
	case "html", "":
		// Raw outer HTML — useful for capturing <a> snippets; we still
		// trim spaces for downstream filters.
		h, _ := sel.First().Html()
		return strings.TrimSpace(h)
	case "attr":
		v, ok := sel.First().Attr(f.Attribute)
		if !ok {
			return ""
		}
		return v
	default:
		return ""
	}
}

// assignField places a parsed string value into the matching
// SearchResult field, applying light type coercion for numbers and
// dates whose filters have already normalized them.
func assignField(r *model.SearchResult, name, val string) {
	switch name {
	case "title":
		r.Title = val
	case "category":
		r.Category = val
	case "size":
		if v, err := strconv.ParseInt(val, 10, 64); err == nil {
			r.SizeBytes = &v
		} else {
			r.SizeBytes = nil
		}
	case "seeders":
		if v, err := strconv.Atoi(val); err == nil {
			r.Seeders = &v
		}
	case "leechers":
		if v, err := strconv.Atoi(val); err == nil {
			r.Leechers = &v
		}
	case "downloads":
		if v, err := strconv.Atoi(val); err == nil {
			r.Downloads = &v
		}
	case "published_at":
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			t = t.UTC()
			r.PublishedAt = &t
		}
	case "magnet_url":
		r.MagnetURL = val
	case "torrent_url":
		r.TorrentURL = val
	case "direct_url":
		r.DirectURL = val
	case "detail_url":
		r.DetailURL = val
	case "info_hash":
		r.InfoHash = strings.ToUpper(val)
	default:
		// Unknown field: ignore. The validator doesn't yet enforce the
		// §13.4 field allow-list; that's a future tightening.
	}
}

// rowIndex gives every row within a single HTML page a distinct id
// when no natural id exists. goquery's Selection doesn't expose its
// own index here, so we use a process-level counter.
var rowCounter int

func rowIndex(s *goquery.Selection) int {
	rowCounter++
	return rowCounter
}

// The factory the registry uses.

type declarativeFactory struct{}

// NewDeclarativeFactory returns the factory the registry looks up for
// protocol "declarative". Pass the process-wide *Client so SSRF
// defenses apply.
func NewDeclarativeFactory() AdapterFactory { return declarativeFactory{} }

func (declarativeFactory) Create(def model.IndexerDefinition, installed model.InstalledIndexer, client *Client) (IndexerAdapter, error) {
	// The factory only owns the declarative family of formats. Torznab
	// (and any future non-declarative protocol) must be routed to its
	// own factory — refuse them here so the registry's Get+Create path
	// never silently falls back to the wrong parser.
	switch def.Result.Format {
	case "", "html", "json", "xml":
		// supported
	default:
		return nil, fmt.Errorf("%w: %q (declarative factory accepts html|json|xml)", ErrFormatUnsupported, def.Result.Format)
	}
	if installed.BaseURL == "" {
		return nil, errors.New("declarative adapter: installed.BaseURL is empty")
	}
	if client != nil {
		setClient(client)
	}
	return &declarativeAdapter{
		id:        installed.ID,
		def:       def,
		installed: installed,
	}, nil
}

func init() {
	Default.Register(ProtocolDeclarative, declarativeFactory{})
}

// SetClientForTest installs the given client into the package-level
// global used by declarative / torznab adapters during HTTP requests.
// It is intended for integration tests that drive adapters directly
// without going through the registry. Production code should construct
// adapters with explicit *Client values via the factory Create path.
func SetClientForTest(c *Client) { setClient(c) }

// Package-scope client holder so fetch() can stay signature-free.
// Mutated by the factory at create time; safe because factories are
// called synchronously before the adapter enters the worker pool.
var (
	clientMu sync.Mutex
	clientPtr *Client
)

func setClient(c *Client) {
	clientMu.Lock()
	clientPtr = c
	clientMu.Unlock()
}

func currentClient() *Client {
	clientMu.Lock()
	defer clientMu.Unlock()
	return clientPtr
}

func ms(start time.Time) int64 { return time.Since(start).Milliseconds() }
