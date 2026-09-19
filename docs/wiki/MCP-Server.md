# MCP Server

Shortr exposes a subset of the REST API as [Model Context
Protocol](https://modelcontextprotocol.io/) tools, so an AI agent (Claude,
an IDE assistant, a custom agent) can manage short links using the same
per-user, scoped API keys as the REST API — there's no separate auth system
to configure.

## Enabling it

MCP is off by default. An admin turns it on in **Admin → Settings**. Once
enabled, it's reachable at `POST {SHORTR_BASE_URL}/mcp`.

## Transport

The endpoint implements the "Streamable HTTP" transport from the spec, in
its simplest legal form: one JSON-RPC 2.0 request per POST, one JSON
response per call — no SSE stream. That's sufficient for agents that only
need synchronous tool calls (the common case for link management).

## Authenticating

Same as the REST API: `Authorization: Bearer sk_...` with an API key
carrying whatever scopes the tools you want to call require (see below), or
a session cookie if you're calling it from an already-authenticated browser
context. `/mcp` is subject to the same `SHORTR_RATE_LIMIT_API` limiter as
the rest of `/api/*`.

## Available tools

| Tool | Required scope | Description |
|---|---|---|
| `create_short_link` | `links:write` | Create a new short link; returns the created link and its short URL. |
| `list_links` | `links:read` | List the caller's links, newest first — free-text search, status filter, cursor pagination. |
| `get_link` | `links:read` | Get a single link by ID or short code. |
| `delete_link` | `links:write` | Soft-delete a link (recoverable for 30 days). |
| `get_link_analytics` | `stats:read` | Click totals + top breakdowns (country/device/browser/referrer) over the last N days. |
| `check_ip_location` | `stats:read` | Geolocation/ASN lookup for an IP, if GeoIP is configured. |

A session-authenticated caller (not an API key) implicitly has every scope
their role allows, same as the REST API. An API-key caller missing the
required scope for a tool gets a JSON-RPC error naming the missing scope.

## Example: initialize + call a tool

```bash
KEY="sk_..."
BASE="https://links.example.com"

# 1. initialize (optional but conventional for MCP clients)
curl -sS "$BASE/mcp" -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' -d '{
  "jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}
}'

# 2. list tools
curl -sS "$BASE/mcp" -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' -d '{
  "jsonrpc": "2.0", "id": 2, "method": "tools/list"
}'

# 3. call a tool
curl -sS "$BASE/mcp" -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' -d '{
  "jsonrpc": "2.0", "id": 3, "method": "tools/call",
  "params": { "name": "create_short_link", "arguments": { "targetUrl": "https://example.com/very/long/path" } }
}'
```

## Configuring an MCP client

Most MCP-capable clients (Claude Desktop, Claude Code, IDE extensions) that
support a remote/HTTP MCP server just need the URL and an `Authorization`
header:

```json
{
  "mcpServers": {
    "shortr": {
      "url": "https://links.example.com/mcp",
      "headers": { "Authorization": "Bearer sk_..." }
    }
  }
}
```

Check your specific client's docs for the exact config key names — some
clients call this `httpUrl`/`transport: "http"` instead of `url`.

## Security notes

- Create a dedicated API key for MCP use with the minimum scopes the agent
  actually needs (e.g. `links:write` only, not `admin:*`), rather than
  reusing a personal browser session key.
- `SHORTR_ALLOW_PRIVATE_TARGETS=true` affects `create_short_link` the same
  way it affects the REST API — don't enable it unless you specifically need
  agents (or anyone) to be able to target your internal network.
