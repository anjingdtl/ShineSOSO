// Package prowlarr manages the Prowlarr runtime shipped alongside EasySearch.
// It deliberately only talks to its own loopback instance; user supplied
// indexer URLs still go through the hardened indexer client.
package prowlarr

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/local/easysearch/backend/internal/indexer"
	"github.com/local/easysearch/backend/internal/model"
)

const defaultBaseURL = "http://127.0.0.1:9696"

type Config struct {
	Executable string
	DataDir    string
	BaseURL    string // tests may point this at an httptest server
	HTTPClient *http.Client
}

type Status struct {
	State   string `json:"state"`
	Version string `json:"version,omitempty"`
	Message string `json:"message,omitempty"`
}

type Candidate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Privacy     string `json:"privacy,omitempty"`
	CanQuickAdd bool   `json:"canQuickAdd"`
	Reason      string `json:"reason,omitempty"`
}

// InstalledIndexer is an indexer that lives in the managed Prowlarr
// database. It is intentionally distinct from EasySearch's legacy local
// definition records, but the UI presents both in one installed area.
type InstalledIndexer struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Enabled  bool     `json:"enabled"`
	Protocol string   `json:"protocol,omitempty"`
	Privacy  string   `json:"privacy,omitempty"`
	URLs     []string `json:"urls,omitempty"`
}

type Manager struct {
	mu         sync.RWMutex
	executable string
	dataDir    string
	baseURL    string
	http       *http.Client
	apiKey     string
	status     Status
	process    *exec.Cmd
	starting   bool
}

func NewManager(cfg Config) *Manager {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = defaultBaseURL
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &Manager{executable: cfg.Executable, dataDir: cfg.DataDir, baseURL: base, http: client, status: Status{State: "unavailable", Message: "未找到随 EasySearch 分发的 Prowlarr 运行时"}}
}

// StartAsync never blocks EasySearch's UI server. Status reports startup
// progress and operations return a clear retryable error until it is ready.
func (m *Manager) StartAsync(ctx context.Context) {
	m.mu.Lock()
	if m.starting || m.status.State == "ready" {
		m.mu.Unlock()
		return
	}
	if m.executable == "" {
		m.mu.Unlock()
		return
	}
	if _, err := os.Stat(m.executable); err != nil {
		m.status = Status{State: "unavailable", Message: "Prowlarr 运行时不在发布包中，请使用 EasySearch 便携版"}
		m.mu.Unlock()
		return
	}
	m.starting = true
	m.status = Status{State: "starting", Message: "正在启动内置 Prowlarr 引擎"}
	m.mu.Unlock()
	go m.start(ctx)
}

func (m *Manager) start(ctx context.Context) {
	if err := os.MkdirAll(m.dataDir, 0o755); err != nil {
		m.setFailure(fmt.Errorf("创建 Prowlarr 数据目录: %w", err))
		return
	}
	if key, base, ok := m.readConfig(); ok && m.ping(ctx, base, key) == nil {
		m.setReady(base, key)
		return
	}

	logFile, err := os.OpenFile(filepath.Join(m.dataDir, "easysearch-runtime.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		m.setFailure(fmt.Errorf("打开 Prowlarr 日志: %w", err))
		return
	}
	cmd := exec.Command(m.executable, "-nobrowser", "-data="+m.dataDir)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		m.setFailure(fmt.Errorf("启动 Prowlarr: %w", err))
		return
	}
	m.mu.Lock()
	m.process = cmd
	m.mu.Unlock()
	go func() { _ = cmd.Wait(); _ = logFile.Close() }()

	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		if key, base, ok := m.readConfig(); ok && m.ping(ctx, base, key) == nil {
			m.setReady(base, key)
			return
		}
		select {
		case <-ctx.Done():
			m.setFailure(ctx.Err())
			return
		case <-deadline.C:
			m.setFailure(fmt.Errorf("Prowlarr 在 90 秒内没有就绪，请查看 %s", filepath.Join(m.dataDir, "easysearch-runtime.log")))
			return
		case <-tick.C:
		}
	}
}

