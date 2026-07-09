package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/local/easysearch/backend/internal/catalog"
	"github.com/local/easysearch/backend/internal/indexer"
	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/store"
)

// ImportHandler hosts POST /api/v1/indexer-catalog/import (spec §19.10,
// Phase 5 deliverable: validate a user-supplied YAML, run a probe,
// persist to imported_definitions, return an import decision).
//
// The endpoint intentionally stops short of *enabling* the indexer;
// the user must call POST /api/v1/indexers afterwards with the new
// definition + baseUrl to actually install it.
type ImportHandler struct {
	Logger    *slog.Logger
	Repo      *store.ImportedDefinitionRepo
	Catalog   *catalog.Catalog
	HTTPClient *indexer.Client
}

type importRequest struct {
	YAML     string `json:"yaml"`
	Filename string `json:"filename,omitempty"`
	Test     bool   `json:"test,omitempty"` // when true, runs Test() and adds duration/error to the response
}

// ImportResponse is the structured envelope the UI renders as the
// "import decision" panel.
type ImportResponse struct {
	ID           string                   `json:"id"`
	Valid        bool                     `json:"valid"`
	Errors       []ImportValidationError  `json:"errors,omitempty"`
	Definition   *model.IndexerDefinition `json:"definition,omitempty"`
	Installed    bool                     `json:"installed"`            // true if a matching installed_indexer already exists
	InstalledID  string                   `json:"installedId,omitempty"`
	Test         *indexer.TestResult      `json:"test,omitempty"`
	Persisted    bool                     `json:"persisted"` // false in dry-run mode (when Validation fails)
}

// ImportValidationError is one problem with the YAML; multiple may
// surface in the UI but only the first blocks persistence.
type ImportValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Import POST /api/v1/indexer-catalog/import
//
// Accepts a JSON body {yaml, filename?} and:
//  1. parses & validates against spec §13.8
//  2. runs an optional Test() when the definition is supported
//  3. persists to imported_definitions unless validation failed
//
// On success the caller should follow up with POST /api/v1/indexers
// to actually add the indexer to their enabled list.
func (h *ImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "请求体不是合法 JSON"})
		return
	}
	yaml := strings.TrimSpace(req.YAML)
	if yaml == "" {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "yaml 字段必填"})
		return
	}
	if req.Filename == "" {
		req.Filename = "uploaded.yml"
	}

	// 1. Parse + §13.8 validate
	def, err := catalog.LoadDefinition([]byte(yaml), req.Filename)
	if err != nil {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{
			Code:    mapLoaderError(err),
			Message: err.Error(),
		})
		return
	}

	resp := ImportResponse{
		ID:         def.ID,
		Valid:      true,
		Definition: &def,
	}

	if verr := catalog.Validate(def); verr != nil {
		var ve *catalog.ValidationError
		if errors.As(verr, &ve) {
			resp.Valid = false
			resp.Errors = append(resp.Errors, ImportValidationError{Code: ve.Code, Message: ve.Message})
			// Bail out: don't persist, don't probe.
			WriteJSON(w, http.StatusOK, resp)
			return
		}
		// Non-ValidationError — surface as a generic 400.
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "VALIDATION", Message: verr.Error()})
		return
	}

	// Cross-check: the catalog mustn't already have a definition with
	// the same ID (built-in or otherwise). This protects Phase 6's
	// catalog updater from being silently shadowed.
	if _, dup := h.Catalog.GetDefinition(def.ID); dup {
		resp.Installed = true
		// Don't overwrite; tell the user to delete the existing one first.
		WriteJSON(w, http.StatusOK, resp)
		return
	}

	// 2. Optional probe
	if req.Test {
		// We can only probe adapters the current build understands;
		// skip with a clear note if not.
		if def.Result.Format != "html" {
			resp.Test = &indexer.TestResult{
				OK:           false,
				DurationMs:   0,
				ErrorCode:    "FORMAT_UNSUPPORTED",
				ErrorMessage: "测试仅支持 html 格式；其他格式待后续版本",
			}
		} else {
			installed, _ := h.Catalog.GetDefinition(def.ID)
			_ = installed
			// Build a transient installed indexer pointing at the
			// first link so adapter.Test() can compose a URL.
			base := def.Links[0]
			transient := model.InstalledIndexer{
				ID:    def.ID,
				Name:  def.Name,
				BaseURL: base,
			}
			adapter, aErr := indexer.NewDeclarativeFactory().Create(def, transient, h.HTTPClient)
			if aErr != nil {
				resp.Test = &indexer.TestResult{OK: false, ErrorCode: "ADAPTER_BUILD", ErrorMessage: aErr.Error()}
			} else {
				ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
				defer cancel()
				res := adapter.Test(ctx)
				resp.Test = &res
			}
		}
	}

	// 3. Persist
	imp := model.ImportedDefinition{
		ID:      def.ID,
		Version: def.Version,
		Content: yaml,
	}
	if err := h.Repo.Create(imp); err != nil {
		// ErrDuplicate means someone raced us; treat as success-ish.
		if !errors.Is(err, store.ErrDuplicate) {
			WriteError(w, h.Logger, http.StatusInternalServerError, ErrorPayload{Code: "PERSIST_FAILED", Message: "保存定义失败: " + err.Error()})
			return
		}
	}
	resp.Persisted = true

	// Register the parsed definition into the in-memory catalog so the
	// user can immediately add it via POST /api/v1/indexers.
	h.Catalog.RegisterDefinition(def)

	WriteJSON(w, http.StatusOK, resp)
}

// ListImported GET /api/v1/indexer-catalog/imported lists every
// previously-imported definition so the UI can offer re-install.
func (h *ImportHandler) ListImported(w http.ResponseWriter, r *http.Request) {
	items, err := h.Repo.List()
	if err != nil {
		WriteError(w, h.Logger, http.StatusInternalServerError, ErrorPayload{Code: "INTERNAL_ERROR", Message: "列出已导入定义失败"})
		return
	}
	if items == nil {
		items = []model.ImportedDefinition{}
	}
	WriteJSON(w, http.StatusOK, listImportedResponse{Items: items})
}

type listImportedResponse struct {
	Items []model.ImportedDefinition `json:"items"`
}

// mapLoaderError converts catalog loader sentinels into API error codes.
func mapLoaderError(err error) string {
	switch {
	case errors.Is(err, catalog.ErrDefinitionTooLarge):
		return "DEFINITION_TOO_LARGE"
	case errors.Is(err, catalog.ErrInvalidYAML):
		return "INVALID_YAML"
	default:
		return "INVALID_YAML"
	}
}

// kill the unused io import warning for future readers without forcing
// the handler to ingest the body via ReadAll; json.Decoder streams
// fine. Kept as a hook so the swagger "yaml file upload" branch can be
// added later without re-touching imports.
var _ = io.Discard
var _ = uuid.NewString
