package store

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrConflict is returned when a write violates a unique constraint.
var ErrConflict = errors.New("conflict")

// classifyErr maps low-level driver errors to sentinel errors the service
// layer can switch on, without leaking SQL/driver detail to callers.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate key") {
		return fmt.Errorf("%w: %s", ErrConflict, err.Error())
	}
	return err
}

// ErrInvalidCursor is returned for a cursor this server did not issue; the
// API maps it to 400 rather than a 500.
var ErrInvalidCursor = errors.New("invalid cursor")

// encodeCursor/decodeCursor implement opaque cursor pagination on
// (sort_value_ms, id) — stable under concurrent inserts, unlike offsets.
func encodeCursor(ms int64, id string) string {
	return encodeKeyCursor(strconv.FormatInt(ms, 10), id)
}

func decodeCursor(cursor string) (int64, string, error) {
	v, id, err := decodeKeyCursor(cursor)
	if err != nil {
		return 0, "", err
	}
	ms, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, "", ErrInvalidCursor
	}
	return ms, id, nil
}

// encodeKeyCursor/decodeKeyCursor carry an arbitrary sort value (e.g. a
// title). The id never contains "|", so the last one is the separator.
func encodeKeyCursor(val, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(val + "|" + id))
}

func decodeKeyCursor(cursor string) (string, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", ErrInvalidCursor
	}
	i := strings.LastIndexByte(string(b), '|')
	if i < 0 {
		return "", "", ErrInvalidCursor
	}
	return string(b[:i]), string(b[i+1:]), nil
}