func (m *Manager) readConfig() (key, base string, ok bool) {
	b, err := os.ReadFile(filepath.Join(m.dataDir, "config.xml"))
	if err != nil {
		return "", "", false
	}
	var cfg struct {
		Port    string `xml:"Port"`
		ApiKey  string `xml:"ApiKey"`
		URLBase string `xml:"UrlBase"`
	}
	if xml.Unmarshal(b, &cfg) != nil || strings.TrimSpace(cfg.ApiKey) == "" {
		return "", "", false
	}
	port := strings.TrimSpace(cfg.Port)
	if port == "" {
		port = "9696"
	}
	base = "http://127.0.0.1:" + port
	if p := strings.Trim(strings.TrimSpace(cfg.URLBase), "/"); p != "" {
		base += "/" + p
	}
	return strings.TrimSpace(cfg.ApiKey), base, true
}

func (m *Manager) ping(ctx context.Context, base, key string) error {
	if key == "" {
		return fmt.Errorf("Prowlarr API Key 尚未生成")
	}
	var system struct {
		Version string `json:"version"`
	}
	return m.request(ctx, http.MethodGet, base, key, "/api/v1/system/status", nil, &system)
}

func (m *Manager) setReady(base, key string) {
	var system struct {
		Version string `json:"version"`
	}
	_ = m.request(context.Background(), http.MethodGet, base, key, "/api/v1/system/status", nil, &system)
	m.mu.Lock()
	m.baseURL, m.apiKey = base, key
	m.status = Status{State: "ready", Version: system.Version, Message: "内置 Prowlarr 引擎已就绪"}
	m.starting = false
	m.mu.Unlock()
}

func (m *Manager) setFailure(err error) {
	m.mu.Lock()
	m.status = Status{State: "error", Message: err.Error()}
	m.starting = false
	m.mu.Unlock()
}

func (m *Manager) Status() Status { m.mu.RLock(); defer m.mu.RUnlock(); return m.status }
func (m *Manager) Ready() bool    { return m.Status().State == "ready" }

func (m *Manager) Close() {
	m.mu.Lock()
	cmd := m.process
	m.process = nil
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (m *Manager) request(ctx context.Context, method, base, key, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// The local schema response contains hundreds of Cardigann definitions and
	// legitimately exceeds 2 MiB. This client only reaches our own loopback
	// companion process, so a bounded 20 MiB cap is appropriate here.
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(b))
		if len(message) > 500 {
			message = message[:500]
		}
		return fmt.Errorf("Prowlarr API 返回 %d: %s", resp.StatusCode, message)
	}
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("解析 Prowlarr 响应: %w", err)
		}
	}
	return nil
}

func (m *Manager) schema(ctx context.Context) ([]map[string]any, error) {
	m.mu.RLock()
	base, key := m.baseURL, m.apiKey
	ready := m.status.State == "ready"
	m.mu.RUnlock()
	if !ready {
		return nil, fmt.Errorf("Prowlarr 引擎尚未就绪")
	}
	var schemas []map[string]any
	if err := m.request(ctx, http.MethodGet, base, key, "/api/v1/indexer/schema", nil, &schemas); err != nil {
		return nil, err
	}
	return schemas, nil
}

