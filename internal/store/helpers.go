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

// encodeCursor/decodeCursor implement opaque cursor pagination on
// (sort_value_ms, id) — stable under concurrent inserts, unlike offsets.
func encodeCursor(ms int64, id string) string {
	raw := fmt.Sprintf("%d|%s", ms, id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (int64, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", fmt.Errorf("invalid cursor")
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid cursor")
	}
	ms, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid cursor")
	}
	return ms, parts[1], nil
}
