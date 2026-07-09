// Package api exposes the HTTP surface of EasySearch. All handlers share a
// uniform JSON error envelope (see ErrorBody) and a context-bound logger.
package api

import (
    "encoding/json"
    "log/slog"
    "net/http"
)

// ErrorBody is the JSON shape of every non-2xx response (spec §22.1).
type ErrorBody struct {
    Error ErrorPayload `json:"error"`
}

// ErrorPayload carries a machine code and a Chinese-localized human message.
// Details is optional; the server may include extra context.
type ErrorPayload struct {
    Code    string         `json:"code"`
    Message string         `json:"message"`
    Details map[string]any `json:"details,omitempty"`
}

// WriteError writes a JSON error envelope and sets the status code.
// The logger is optional; when nil, the response carries no server log.
func WriteError(w http.ResponseWriter, logger *slog.Logger, status int, p ErrorPayload) {
    if logger != nil {
        logger.Warn("api error",
            "status", status,
            "code", p.Code,
            "message", p.Message,
        )
    }
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(ErrorBody{Error: p})
}

// WriteJSON writes a 2xx JSON body.
func WriteJSON(w http.ResponseWriter, status int, body any) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(body)
}