func (m *Manager) Discover(ctx context.Context, query string) ([]Candidate, error) {
	schemas, err := m.schema(ctx)
	if err != nil {
		return nil, err
	}
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	out := make([]Candidate, 0, len(schemas))
	for _, schema := range schemas {
		name, _ := schema["name"].(string)
		id := schemaIdentifier(schema)
		privacy, _ := schema["privacy"].(string)
		protocol, _ := schema["protocol"].(string)
		if id == "" || name == "" || !matches(name+" "+id+" "+privacy+" "+protocol, terms) {
			continue
		}
		candidate := Candidate{ID: id, Name: name, Privacy: privacy, Protocol: protocol, CanQuickAdd: strings.EqualFold(privacy, "public")}
		if candidate.CanQuickAdd && needsInput(schema) {
			candidate.CanQuickAdd, candidate.Reason = false, "需要账号、验证码或站点专用配置"
		}
		if !candidate.CanQuickAdd && candidate.Reason == "" {
			candidate.Reason = "不是公开免配置索引器"
		}
		out = append(out, candidate)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CanQuickAdd != out[j].CanQuickAdd {
			return out[i].CanQuickAdd
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if len(out) > 60 {
		out = out[:60]
	}
	return out, nil
}

func (m *Manager) ListInstalled(ctx context.Context) ([]InstalledIndexer, error) {
	m.mu.RLock()
	base, key, ready := m.baseURL, m.apiKey, m.status.State == "ready"
	m.mu.RUnlock()
	if !ready {
		return nil, fmt.Errorf("Prowlarr 引擎尚未就绪")
	}
	var raw []struct {
		ID          int64    `json:"id"`
		Name        string   `json:"name"`
		Enable      bool     `json:"enable"`
		Protocol    string   `json:"protocol"`
		Privacy     string   `json:"privacy"`
		IndexerURLs []string `json:"indexerUrls"`
	}
	if err := m.request(ctx, http.MethodGet, base, key, "/api/v1/indexer", nil, &raw); err != nil {
		return nil, err
	}
	out := make([]InstalledIndexer, 0, len(raw))
	for _, item := range raw {
		out = append(out, InstalledIndexer{ID: item.ID, Name: item.Name, Enabled: item.Enable, Protocol: item.Protocol, Privacy: item.Privacy, URLs: item.IndexerURLs})
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func matches(value string, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	v := strings.ToLower(value)
	for _, term := range terms {
		if !strings.Contains(v, term) {
			return false
		}
	}
	return true
}

// Schema resources are templates, not installed indexers, so Prowlarr's
// numeric `id` is normally omitted (zero). `implementation` is the stable
// template identifier used by its own API and is unique in the schema list.
func schemaIdentifier(schema map[string]any) string {
	for _, key := range []string{"definitionName", "implementation", "id"} {
		if value, ok := schema[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func needsInput(schema map[string]any) bool {
	fields, _ := schema["fields"].([]any)
	for _, raw := range fields {
		field, _ := raw.(map[string]any)
		if field == nil {
			continue
		}
		if advanced, _ := field["advanced"].(bool); advanced {
			continue // optional runtime tuning, not account setup
		}
		name, _ := field["name"].(string)
		typ, _ := field["type"].(string)
		if name == "baseUrl" || name == "tags" || name == "downloadClientId" || name == "priority" {
			continue
		}
		lowerName := strings.ToLower(name)
		if strings.Contains(lowerName, "user") || strings.Contains(lowerName, "pass") || strings.Contains(lowerName, "apikey") || strings.Contains(lowerName, "api_key") || strings.Contains(lowerName, "cookie") || strings.Contains(lowerName, "token") || strings.Contains(lowerName, "captcha") {
			return true
		}
		if value, exists := field["value"]; exists && value != nil && fmt.Sprint(value) != "" {
			continue
		}
		if strings.EqualFold(typ, "checkbox") || strings.EqualFold(typ, "select") || strings.EqualFold(typ, "number") {
			continue
		}
		return true
	}
	return false
}

func (m *Manager) AddAndTest(ctx context.Context, schemaID string) (Candidate, error) {
	schemas, err := m.schema(ctx)
	if err != nil {
		return Candidate{}, err
	}
	for _, schema := range schemas {
		id := schemaIdentifier(schema)
		if id != schemaID {
			continue
		}
		if privacy, _ := schema["privacy"].(string); !strings.EqualFold(privacy, "public") || needsInput(schema) {
			return Candidate{}, fmt.Errorf("该索引器需要额外配置，不能一键添加")
		}
		profileID, err := m.appProfileID(ctx)
		if err != nil {
			return Candidate{}, err
		}
		schema["appProfileId"] = profileID
		schema["enable"] = true
		m.mu.RLock()
		base, key := m.baseURL, m.apiKey
		m.mu.RUnlock()
		if err := m.request(ctx, http.MethodPost, base, key, "/api/v1/indexer/test", schema, nil); err != nil {
			return Candidate{}, fmt.Errorf("Prowlarr 测试未通过: %w", err)
		}
		if err := m.request(ctx, http.MethodPost, base, key, "/api/v1/indexer", schema, nil); err != nil {
			return Candidate{}, fmt.Errorf("Prowlarr 添加失败: %w", err)
		}
		name, _ := schema["name"].(string)
		protocol, _ := schema["protocol"].(string)
		return Candidate{ID: id, Name: name, Privacy: "public", Protocol: protocol, CanQuickAdd: true}, nil
	}
	return Candidate{}, fmt.Errorf("未找到索引器定义 %q", schemaID)
}

func (m *Manager) appProfileID(ctx context.Context) (int, error) {
	m.mu.RLock()
	base, key := m.baseURL, m.apiKey
	m.mu.RUnlock()
	var profiles []struct {
		ID int `json:"id"`
	}
	if err := m.request(ctx, http.MethodGet, base, key, "/api/v1/appprofile", nil, &profiles); err != nil {
		return 0, fmt.Errorf("读取 Prowlarr 应用配置: %w", err)
	}
	if len(profiles) == 0 || profiles[0].ID <= 0 {
		return 0, fmt.Errorf("Prowlarr 没有可用的应用配置")
	}
	return profiles[0].ID, nil
}

// Search satisfies EasySearch's adapter contract while Prowlarr handles all
// Cardigann parsing and tracker-specific behavior behind its local API.
func (m *Manager) ID() string { return "managed-prowlarr" }
func (m *Manager) Test(ctx context.Context) indexer.TestResult {
	started := time.Now()
	err := m.pingReady(ctx)
	result := indexer.TestResult{OK: err == nil, DurationMs: time.Since(started).Milliseconds()}
	if err != nil {
		result.ErrorMessage = err.Error()
	}
	return result
}
func (m *Manager) pingReady(ctx context.Context) error {
	m.mu.RLock()
	base, key, ready := m.baseURL, m.apiKey, m.status.State == "ready"
	m.mu.RUnlock()
	if !ready {
		return fmt.Errorf("Prowlarr 引擎尚未就绪")
	}
	return m.ping(ctx, base, key)
}

func (m *Manager) Search(ctx context.Context, query model.SearchQuery) ([]model.SearchResult, error) {
	m.mu.RLock()
	base, key, ready := m.baseURL, m.apiKey, m.status.State == "ready"
	m.mu.RUnlock()
	if !ready {
		return nil, fmt.Errorf("Prowlarr 引擎尚未就绪")
	}
	path := "/api/v1/search?query=" + url.QueryEscape(query.Keyword) + "&limit=100"
	var releases []struct {
		Guid        string     `json:"guid"`
		Title       string     `json:"title"`
		Size        *int64     `json:"size"`
		Indexer     string     `json:"indexer"`
		PublishDate *time.Time `json:"publishDate"`
		DownloadURL string     `json:"downloadUrl"`
		InfoURL     string     `json:"infoUrl"`
		MagnetURL   string     `json:"magnetUrl"`
		InfoHash    string     `json:"infoHash"`
		Seeders     *int       `json:"seeders"`
		Leechers    *int       `json:"leechers"`
		Grabs       *int       `json:"grabs"`
	}
	if err := m.request(ctx, http.MethodGet, base, key, path, nil, &releases); err != nil {
		return nil, err
	}
	out := make([]model.SearchResult, 0, len(releases))
	for i, r := range releases {
		id := r.Guid
		if id == "" {
			id = fmt.Sprintf("prowlarr-%d", i)
		}
		out = append(out, model.SearchResult{ID: id, Title: r.Title, Category: query.Category, SizeBytes: r.Size, Seeders: r.Seeders, Leechers: r.Leechers, Downloads: r.Grabs, PublishedAt: r.PublishDate, MagnetURL: r.MagnetURL, TorrentURL: r.DownloadURL, DetailURL: r.InfoURL, InfoHash: r.InfoHash, IndexerID: m.ID(), IndexerName: r.Indexer})
	}
	return out, nil
}
