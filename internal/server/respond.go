package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"shortr/internal/store"
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// respondError maps an error to the PLAN.md §17 envelope. Unknown errors
// (not *APIError, not a known store sentinel) become a generic 500 — no
// internal detail (SQL, paths, stack traces) ever reaches the client; the
// full error is logged server-side with the request id for correlation.
func respondError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := requestIDFromContext(r.Context())
	ae, ok := AsAPIError(err)
	if !ok {
		switch {
		case errors.Is(err, store.ErrNotFound):
			ae = ErrNotFound
		case errors.Is(err, store.ErrConflict):
			ae = NewAPIError(http.StatusConflict, "CONFLICT", "resource already exists")
		case errors.Is(err, context.DeadlineExceeded):
			ae = ErrDBUnavailable
		default:
			slog.Error("unhandled error", "request_id", reqID, "error", err, "path", r.URL.Path)
			ae = ErrInternal
		}
	}
	if ae.Status >= 500 {
		slog.Error("request failed", "request_id", reqID, "code", ae.Code, "error", err, "path", r.URL.Path)
	}
	writeJSON(w, ae.Status, errorEnvelope{Error: errorBody{Code: ae.Code, Message: ae.Message, Fields: ae.Fields, RequestID: reqID}})
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	writeJSON(w, status, v)
}

type listEnvelope[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

func respondList[T any](w http.ResponseWriter, items []T, nextCursor string) {
	if items == nil {
		items = []T{}
	}
	writeJSON(w, http.StatusOK, listEnvelope[T]{Items: items, NextCursor: nextCursor})
}

// decodeJSON reads and validates a JSON body: size-limited, rejects unknown
// fields (PLAN.md §13.3 / §16 "JSON bodies").
func decodeJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return ErrPayloadTooLarge
		}
		return NewAPIError(http.StatusBadRequest, "BAD_REQUEST", "malformed JSON: "+err.Error())
	}
	return nil
}
