import * as React from "react";
import { FileClock, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { LocalTime } from "@/components/local-time";
import { EmptyState } from "@/components/empty-state";
import { AdminNav } from "@/features/admin/admin-nav";
import { useAuditLog } from "@/features/admin/api";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useCursorPager } from "@/hooks/use-cursor-pager";
import { cn } from "@/lib/utils";
import type { AuditEntry } from "@/lib/types";

// Action filters match by prefix on the server, so "user.login_" finds
// both failed and locked-out sign-ins.
const PRESETS: { label: string; action: string }[] = [
  { label: "All", action: "" },
  { label: "Sign-ins", action: "user.login" },
  { label: "Failed sign-ins", action: "user.login_" },
  { label: "Links", action: "link." },
  { label: "Users", action: "user." },
  { label: "API keys", action: "apikey." },
  { label: "SSO", action: "oidc." },
  { label: "Settings", action: "settings." },
];

const VIA_LABEL: Record<string, string> = { session: "web", api_key: "API key", oidc_token: "SSO token" };

function actionVariant(action: string): "destructive" | "warning" | "outline" {
  if (/_failed$|_locked$/.test(action)) return "destructive";
  if (/delete|purge|revoke|unlink|reset_password/.test(action)) return "warning";
  return "outline";
}

/** One short, human line out of the entry's metadata. */
function describe(meta: AuditEntry["meta"]): string {
  if (!meta) return "";
  const parts: string[] = [];
  const str = (k: string) => (typeof meta[k] === "string" && meta[k] ? String(meta[k]) : "");
  if (str("via")) parts.push(`via ${VIA_LABEL[str("via")] ?? str("via")}`);
  if (str("email")) parts.push(str("email"));
  if (str("code")) parts.push(`/${str("code")}`);
  if (str("name")) parts.push(`“${str("name")}”`);
  if (Array.isArray(meta.changed) && meta.changed.length) parts.push(`changed ${meta.changed.join(", ")}`);
  if (str("from") && str("to")) parts.push(`${str("from")} → ${str("to")}`);
  if (meta.bulk === true) parts.push("bulk");
  return parts.join(" · ");
}

export default function AuditPage() {
  const [action, setAction] = React.useState("");
  const [actor, setActor] = React.useState<{ id: string; label: string } | null>(null);
  const debouncedAction = useDebouncedValue(action, 300);
  const pager = useCursorPager();
  const { data, isLoading, isFetching } = useAuditLog({
    cursor: pager.cursor,
    action: debouncedAction.trim() || undefined,
    actor: actor?.id,
  });
  const filtered = Boolean(debouncedAction || actor);

  const pickAction = (a: string) => {
    setAction(a);
    pager.reset();
  };

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Admin</h1>
        <AdminNav />
      </div>

      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap gap-1.5" role="group" aria-label="Quick filters">
          {PRESETS.map((p) => (
            <Button
              key={p.label}
              size="sm"
              variant={action === p.action ? "default" : "outline"}
              className="h-7 rounded-full px-3 text-xs"
              aria-pressed={action === p.action}
              onClick={() => pickAction(p.action)}
            >
              {p.label}
            </Button>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Input
            value={action}
            onChange={(e) => pickAction(e.target.value)}
            placeholder="Filter by action prefix, e.g. link.create"
            className="max-w-sm"
            aria-label="Filter by action"
          />
          {actor && (
            <Badge variant="secondary" className="h-7 gap-1 pl-2.5 pr-1 text-xs">
              Actor: {actor.label}
              <button
                type="button"
                className="rounded p-0.5 hover:bg-foreground/10"
                aria-label="Clear actor filter"
                onClick={() => {
                  setActor(null);
                  pager.reset();
                }}
              >
                <X className="size-3" />
              </button>
            </Badge>
          )}
        </div>
      </div>

      <Card>
        <CardContent className={cn("p-0 transition-opacity", isFetching && !isLoading && "opacity-70")}>
          {isLoading ? (
            <div className="p-4">
              <Skeleton className="h-64 w-full" />
            </div>
          ) : !data?.items.length ? (
            <EmptyState
              icon={FileClock}
              title={filtered ? "Nothing matches these filters" : "No audit entries yet"}
              description={filtered ? "Try a broader prefix, or clear the filters to see everything." : undefined}
            />
          ) : (
            <>
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Time</TableHead>
                      <TableHead>Actor</TableHead>
                      <TableHead>Action</TableHead>
                      <TableHead>Details</TableHead>
                      <TableHead>IP</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {data.items.map((entry) => {
                      const who = entry.actorEmail || entry.actorUserId;
                      const ua = typeof entry.meta?.ua === "string" ? entry.meta.ua : undefined;
                      return (
                        <TableRow key={entry.id}>
                          <TableCell className="whitespace-nowrap text-sm">
                            <LocalTime iso={entry.ts} />
                          </TableCell>
                          <TableCell className="max-w-48 truncate text-sm">
                            {who ? (
                              <button
                                type="button"
                                className="truncate underline-offset-4 hover:underline"
                                title="Show only this actor"
                                onClick={() => {
                                  setActor({ id: entry.actorUserId, label: who });
                                  pager.reset();
                                }}
                              >
                                {who}
                              </button>
                            ) : (
                              <span className="text-muted-foreground">anonymous</span>
                            )}
                          </TableCell>
                          <TableCell>
                            <Badge variant={actionVariant(entry.action)} className="font-mono text-[11px]">
                              {entry.action}
                            </Badge>
                          </TableCell>
                          <TableCell className="max-w-80 truncate text-sm text-muted-foreground" title={ua}>
                            {describe(entry.meta) || (entry.targetId ? `${entry.targetType} ${entry.targetId.slice(0, 8)}` : "—")}
                          </TableCell>
                          <TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">
                            {entry.actorIp || "—"}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>
              <nav className="flex items-center justify-between border-t p-3" aria-label="Pagination">
                <Button variant="outline" size="sm" disabled={!pager.hasPrevious} onClick={pager.previous}>
                  Previous
                </Button>
                <span className="text-xs text-muted-foreground">Page {pager.page}</span>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={!data.nextCursor}
                  onClick={() => data.nextCursor && pager.next(data.nextCursor)}
                >
                  Next
                </Button>
              </nav>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
