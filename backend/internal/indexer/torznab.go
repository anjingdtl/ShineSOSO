// Package indexer — Torznab adapter (Phase 6, spec §14).
//
// Torznab is an RSS dialect with Torznab-specific <torznab:attr> nodes
// carrying size, seeders, infohash, etc. The adapter mirrors the
// declarative adapter's shape:
//
//   - build URL:   {base_url}/api?t=search&q={keyword}&cat={cat}
//   - parse body:  RSS <item>s with torznab:attr overrides
//   - emit results with magnet/torrent/infohash populated
//
// Unlike the declarative adapter, the response shape is fixed by the
// spec, so we don't expose field selector maps; the field extraction
// logic in extractTorznabItem knows exactly where each piece lives.
package indexer

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/local/easysearch/backend/internal/model"
)

// torznabAdapter implements IndexerAdapter for protocol "torznab".
type torznabAdapter struct {
	def        model.IndexerDefinition
	installed  model.InstalledIndexer
	httpClient *Client
}

// torznabRSS is the top-level RSS envelope we parse. We only care about
// the channel/item subtree; everything else is ignored.
type torznabRSS struct {
	XMLName xml.Name    `xml:"rss"`
	Channel torznabChan `xml:"channel"`
}

type torznabChan struct {
	Title string        `xml:"title"`
	Items []torznabItem `xml:"item"`
}

type torznabItem struct {
	Title       string           `xml:"title"`
	GUID        string           `xml:"guid"`
	Link        string           `xml:"link"`
	Comments    string           `xml:"comments"`
	PubDate     string           `xml:"pubDate"`
	Category    string           `xml:"category"`
	Description string           `xml:"description"`
	Enclosure   torznabEnclosure `xml:"enclosure"`
	Attrs       []torznabAttr    `xml:"http://torznab.com/schemas/2015/feed attr"`
}

type torznabEnclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

type torznabAttr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// NewTorznabFactory returns the factory registered for ProtocolTorznab.
func NewTorznabFactory() AdapterFactory { return torznabFactory{} }

type torznabFactory struct{}

func (a *torznabAdapter) ID() string { return a.installed.ID }

func (torznabFactory) Create(def model.IndexerDefinition, installed model.InstalledIndexer, client *Client) (IndexerAdapter, error) {
	if def.Protocol != "torznab" {
		return nil, fmt.Errorf("torznab factory: protocol mismatch (got %q)", def.Protocol)
	}
	if installed.BaseURL == "" {
		return nil, errors.New("torznab adapter: installed.BaseURL is empty")
	}
	if client == nil {
		return nil, errors.New("torznab adapter: nil HTTP client")
	}
	return &torznabAdapter{def: def, installed: installed, httpClient: client}, nil
}

func init() {
	Default.Register(ProtocolTorznab, torznabFactory{})
}

// buildURL composes the request URL according to spec §14.1. The path
// default is /api and the parameter defaults are t/q/cat; both can be
// overridden in the definition's search block.
func (a *torznabAdapter) buildURL(keyword, category, categoryID, page string) (*url.URL, error) {
	def := a.def.Search
	if def.Method != "" && def.Method != "GET" {
		return nil, fmt.Errorf("torznab adapter only supports GET; got %q", def.Method)
	}

	base, err := url.Parse(a.installed.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("base url: %w", err)
	}
	path := def.Path
	if path == "" {
		path = "/api"
	}
	ref, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("path: %w", err)
	}
	final := base.ResolveReference(ref)

	q := url.Values{}
	for k, v := range def.Query {
		rendered, err := RenderTemplate(v, TemplateData{
			Query:   QueryData{Keyword: keyword, Category: category, CategoryID: categoryID, Page: page},
			Indexer: IndexerData{BaseURL: a.installed.BaseURL},
		})
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", k, err)
		}
		q.Set(k, rendered)
	}
	// Apply defaults only when the YAML didn't supply the key — spec §14.1.
	if _, ok := def.Query["t"]; !ok {
		q.Set("t", "search")
	}
	if _, ok := def.Query["q"]; !ok && keyword != "" {
		q.Set("q", keyword)
	}
	if _, ok := def.Query["cat"]; !ok && categoryID != "" {
		q.Set("cat", categoryID)
	}
	if len(q) > 0 {
		final.RawQuery = q.Encode()
	}
	return final, nil
}

