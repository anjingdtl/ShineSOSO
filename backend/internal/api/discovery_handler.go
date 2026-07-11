package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/local/easysearch/backend/internal/indexer"
)

// DiscoveryHandler searches public web results locally, then only accepts a
// candidate after its Torznab capability endpoint answers successfully.
// Results are candidates, never silently-installed indexers.
type DiscoveryHandler struct {
	Logger        *slog.Logger
	SearchURL     string
	HTTP          *http.Client
	IndexerClient *indexer.Client
	CatalogURL    string
	mu            sync.Mutex
	catalog       []remoteIndexerDefinition
	catalogAt     time.Time
}
type discoverySearchRequest struct {
	Query string `json:"query"`
}
type discoveryCandidate struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Summary  string `json:"summary,omitempty"`
	Source   string `json:"source,omitempty"`
	Type     string `json:"type,omitempty"`
	Language string `json:"language,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Score    int    `json:"score,omitempty"`
}
type remoteIndexerDefinition struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Protocol    string   `json:"protocol"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Language    string   `json:"language"`
	Links       []string `json:"links"`
}
type discoveryProbeRequest struct {
	URL string `json:"url"`
}

func (h *DiscoveryHandler) searchURL(query string) string {
	base := h.SearchURL
	if base == "" {
		base = "https://html.duckduckgo.com/html/"
	}
	u, _ := url.Parse(base)
	q := u.Query()
	q.Set("q", query+" torznab indexer")
	u.RawQuery = q.Encode()
	return u.String()
}
func (h *DiscoveryHandler) httpClient() *http.Client {
	if h.HTTP != nil {
		return h.HTTP
	}
	return &http.Client{Timeout: 12 * time.Second}
}
func (h *DiscoveryHandler) Search(w http.ResponseWriter, r *http.Request) {
	var in discoverySearchRequest
	if json.NewDecoder(r.Body).Decode(&in) != nil || len([]rune(strings.TrimSpace(in.Query))) < 2 {
		WriteError(w, h.Logger, 400, ErrorPayload{Code: "INVALID_REQUEST", Message: "请至少输入两个字符"})
		return
	}
	query := strings.TrimSpace(in.Query)
	out, catalogErr := h.searchStructuredCatalog(r.Context(), query)
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, h.searchURL(query), nil)
	req.Header.Set("User-Agent", "EasySearch/0.1 local discovery")
	resp, err := h.httpClient().Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		doc, parseErr := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, 2<<20))
		if parseErr == nil {
			out = append(out, parseWebCandidates(doc)...)
		}
	} else if resp != nil {
		resp.Body.Close()
	}
	if len(out) == 0 && catalogErr != nil {
		WriteError(w, h.Logger, 502, ErrorPayload{Code: "DISCOVERY_UNAVAILABLE", Message: "结构化目录和公开搜索服务均暂不可用"})
		return
	}
	out = dedupeAndLimitCandidates(out, 50)
	WriteJSON(w, 200, map[string]any{"items": out, "structured": catalogErr == nil})
}

func parseWebCandidates(doc *goquery.Document) []discoveryCandidate {
	out := []discoveryCandidate{}
	seen := map[string]bool{}
	doc.Find(".result").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		a := s.Find(".result__a").First()
		raw, ok := a.Attr("href")
		if !ok {
			return true
		}
		target := decodeDuckDuckGoURL(raw)
		u, err := url.Parse(target)
		if err != nil || u.Scheme != "https" || u.Host == "" || seen[u.String()] {
			return true
		}
		seen[u.String()] = true
		out = append(out, discoveryCandidate{Name: strings.TrimSpace(a.Text()), URL: u.String(), Summary: strings.TrimSpace(s.Find(".result__snippet").Text()), Source: "全网搜索", Score: 20})
		return len(out) < 10
	})
	return out
}

