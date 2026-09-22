import * as React from "react";
import { toast } from "sonner";
import { Copy, Plug, CheckCircle2, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { SettingsNav } from "@/features/settings/settings-nav";
import { api } from "@/lib/api";
import { copyText } from "@/lib/clipboard";

interface McpTool {
  name: string;
  description: string;
}

function CodeBlock({ children }: { children: string }) {
  return (
    <div className="flex items-start gap-2 rounded-lg border border-border bg-muted px-3 py-2">
      <pre className="min-w-0 flex-1 overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs">{children}</pre>
      <Button
        variant="ghost"
        size="icon"
        className="shrink-0"
        aria-label="Copy"
        onClick={async () => {
          const copied = await copyText(children);
          toast[copied ? "success" : "error"](copied ? "Copied to clipboard" : "Copy failed", {
            description: copied ? undefined : children,
          });
        }}
      >
        <Copy className="size-4" />
      </Button>
    </div>
  );
}

export default function McpPage() {
  const [status, setStatus] = React.useState<"checking" | "enabled" | "disabled">("checking");
  const [tools, setTools] = React.useState<McpTool[] | null>(null);

  React.useEffect(() => {
    let cancelled = false;
    api
      .post<{ error?: { message: string }; result?: { tools: McpTool[] } }>("/mcp", { jsonrpc: "2.0", id: 1, method: "tools/list" })
      .then((res) => {
        if (cancelled) return;
        if (res.error) {
          setStatus("disabled");
        } else {
          setStatus("enabled");
          setTools(res.result?.tools ?? []);
        }
      })
      .catch(() => !cancelled && setStatus("disabled"));
    return () => {
      cancelled = true;
    };
  }, []);

  const endpoint = `${window.location.origin}/mcp`;
  const clientConfig = JSON.stringify(
    { mcpServers: { shortr: { url: endpoint, headers: { Authorization: "Bearer sk_your_api_key_here" } } } },
    null,
    2,
  );

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
        <SettingsNav />
      </div>

      <Card className="max-w-2xl">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base">MCP server</CardTitle>
            <CardDescription>Connect AI agents (Claude, Cursor, and other MCP clients) to your links and analytics.</CardDescription>
          </div>
          {status === "checking" ? (
            <Skeleton className="h-5 w-20" />
          ) : status === "enabled" ? (
            <Badge variant="success" className="gap-1">
              <CheckCircle2 className="size-3" /> Enabled
            </Badge>
          ) : (
            <Badge variant="secondary" className="gap-1">
              <XCircle className="size-3" /> Disabled
            </Badge>
          )}
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {status === "disabled" && (
            <p className="text-sm text-muted-foreground">
              An admin needs to turn this on in <span className="font-medium text-foreground">Admin → Settings → MCP server</span> before
              agents can connect.
            </p>
          )}

          <div className="flex flex-col gap-1.5">
            <p className="text-sm font-medium">1. Create an API key</p>
            <p className="text-sm text-muted-foreground">
              Go to <span className="font-medium text-foreground">Settings → API Keys</span> and create a key with the scopes you want to
              grant the agent (<code className="text-xs">links:read</code>, <code className="text-xs">links:write</code>,{" "}
              <code className="text-xs">stats:read</code>).
            </p>
          </div>

          <div className="flex flex-col gap-1.5">
            <p className="text-sm font-medium">2. Endpoint</p>
            <CodeBlock>{endpoint}</CodeBlock>
          </div>

          <div className="flex flex-col gap-1.5">
            <p className="text-sm font-medium">3. Add to your MCP client config</p>
            <p className="text-sm text-muted-foreground">
              Works with any client that supports Streamable HTTP MCP servers with a custom header (Claude Desktop, Cursor, etc. via{" "}
              <code className="text-xs">mcp-remote</code> or native HTTP support).
            </p>
            <CodeBlock>{clientConfig}</CodeBlock>
          </div>

          <div className="flex flex-col gap-1.5">
            <p className="text-sm font-medium">Available tools</p>
            {tools === null ? (
              <p className="text-sm text-muted-foreground">Tool list unavailable until MCP is enabled.</p>
            ) : (
              <div className="flex flex-col divide-y divide-border rounded-lg border border-border">
                {tools.map((t) => (
                  <div key={t.name} className="flex items-start gap-2 p-2.5">
                    <Plug className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
                    <div>
                      <p className="font-mono text-xs font-medium">{t.name}</p>
                      <p className="text-xs text-muted-foreground">{t.description}</p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