func (a *torznabAdapter) Test(ctx context.Context) TestResult {
	start := time.Now()
	u, err := a.buildURL("", "", "", "1")
	if err != nil {
		return TestResult{OK: false, DurationMs: ms(start), ErrorCode: "URL_BUILD", ErrorMessage: err.Error()}
	}
	if a.def.Search.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(a.def.Search.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	resp, err := a.httpClient.Get(ctx, u.String())
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
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if len(buf) == 0 {
		return TestResult{OK: false, StatusCode: resp.StatusCode, DurationMs: ms(start), ErrorCode: "EMPTY"}
	}
	return TestResult{OK: true, StatusCode: resp.StatusCode, DurationMs: ms(start)}
}

func (a *torznabAdapter) Search(ctx context.Context, q model.SearchQuery) ([]model.SearchResult, error) {
	// spec §14.1: the &cat= parameter takes a numeric category ID. The
	// orchestrator hands us the canonical category name; we pass it as
	// CategoryID when set (callers can pre-translate via the
	// indexer's categories map; for v1 we ship the name as-is and let
	// Torznab's lookup logic handle it).
	u, err := a.buildURL(q.Keyword, q.Category, q.Category, "1")
	if err != nil {
		return nil, fmt.Errorf("build url: %w", err)
	}
	if a.def.Search.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(a.def.Search.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	resp, err := a.httpClient.Get(ctx, u.String())
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

	var rss torznabRSS
	if err := xml.NewDecoder(bytes.NewReader(body)).Decode(&rss); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}

	out := make([]model.SearchResult, 0, len(rss.Channel.Items))
	for idx, it := range rss.Channel.Items {
		res, ok := a.extractTorznabItem(it, idx)
		if !ok {
			continue
		}
		res.IndexerID = a.installed.ID
		res.IndexerName = a.installed.Name
		res.NormalizedTitle = strings.ToLower(strings.TrimSpace(res.Title))
		out = append(out, res)
	}
	return out, nil
}

// extractTorznabItem maps a single <item> into a SearchResult. The
// priority order for download URLs is magnet → torrent → direct → detail
// (spec §6.8). Torznab attributes override RSS plain fields when both
// exist (this is the spec's intended semantic).
func (a *torznabAdapter) extractTorznabItem(it torznabItem, idx int) (model.SearchResult, bool) {
	attrs := indexTorznabAttrs(it.Attrs)
	res := model.SearchResult{
		ID:    fmt.Sprintf("%s:torznab:%d:%s", a.installed.ID, idx, it.GUID),
		Title: strings.TrimSpace(it.Title),
	}

	if v := attrString(attrs, "magneturl"); v != "" {
		res.MagnetURL = v
	}
	if v := attrString(attrs, "infohash"); v != "" {
		res.InfoHash = strings.ToUpper(v)
	}
	if res.MagnetURL == "" && res.InfoHash != "" {
		// Reconstruct a magnet so the UI can hand the user a copy-able link.
		res.MagnetURL = "magnet:?xt=urn:btih:" + strings.ToLower(res.InfoHash)
	}

	if res.MagnetURL == "" {
		// Fall back to <enclosure url="…"> when no magnet is present.
		switch strings.ToLower(it.Enclosure.Type) {
		case "application/x-bittorrent":
			res.TorrentURL = it.Enclosure.URL
		case "application/octet-stream", "":
			// Heuristic: URL ending in .torrent -> torrent; otherwise direct.
			if strings.HasSuffix(strings.ToLower(it.Enclosure.URL), ".torrent") {
				res.TorrentURL = it.Enclosure.URL
			} else if it.Enclosure.URL != "" {
				res.DirectURL = it.Enclosure.URL
			}
		}
	}

	res.DetailURL = it.Link
	res.Category = it.Category

	if it.PubDate != "" {
		if t, err := parseTorznabDate(it.PubDate); err == nil {
			t = t.UTC()
			res.PublishedAt = &t
		}
	}

	if v := attrString(attrs, "size"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			res.SizeBytes = &n
		}
	}
	if v := attrString(attrs, "seeders"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			res.Seeders = &n
		}
	}
	if v := attrString(attrs, "peers"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			res.Leechers = &n
		}
	}
	if v := attrString(attrs, "grabs"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			res.Downloads = &n
		}
	}

	if res.Title == "" {
		return res, false
	}
	return res, true
}

func indexTorznabAttrs(in []torznabAttr) map[string]string {
	out := make(map[string]string, len(in))
	for _, a := range in {
		out[strings.ToLower(a.Name)] = a.Value
	}
	return out
}

func attrString(m map[string]string, key string) string {
	if v, ok := m[key]; ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// parseTorznabDate parses the RFC1123Z / RFC1123 / RFC850 / ANSIC formats
// the spec calls out (these are the formats commonly produced by
// Torznab endpoints). It returns the first successful parse.
func parseTorznabDate(s string) (time.Time, error) {
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC850,
		time.ANSIC,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized pubDate %q", s)
}

// Compile-time assertion: torznabAdapter implements IndexerAdapter.
var _ IndexerAdapter = (*torznabAdapter)(nil)