// MCP exposes a subset of the REST API as Model Context Protocol tools, so
// an AI agent can create/list/inspect links (and check IP locations) using
// the same per-user API keys and scopes as the REST API — no separate auth
// system. Transport is the "Streamable HTTP" flavor from the spec, restricted
// to its simplest legal form: one JSON-RPC request per POST, one JSON
// response per call, no SSE stream (agents that only need synchronous tool
// calls don't need one). It is off by default and toggled from Admin >
// Settings, same as SMTP/Gotify/OIDC.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"shortr/internal/iploc"
	"shortr/internal/link"
	"shortr/internal/store"
)

const mcpProtocolVersion = "2024-11-05"

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	mcpParseError     = -32700
	mcpInvalidRequest = -32600
	mcpMethodNotFound = -32601
	mcpInvalidParams  = -32602
	mcpInternalError  = -32603
)

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	scope       string         // REST scope this tool requires from an API-key caller; "" = any authenticated identity
}

type mcpToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolResult struct {
	Content []mcpToolContent `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

func textResult(v any) mcpToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcpToolResult{Content: []mcpToolContent{{Type: "text", Text: err.Error()}}, IsError: true}
	}
	return mcpToolResult{Content: []mcpToolContent{{Type: "text", Text: string(b)}}}
}

func errResult(msg string) mcpToolResult {
	return mcpToolResult{Content: []mcpToolContent{{Type: "text", Text: msg}}, IsError: true}
}

var mcpTools = []mcpTool{
	{
		Name:        "create_short_link",
		Description: "Create a new short link. Returns the created link, including its short URL.",
		scope:       "links:write",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"targetUrl"},
			"properties": map[string]any{
				"targetUrl": map[string]any{"type": "string", "description": "The destination URL to redirect to."},
				"code":      map[string]any{"type": "string", "description": "Optional custom short code; auto-generated if omitted."},
				"title":     map[string]any{"type": "string", "description": "Optional human-readable title."},
				"tags":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
		},
	},
	{
		Name:        "list_links",
		Description: "List the caller's short links, newest first. Supports free-text search and status filtering.",
		scope:       "links:read",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":  map[string]any{"type": "string", "description": "Free-text search over code, title, and target URL."},
				"status": map[string]any{"type": "string", "enum": []string{"active", "disabled", "deleted"}},
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 25},
				"cursor": map[string]any{"type": "string", "description": "Pagination cursor from a previous call's nextCursor."},
			},
		},
	},
	{
		Name:        "get_link",
		Description: "Get a single link by its ID or short code.",
		scope:       "links:read",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"idOrCode"},
			"properties": map[string]any{
				"idOrCode": map[string]any{"type": "string", "description": "The link's ID or short code."},
			},
		},
	},
	{
		Name:        "delete_link",
		Description: "Soft-delete a link by its ID or short code (recoverable for 30 days).",
		scope:       "links:write",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"idOrCode"},
			"properties": map[string]any{
				"idOrCode": map[string]any{"type": "string"},
			},
		},
	},
	{
		Name:        "get_link_analytics",
		Description: "Get click totals and top breakdowns (country, device, browser, referrer) for a link over the last N days (default 30).",
		scope:       "stats:read",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"idOrCode"},
			"properties": map[string]any{
				"idOrCode": map[string]any{"type": "string"},
				"days":     map[string]any{"type": "integer", "minimum": 1, "maximum": 730, "default": 30},
			},
		},
	},
	{
		Name:        "check_ip_location",
		Description: "Look up geolocation and ASN/organization info for an IP address, if the admin has configured an IP location checker service.",
		scope:       "stats:read",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"ip"},
			"properties": map[string]any{
				"ip": map[string]any{"type": "string", "description": "IPv4 or IPv6 address to look up."},
			},
		},
	},
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request, u *store.User) {
	if !s.mcpEnabled(r.Context()) {
		writeMCPError(w, nil, mcpInvalidRequest, "MCP server is disabled — an admin can enable it in Admin > Settings")
		return
	}

	var req mcpRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeMCPError(w, nil, mcpParseError, "invalid JSON")
		return
	}
	isNotification := len(req.ID) == 0

	result, rpcErr := s.dispatchMCP(r.Context(), u, req)
	if isNotification {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if rpcErr != nil {
		writeMCPError(w, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}
	respondJSON(w, http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result})
}

func writeMCPError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	respondJSON(w, http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: id, Error: &mcpRPCError{Code: code, Message: message}})
}

func (s *Server) dispatchMCP(ctx context.Context, u *store.User, req mcpRequest) (any, *mcpRPCError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "shortr", "version": s.version},
		}, nil
	case "notifications/initialized", "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": mcpTools}, nil
	case "tools/call":
		return s.dispatchMCPToolCall(ctx, u, req.Params)
	default:
		return nil, &mcpRPCError{Code: mcpMethodNotFound, Message: "unknown method: " + req.Method}
	}
}

func (s *Server) dispatchMCPToolCall(ctx context.Context, u *store.User, rawParams json.RawMessage) (any, *mcpRPCError) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(rawParams, &call); err != nil {
		return nil, &mcpRPCError{Code: mcpInvalidParams, Message: "invalid tool call params"}
	}
	var tool *mcpTool
	for i := range mcpTools {
		if mcpTools[i].Name == call.Name {
			tool = &mcpTools[i]
			break
		}
	}
	if tool == nil {
		return nil, &mcpRPCError{Code: mcpInvalidParams, Message: "unknown tool: " + call.Name}
	}
	if tool.scope != "" {
		if key := apiKeyFromContext(ctx); key != nil && !hasScope(key.Scopes, tool.scope) {
			// Reported as a tool-level error (not a JSON-RPC error) so the
			// agent's model sees why and can tell the user which scope to add.
			return errResult("API key missing required scope: " + tool.scope), nil
		}
	}

	args := call.Arguments
	switch call.Name {
	case "create_short_link":
		return s.mcpCreateLink(ctx, u, args), nil
	case "list_links":
		return s.mcpListLinks(ctx, u, args), nil
	case "get_link":
		return s.mcpGetLink(ctx, u, args), nil
	case "delete_link":
		return s.mcpDeleteLink(ctx, u, args), nil
	case "get_link_analytics":
		return s.mcpLinkAnalytics(ctx, u, args), nil
	case "check_ip_location":
		return s.mcpCheckIP(ctx, args), nil
	default:
		return nil, &mcpRPCError{Code: mcpInvalidParams, Message: "unknown tool: " + call.Name}
	}
}

func (s *Server) mcpResolveLink(ctx context.Context, u *store.User, idOrCode string) (*store.Link, error) {
	if l, err := s.links.Get(ctx, idOrCode); err == nil {
		return l, nil
	}
	return s.links.GetByCode(ctx, idOrCode)
}

func (s *Server) mcpOwnedLink(ctx context.Context, u *store.User, idOrCode string) (*store.Link, *mcpToolResult) {
	l, err := s.mcpResolveLink(ctx, u, idOrCode)
	if err != nil {
		r := errResult("link not found: " + idOrCode)
		return nil, &r
	}
	if !u.IsAdmin() && (l.UserID == nil || *l.UserID != u.ID) {
		r := errResult("you do not have access to this link")
		return nil, &r
	}
	return l, nil
}

func (s *Server) mcpCreateLink(ctx context.Context, u *store.User, args json.RawMessage) mcpToolResult {
	var in struct {
		TargetURL string   `json:"targetUrl"`
		Code      string   `json:"code"`
		Title     string   `json:"title"`
		Tags      []string `json:"tags"`
	}
	if err := json.Unmarshal(args, &in); err != nil || in.TargetURL == "" {
		return errResult("targetUrl is required")
	}
	uid := u.ID
	l, err := s.links.Create(ctx, &uid, "", link.CreateInput{TargetURL: in.TargetURL, Code: in.Code, Title: in.Title, Tags: in.Tags})
	if err != nil {
		return errResult(err.Error())
	}
	return textResult(s.toLinkDTO(l))
}

func (s *Server) mcpListLinks(ctx context.Context, u *store.User, args json.RawMessage) mcpToolResult {
	var in struct {
		Query  string `json:"query"`
		Status string `json:"status"`
		Limit  int    `json:"limit"`
		Cursor string `json:"cursor"`
	}
	_ = json.Unmarshal(args, &in)
	if in.Limit <= 0 {
		in.Limit = 25
	}
	links, next, err := s.store.ListLinks(ctx, store.LinkFilter{
		UserID: u.ID, Query: in.Query, Status: in.Status, Limit: in.Limit, Cursor: in.Cursor,
		Sort: "created_at", Order: "desc",
	})
	if err != nil {
		return errResult(err.Error())
	}
	out := make([]linkDTO, 0, len(links))
	for _, l := range links {
		out = append(out, s.toLinkDTO(l))
	}
	return textResult(map[string]any{"items": out, "nextCursor": next})
}

func (s *Server) mcpGetLink(ctx context.Context, u *store.User, args json.RawMessage) mcpToolResult {
	var in struct {
		IDOrCode string `json:"idOrCode"`
	}
	if err := json.Unmarshal(args, &in); err != nil || in.IDOrCode == "" {
		return errResult("idOrCode is required")
	}
	l, errRes := s.mcpOwnedLink(ctx, u, in.IDOrCode)
	if errRes != nil {
		return *errRes
	}
	return textResult(s.toLinkDTO(l))
}

func (s *Server) mcpDeleteLink(ctx context.Context, u *store.User, args json.RawMessage) mcpToolResult {
	var in struct {
		IDOrCode string `json:"idOrCode"`
	}
	if err := json.Unmarshal(args, &in); err != nil || in.IDOrCode == "" {
		return errResult("idOrCode is required")
	}
	l, errRes := s.mcpOwnedLink(ctx, u, in.IDOrCode)
	if errRes != nil {
		return *errRes
	}
	if err := s.links.Delete(ctx, l.ID, l.Code); err != nil {
		return errResult(err.Error())
	}
	return textResult(map[string]any{"deleted": true, "id": l.ID})
}

func (s *Server) mcpLinkAnalytics(ctx context.Context, u *store.User, args json.RawMessage) mcpToolResult {
	var in struct {
		IDOrCode string `json:"idOrCode"`
		Days     int    `json:"days"`
	}
	if err := json.Unmarshal(args, &in); err != nil || in.IDOrCode == "" {
		return errResult("idOrCode is required")
	}
	if in.Days <= 0 {
		in.Days = 30
	}
	l, errRes := s.mcpOwnedLink(ctx, u, in.IDOrCode)
	if errRes != nil {
		return *errRes
	}
	to := time.Now()
	from := to.Add(-time.Duration(in.Days) * 24 * time.Hour)

	totalClicks, err := s.store.TotalClicks(ctx, l.ID, from, to, false)
	if err != nil {
		return errResult(err.Error())
	}
	uniques, err := s.store.UniqueVisitors(ctx, l.ID, from, to)
	if err != nil {
		return errResult(err.Error())
	}
	byCountry, _ := s.store.ClicksBreakdown(ctx, l.ID, "country", from, to, 5)
	byDevice, _ := s.store.ClicksBreakdown(ctx, l.ID, "device", from, to, 5)
	byReferrer, _ := s.store.ClicksBreakdown(ctx, l.ID, "referrer_host", from, to, 5)

	return textResult(map[string]any{
		"link": s.toLinkDTO(l), "days": in.Days,
		"totals":     map[string]any{"clicks": totalClicks, "uniques": uniques},
		"byCountry":  toBDRows(byCountry),
		"byDevice":   toBDRows(byDevice),
		"byReferrer": toBDRows(byReferrer),
	})
}

func (s *Server) mcpCheckIP(ctx context.Context, args json.RawMessage) mcpToolResult {
	var in struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(args, &in); err != nil || in.IP == "" {
		return errResult("ip is required")
	}
	enabled, baseURL := s.ipLocationSettings(ctx)
	if !enabled || baseURL == "" {
		return errResult("IP location checker is not configured — ask an admin to enable it in Admin > Settings")
	}
	res, err := iploc.Lookup(ctx, baseURL, in.IP)
	if err != nil {
		return errResult(err.Error())
	}
	return textResult(res)
}
