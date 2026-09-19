// Package server wires config, store, and all services into the HTTP API,
// the redirect hot path, server-rendered public pages, and the embedded SPA.
package server

import (
	"errors"
	"net/http"

	"shortr/internal/validate"
)

// APIError is a typed error handlers can return; respondError maps it to the
// PLAN.md §17 error-code table. A plain (non-*APIError) error becomes a
// generic 500 INTERNAL — nothing about it is ever shown to the client.
type APIError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
}

func (e *APIError) Error() string { return e.Message }

func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

func ValidationFailed(errs validate.Errors) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "VALIDATION_FAILED", Message: "validation failed", Fields: errs.Map()}
}

var (
	ErrBadRequest       = NewAPIError(http.StatusBadRequest, "BAD_REQUEST", "malformed request")
	ErrUnauthenticated  = NewAPIError(http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
	ErrSessionExpired   = NewAPIError(http.StatusUnauthorized, "SESSION_EXPIRED", "session expired, please sign in again")
	ErrForbidden        = NewAPIError(http.StatusForbidden, "FORBIDDEN", "you do not have permission to do this")
	ErrCSRFFailed       = NewAPIError(http.StatusForbidden, "CSRF_FAILED", "csrf validation failed")
	ErrSudoRequired     = NewAPIError(http.StatusForbidden, "SUDO_REQUIRED", "please re-enter your password to continue")
	ErrUserDisabled     = NewAPIError(http.StatusForbidden, "USER_DISABLED", "this account has been disabled")
	ErrNotFound         = NewAPIError(http.StatusNotFound, "NOT_FOUND", "not found")
	ErrCodeTaken        = NewAPIError(http.StatusConflict, "CODE_TAKEN", "this alias is already taken")
	ErrEmailTaken       = NewAPIError(http.StatusConflict, "EMAIL_TAKEN", "an account with this email already exists")
	ErrSetupDone        = NewAPIError(http.StatusConflict, "SETUP_DONE", "setup has already been completed")
	ErrLastAdmin        = NewAPIError(http.StatusConflict, "LAST_ADMIN", "cannot remove the last remaining admin")
	ErrCannotUnlink     = NewAPIError(http.StatusConflict, "CANNOT_UNLINK", "cannot unlink your only sign-in method")
	ErrLinkGone         = NewAPIError(http.StatusGone, "LINK_GONE", "this link has been removed or expired")
	ErrPreconditionFail = NewAPIError(http.StatusPreconditionFailed, "PRECONDITION_FAILED", "resource has changed, please refresh")
	ErrPayloadTooLarge  = NewAPIError(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body too large")
	ErrTargetBlocked    = NewAPIError(http.StatusUnprocessableEntity, "TARGET_BLOCKED", "this target URL is not allowed")
	ErrLinkLimitReached = NewAPIError(http.StatusUnprocessableEntity, "LINK_LIMIT_REACHED", "you have reached your link limit")
	ErrRateLimited      = NewAPIError(http.StatusTooManyRequests, "RATE_LIMITED", "too many requests, please slow down")
	ErrLockedOut        = NewAPIError(http.StatusTooManyRequests, "LOCKED_OUT", "too many failed attempts, try again later")
	ErrInternal         = NewAPIError(http.StatusInternalServerError, "INTERNAL", "something went wrong")
	ErrDBUnavailable    = NewAPIError(http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database temporarily unavailable")
	ErrCodeExhausted    = NewAPIError(http.StatusServiceUnavailable, "CODE_EXHAUSTED", "could not generate a unique code, please try a custom alias")
	ErrShuttingDown     = NewAPIError(http.StatusServiceUnavailable, "SHUTTING_DOWN", "server is shutting down")

	ErrOIDCAlreadyLinked   = NewAPIError(http.StatusConflict, "OIDC_ALREADY_LINKED", "this SSO identity is already linked to another account")
	ErrOIDCAccountExists   = NewAPIError(http.StatusConflict, "OIDC_ACCOUNT_EXISTS", "an account with this email already exists; sign in with your password, then link SSO from Settings")
	ErrOIDCNoAccount       = NewAPIError(http.StatusConflict, "OIDC_NO_ACCOUNT", "no account found for you; ask an admin to invite you")
	ErrOIDCEmailUnverified = NewAPIError(http.StatusConflict, "OIDC_EMAIL_UNVERIFIED", "your identity provider has not verified your email")
	ErrOIDCDomainNotAllowed = NewAPIError(http.StatusConflict, "OIDC_DOMAIN_NOT_ALLOWED", "your email domain is not allowed to sign in")
)

// AsAPIError extracts an *APIError if err is (or wraps) one.
func AsAPIError(err error) (*APIError, bool) {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
