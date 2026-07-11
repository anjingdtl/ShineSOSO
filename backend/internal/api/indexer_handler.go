package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/local/easysearch/backend/internal/catalog"
	"github.com/local/easysearch/backend/internal/indexer"
	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/security"
	"github.com/local/easysearch/backend/internal/store"
)

// IndexerHandler hosts the /api/v1/indexers endpoints (spec §19.4-§19.8).
type IndexerHandler struct {
	Logger      *slog.Logger
	Catalog     *catalog.Catalog
	Repo        *store.IndexerRepo
	HTTPClient  *indexer.Client
	Definitions []model.IndexerDefinition // served by GET /indexer-catalog
}

// listResponse wraps the array so the JSON shape matches the spec
// (clients can later read pagination metadata).
type listIndexersResponse struct {
	Items []model.InstalledIndexer `json:"items"`
}

// List GET /api/v1/indexers
func (h *IndexerHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.Repo.List()
	if err != nil {
		WriteError(w, h.Logger, http.StatusInternalServerError, ErrorPayload{Code: "INTERNAL_ERROR", Message: "列出索引器失败"})
		return
	}
	if items == nil {
		items = []model.InstalledIndexer{}
	}
	WriteJSON(w, http.StatusOK, listIndexersResponse{Items: items})
}

type createIndexerRequest struct {
	DefinitionID string `json:"definitionId"`
	BaseURL      string `json:"baseUrl"`
	Name         string `json:"name,omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`
	TestBefore   *bool  `json:"testBeforeEnable,omitempty"`
}

// Create POST /api/v1/indexers
func (h *IndexerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createIndexerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "请求体不是合法 JSON"})
		return
	}
	if req.DefinitionID == "" {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "definitionId 必填"})
		return
	}
	def, ok := h.Catalog.GetDefinition(req.DefinitionID)
	if !ok {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "definitionId 不在已知目录中"})
		return
	}
	if req.BaseURL == "" {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "baseUrl 必填"})
		return
	}
	{
		v := security.DefaultValidator{}
		if _, err := v.ValidateURL(req.BaseURL); err != nil {
			WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "UNSAFE_INDEXER_URL", Message: "baseUrl 被 SSRF 策略拒绝: " + err.Error()})
			return
		}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.TestBefore != nil && *req.TestBefore {
		// Probe before persisting. We run Test against the factory-built
		// adapter; failure aborts the create.
		inst, _, err := h.buildAdapter(def, model.InstalledIndexer{BaseURL: req.BaseURL, Name: def.Name})
		if err != nil {
			WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: err.Error()})
			return
		}
		res := inst.Test(r.Context())
		if !res.OK {
			WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INDEXER_TEST_FAILED", Message: "测试未通过: " + res.ErrorMessage})
			return
		}
	}

	now := time.Now().UTC()
	name := def.Name
	if strings.TrimSpace(req.Name) != "" {
		name = strings.TrimSpace(req.Name)
	}
	installed := model.InstalledIndexer{
		ID:                uuid.NewString(),
		DefinitionID:      def.ID,
		Name:              name,
		Enabled:           enabled,
		BaseURL:           req.BaseURL,
		DefinitionVersion: def.Version,
		Status:            "unknown",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := h.Repo.Create(installed); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			WriteError(w, h.Logger, http.StatusConflict, ErrorPayload{Code: "DUPLICATE_NAME", Message: "已存在同名索引器"})
			return
		}
		WriteError(w, h.Logger, http.StatusInternalServerError, ErrorPayload{Code: "INTERNAL_ERROR", Message: "保存索引器失败"})
		return
	}
	if err := h.Catalog.Refresh(); err != nil {
		h.Logger.Warn("catalog refresh after create", "err", err)
	}
	WriteJSON(w, http.StatusCreated, installed)
}

// Get GET /api/v1/indexers/{id}
func (h *IndexerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inst, err := h.Repo.Get(id)
	if err != nil {
		WriteError(w, h.Logger, http.StatusNotFound, ErrorPayload{Code: "INDEXER_NOT_FOUND", Message: "索引器不存在"})
		return
	}
	WriteJSON(w, http.StatusOK, inst)
}

type updateIndexerRequest struct {
	Enabled *bool   `json:"enabled,omitempty"`
	BaseURL *string `json:"baseUrl,omitempty"`
	Name    *string `json:"name,omitempty"`
}

