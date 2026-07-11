package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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
}
type discoverySearchRequest struct {
	Query string `json:"query"`
}
type discoveryCandidate struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Summary string `json:"summary,omitempty"`
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
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, h.searchURL(strings.TrimSpace(in.Query)), nil)
	req.Header.Set("User-Agent", "EasySearch/0.1 local discovery")
	resp, err := h.httpClient().Do(req)
	if err != nil {
		WriteError(w, h.Logger, 502, ErrorPayload{Code: "DISCOVERY_UNAVAILABLE", Message: "无法连接公开搜索服务"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		WriteError(w, h.Logger, 502, ErrorPayload{Code: "DISCOVERY_UNAVAILABLE", Message: "公开搜索服务暂不可用"})
		return
	}
	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		WriteError(w, h.Logger, 502, ErrorPayload{Code: "DISCOVERY_ERROR", Message: "无法解析搜索结果"})
		return
	}
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
		out = append(out, discoveryCandidate{Name: strings.TrimSpace(a.Text()), URL: u.String(), Summary: strings.TrimSpace(s.Find(".result__snippet").Text())})
		return len(out) < 10
	})
	WriteJSON(w, 200, map[string]any{"items": out})
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