func (h *DiscoveryHandler) searchStructuredCatalog(ctx context.Context, query string) ([]discoveryCandidate, error) {
	defs, err := h.loadRemoteCatalog(ctx)
	if err != nil {
		return nil, err
	}
	terms := expandDiscoveryTerms(query)
	out := []discoveryCandidate{}
	for _, d := range defs {
		if len(d.Links) == 0 || !strings.HasPrefix(d.Links[0], "https://") {
			continue
		}
		score := scoreRemoteDefinition(d, terms)
		if score == 0 {
			continue
		}
		out = append(out, discoveryCandidate{Name: d.Name, URL: d.Links[0], Summary: d.Description, Source: "Prowlarr 结构化目录", Type: d.Type, Language: d.Language, Protocol: d.Protocol, Score: score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > 40 {
		out = out[:40]
	}
	return out, nil
}
func (h *DiscoveryHandler) loadRemoteCatalog(ctx context.Context) ([]remoteIndexerDefinition, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.catalog) > 0 && time.Since(h.catalogAt) < 30*time.Minute {
		return append([]remoteIndexerDefinition(nil), h.catalog...), nil
	}
	endpoint := h.CatalogURL
	if endpoint == "" {
		endpoint = "https://indexers.prowlarr.com/master/11"
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("User-Agent", "EasySearch/0.1 catalog discovery")
	resp, err := h.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("catalog status %d", resp.StatusCode)
	}
	var defs []remoteIndexerDefinition
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&defs); err != nil {
		return nil, err
	}
	h.catalog = defs
	h.catalogAt = time.Now()
	return append([]remoteIndexerDefinition(nil), defs...), nil
}
func expandDiscoveryTerms(query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	terms := strings.Fields(q)
	syn := map[string][]string{"电影": {"movie", "movies", "film", "video"}, "影视": {"movie", "movies", "tv", "video"}, "电视剧": {"tv", "series", "television"}, "动漫": {"anime", "animation"}, "动画": {"anime", "animation"}, "音乐": {"music", "audio"}, "图书": {"book", "books", "ebook"}, "中文": {"chinese", "zh-cn", "zh-tw"}, "公开": {"public"}}
	for k, v := range syn {
		if strings.Contains(q, k) {
			terms = append(terms, v...)
		}
	}
	if len(terms) == 0 {
		terms = []string{q}
	}
	return uniqueStrings(terms)
}
func scoreRemoteDefinition(d remoteIndexerDefinition, terms []string) int {
	name := strings.ToLower(d.Name)
	hay := strings.ToLower(d.Name + " " + d.Description + " " + d.Language + " " + d.Protocol + " " + d.Type)
	score := 0
	for _, t := range terms {
		if name == t {
			score += 100
		} else if strings.HasPrefix(name, t) {
			score += 60
		} else if strings.Contains(name, t) {
			score += 40
		} else if strings.Contains(hay, t) {
			score += 15
		}
	}
	if d.Type == "public" {
		score += 8
	}
	return score
}
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
func dedupeAndLimitCandidates(in []discoveryCandidate, limit int) []discoveryCandidate {
	seen := map[string]bool{}
	out := []discoveryCandidate{}
	for _, c := range in {
		u, err := url.Parse(c.URL)
		if err != nil || u.Host == "" {
			continue
		}
		key := strings.ToLower(u.Hostname()) + u.EscapedPath()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
		if len(out) >= limit {
			break
		}
	}
	return out
}
func decodeDuckDuckGoURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil {
		if v := u.Query().Get("uddg"); v != "" {
			return v
		}
	}
	return raw
}
func (h *DiscoveryHandler) Probe(w http.ResponseWriter, r *http.Request) {
	var in discoveryProbeRequest
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		WriteError(w, h.Logger, 400, ErrorPayload{Code: "INVALID_REQUEST", Message: "请求体不是合法 JSON"})
		return
	}
	base := strings.TrimRight(strings.TrimSpace(in.URL), "/")
	if strings.HasSuffix(base, "/api") {
		base = strings.TrimSuffix(base, "/api")
	}
	if base == "" {
		WriteError(w, h.Logger, 400, ErrorPayload{Code: "INVALID_REQUEST", Message: "候选 URL 不能为空"})
		return
	}
	c := h.IndexerClient
	if c == nil {
		c = indexer.NewClient()
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	resp, err := c.Get(ctx, base+"/api?t=caps")
	if err != nil {
		WriteError(w, h.Logger, 400, ErrorPayload{Code: "NOT_TORZNAB", Message: "未发现可用 Torznab 接口"})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !strings.Contains(strings.ToLower(string(body)), "caps") {
		WriteError(w, h.Logger, 400, ErrorPayload{Code: "NOT_TORZNAB", Message: "该站点未返回 Torznab 能力声明"})
		return
	}
	WriteJSON(w, 200, map[string]any{"baseUrl": base})
}