// Update PATCH /api/v1/indexers/{id}
func (h *IndexerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req updateIndexerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "请求体不是合法 JSON"})
		return
	}
	inst, err := h.Repo.Get(id)
	if err != nil {
		WriteError(w, h.Logger, http.StatusNotFound, ErrorPayload{Code: "INDEXER_NOT_FOUND", Message: "索引器不存在"})
		return
	}
	if req.Enabled != nil {
		inst.Enabled = *req.Enabled
	}
	if req.BaseURL != nil {
		v := security.DefaultValidator{}
		if _, err := v.ValidateURL(*req.BaseURL); err != nil {
			WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "UNSAFE_INDEXER_URL", Message: "baseUrl 被 SSRF 策略拒绝"})
			return
		}
		inst.BaseURL = *req.BaseURL
	}
	if req.Name != nil {
		inst.Name = *req.Name
	}
	if err := h.Repo.Update(inst); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			WriteError(w, h.Logger, http.StatusConflict, ErrorPayload{Code: "DUPLICATE_NAME", Message: "已存在同名索引器"})
			return
		}
		WriteError(w, h.Logger, http.StatusInternalServerError, ErrorPayload{Code: "INTERNAL_ERROR", Message: "更新失败"})
		return
	}
	if err := h.Catalog.Refresh(); err != nil {
		h.Logger.Warn("catalog refresh after update", "err", err)
	}
	out, _ := h.Repo.Get(id)
	WriteJSON(w, http.StatusOK, out)
}

// Delete DELETE /api/v1/indexers/{id}
func (h *IndexerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Repo.Delete(id); err != nil {
		WriteError(w, h.Logger, http.StatusNotFound, ErrorPayload{Code: "INDEXER_NOT_FOUND", Message: "索引器不存在"})
		return
	}
	if err := h.Catalog.Refresh(); err != nil {
		h.Logger.Warn("catalog refresh after delete", "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

type testIndexerResponse struct {
	OK           bool   `json:"ok"`
	StatusCode   int    `json:"statusCode,omitempty"`
	DurationMs   int64  `json:"durationMs"`
	ResultCount  int    `json:"resultCount,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// Test POST /api/v1/indexers/{id}/test
func (h *IndexerHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inst, err := h.Repo.Get(id)
	if err != nil {
		WriteError(w, h.Logger, http.StatusNotFound, ErrorPayload{Code: "INDEXER_NOT_FOUND", Message: "索引器不存在"})
		return
	}
	def, ok := h.Catalog.GetDefinition(inst.DefinitionID)
	if !ok {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "索引器的 definitionId 不在已知目录中"})
		return
	}
	adapter, _, err := h.buildAdapter(def, inst)
	if err != nil {
		WriteError(w, h.Logger, http.StatusInternalServerError, ErrorPayload{Code: "INTERNAL_ERROR", Message: err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	res := adapter.Test(ctx)

	now := time.Now().UTC()
	status := "error"
	if res.OK {
		status = "healthy"
	}
	lastErr := ""
	if !res.OK {
		lastErr = res.ErrorMessage
	}
	_ = h.Repo.SetStatus(inst.ID, status, &now, &lastErr, res.DurationMs)
	_ = h.Repo.RecordHealthEvent(inst.ID, status, res.DurationMs, res.ErrorCode, res.ErrorMessage)

	WriteJSON(w, http.StatusOK, testIndexerResponse{
		OK:           res.OK,
		StatusCode:   res.StatusCode,
		DurationMs:   res.DurationMs,
		ResultCount:  res.ResultCount,
		ErrorCode:    res.ErrorCode,
		ErrorMessage: res.ErrorMessage,
	})
}

type listCatalogResponse struct {
	Items []model.IndexerDefinition `json:"items"`
}

// ListCatalog GET /api/v1/indexer-catalog
func (h *IndexerHandler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	items := h.Catalog.Definitions()
	if items == nil {
		items = []model.IndexerDefinition{}
	}
	WriteJSON(w, http.StatusOK, listCatalogResponse{Items: items})
}

func (h *IndexerHandler) buildAdapter(def model.IndexerDefinition, inst model.InstalledIndexer) (indexer.IndexerAdapter, *indexer.Client, error) {
	factory, err := indexer.Default.Get(indexer.Protocol(def.Protocol))
	if err != nil {
		return nil, nil, errors.New("协议不支持: " + def.Protocol)
	}
	a, err := factory.Create(def, inst, h.HTTPClient)
	if err != nil {
		return nil, nil, err
	}
	return a, h.HTTPClient, nil
}
