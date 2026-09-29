package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"shortr/internal/auth"
)

func TestMFAEnrollLoginAndRecovery(t *testing.T) {
	env := newTestEnv(t)
	cookies, csrf, _ := adminSetup(t, env, env.srv.handler) // fresh session, so sudo is active

	// enroll
	rec := env.do(t, "POST", "/api/v1/me/mfa/totp/setup", nil, cookies, csrf)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	var setup struct{ Secret, OtpauthUrl string }
	_ = json.Unmarshal(rec.Body.Bytes(), &setup)
	if setup.Secret == "" || setup.OtpauthUrl == "" {
		t.Fatalf("setup body: %s", rec.Body.String())
	}
	if rec := env.do(t, "POST", "/api/v1/me/mfa/totp/enable", map[string]string{"code": "000000"}, cookies, csrf); rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong code accepted: %d", rec.Code)
	}
	now := time.Now()
	code, _ := auth.TOTPCode(setup.Secret, auth.TOTPStep(now))
	rec = env.do(t, "POST", "/api/v1/me/mfa/totp/enable", map[string]string{"code": code}, cookies, csrf)
	var enabled struct{ RecoveryCodes []string }
	_ = json.Unmarshal(rec.Body.Bytes(), &enabled)
	if rec.Code != http.StatusOK || len(enabled.RecoveryCodes) != 10 {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body.String())
	}

	// password alone no longer creates a session
	login := func() string {
		rec := env.do(t, "POST", "/auth/login", map[string]string{"email": "a@example.com", "password": "correcthorsebatterystaple"}, nil, "")
		var out struct {
			MFARequired bool   `json:"mfaRequired"`
			MFAToken    string `json:"mfaToken"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if !out.MFARequired || out.MFAToken == "" || len(rec.Result().Cookies()) != 0 {
			t.Fatalf("login should stop at MFA: %s", rec.Body.String())
		}
		return out.MFAToken
	}

	// the code used for enrollment can't be replayed
	tok := login()
	if rec := env.do(t, "POST", "/auth/login/mfa", map[string]string{"mfaToken": tok, "code": code}, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("replayed code accepted: %d", rec.Code)
	}
	next, _ := auth.TOTPCode(setup.Secret, auth.TOTPStep(now)+1)
	rec = env.do(t, "POST", "/auth/login/mfa", map[string]string{"mfaToken": tok, "code": next}, nil, "")
	if rec.Code != http.StatusOK || len(rec.Result().Cookies()) == 0 {
		t.Fatalf("mfa login: %d %s", rec.Code, rec.Body.String())
	}

	// a recovery code works exactly once
	rc := enabled.RecoveryCodes[0]
	if rec := env.do(t, "POST", "/auth/login/mfa", map[string]string{"mfaToken": login(), "recoveryCode": rc}, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("recovery code: %d %s", rec.Code, rec.Body.String())
	}
	if rec := env.do(t, "POST", "/auth/login/mfa", map[string]string{"mfaToken": login(), "recoveryCode": rc}, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("recovery code reused: %d", rec.Code)
	}
	if rec := env.do(t, "POST", "/auth/login/mfa", map[string]string{"mfaToken": "forged", "code": next}, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged token: %d", rec.Code)
	}
}

func TestMFAPolicyGatesUnenrolledPasswordSessions(t *testing.T) {
	env := newTestEnv(t)
	cookies, csrf, key := adminSetup(t, env, env.srv.handler)
	if rec := env.do(t, "PUT", "/api/v1/settings", map[string]any{"mfaRequired": true}, cookies, csrf); rec.Code != http.StatusOK {
		t.Fatalf("enable policy: %d", rec.Code)
	}
	if rec := env.do(t, "GET", "/api/v1/links", nil, cookies, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("unenrolled session should be gated: %d", rec.Code)
	}
	rec := env.do(t, "GET", "/api/v1/me", nil, cookies, "")
	var me meDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if rec.Code != http.StatusOK || !me.MFAEnrollRequired {
		t.Fatalf("/me should report enrollment required: %s", rec.Body.String())
	}
	if rec := env.do(t, "GET", "/api/v1/me/mfa", nil, cookies, ""); rec.Code != http.StatusOK {
		t.Fatalf("enrollment endpoints must stay open: %d", rec.Code)
	}
	// API keys are separate credentials and not affected
	if rec := serve(env.srv.handler, "GET", "/api/v1/links", nil, key); rec.Code != http.StatusOK {
		t.Fatalf("api key gated: %d", rec.Code)
	}
}
