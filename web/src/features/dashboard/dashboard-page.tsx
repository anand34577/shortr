import * as React from "react";
import { Link as RouterLink, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Area, AreaChart, ResponsiveContainer } from "recharts";
import { ArrowRight, Copy, Link2, TrendingDown, TrendingUp, Users, MousePointerClick, Globe2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { LocalTime } from "@/components/local-time";
import { EmptyState } from "@/components/empty-state";
import { useCreateLink } from "@/features/links/api";
import { useStatsOverview, useRecentActivity } from "@/features/dashboard/api";
import { RangePicker, rangeToDates, type RangeKey } from "@/features/links/detail/range-picker";
import { countryFlag } from "@/features/links/detail/breakdown-bars";
import { targetUrlSchema } from "@/lib/schemas";
import { copyText } from "@/lib/clipboard";
import { ApiError } from "@/lib/api";
import { toastError } from "@/lib/apply-server-errors";

export default function DashboardPage() {
  const navigate = useNavigate();
  const [quickUrl, setQuickUrl] = React.useState("");
  const [quickError, setQuickError] = React.useState<string | undefined>();
  const [lastCreated, setLastCreated] = React.useState<{ id: string; shortUrl: string } | null>(null);
  const create = useCreateLink();
  const [range, setRange] = React.useState<RangeKey>("7d");
  const { from, to } = rangeToDates(range);
  const stats = useStatsOverview({ from, to });
  const recent = useRecentActivity();

  async function handleShorten(e: React.FormEvent) {
    e.preventDefault();
    const parsed = targetUrlSchema.safeParse(quickUrl);
    if (!parsed.success) {
      setQuickError(parsed.error.issues[0]?.message);
      return;
    }
    setQuickError(undefined);
    try {
      const link = await create.mutateAsync({ targetUrl: parsed.data });
      setLastCreated({ id: link.id, shortUrl: link.shortUrl });
      const copied = await copyText(link.shortUrl);
      toast.success(copied ? "Shortened and copied to clipboard" : "Short link created", {
        description: copied ? undefined : link.shortUrl,
      });
      setQuickUrl("");
    } catch (err) {
      // field errors (e.g. a blocked domain) belong under the input
      if (err instanceof ApiError && err.fields?.target_url) setQuickError(err.fields.target_url);
      else toastError(err, "Couldn't shorten that URL");
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
        <p className="mt-1 text-sm text-muted-foreground">Shorten links and keep an eye on what’s getting clicked.</p>
      </div>

      <Card className="border-primary/20 bg-linear-to-br from-primary/5 to-transparent">
        <CardContent className="p-6">
          <h2 className="text-lg font-semibold">Shorten a link</h2>
          <p className="text-sm text-muted-foreground">Paste a URL and go — everything else is optional.</p>
          <form onSubmit={handleShorten} className="mt-4 flex flex-col gap-2 sm:flex-row" noValidate>
            <Input
              type="url"
              inputMode="url"
              value={quickUrl}
              onChange={(e) => {
                setQuickUrl(e.target.value);
                if (quickError) setQuickError(undefined);
              }}
              placeholder="https://example.com/your/long/link"
              className="flex-1"
              aria-label="URL to shorten"
              aria-invalid={!!quickError}
              aria-describedby={quickError ? "quick-url-error" : undefined}
              autoFocus
            />
            <Button type="submit" disabled={create.isPending} className="gap-1.5">
              {create.isPending ? "Shortening…" : "Shorten"}
              <ArrowRight className="size-4" />
            </Button>
          </form>
          {quickError && (
            <p id="quick-url-error" role="alert" className="mt-1.5 text-xs text-destructive">
              {quickError}
            </p>
          )}
          {lastCreated && (
            <div className="mt-3 flex items-center gap-2 rounded-lg border border-border bg-background px-3 py-2 text-sm">
              <Link2 className="size-4 shrink-0 text-primary" aria-hidden="true" />
              <span className="min-w-0 truncate font-mono">{lastCreated.shortUrl}</span>
              <Button
                variant="ghost"
                size="sm"
                className="ml-auto shrink-0"
                aria-label={`Copy ${lastCreated.shortUrl}`}
                onClick={async () => {
                  const copied = await copyText(lastCreated.shortUrl);
                  toast[copied ? "success" : "error"](copied ? "Copied to clipboard" : "Copy failed");
                }}
              >
                <Copy className="size-4" />
              </Button>
              <Button variant="ghost" size="sm" className="shrink-0" onClick={() => navigate(`/app/links/${lastCreated.id}`)}>
                Customize
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatTile
          icon={MousePointerClick}
          label="Total clicks"
          value={stats.data?.totals.clicks}
          delta={stats.data?.deltaPct}
          loading={stats.isLoading}
        />
        <StatTile icon={Users} label="Unique visitors" value={stats.data?.totals.uniques} loading={stats.isLoading} />
        <StatTile icon={Link2} label="Active links" value={stats.data?.totals.activeLinks} loading={stats.isLoading} />
        <StatTile
          icon={Globe2}
          label="Top referrer"
          value={stats.data?.topReferrer || "Direct"}
          loading={stats.isLoading}
          isText
        />
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-sm">Clicks</CardTitle>
          <RangePicker value={range} onChange={setRange} />
        </CardHeader>
        <CardContent>
          {stats.isLoading ? (
            <Skeleton className="h-64 w-full" />
          ) : stats.isError ? (
            <div className="flex h-64 flex-col items-center justify-center gap-3 text-center">
              <p className="text-sm text-muted-foreground">Couldn’t load click analytics.</p>
              <Button variant="outline" size="sm" onClick={() => stats.refetch()}>Retry</Button>
            </div>
          ) : !stats.data?.series.length ? (
            <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">No clicks yet — share a link to see activity here.</div>
          ) : (
            <ResponsiveContainer width="100%" height={260}>
              <AreaChart data={stats.data.series} aria-label="Clicks over time">
                <defs>
                  <linearGradient id="dashClicks" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="var(--color-primary)" stopOpacity={0.35} />
                    <stop offset="95%" stopColor="var(--color-primary)" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <Area type="monotone" dataKey="clicks" stroke="var(--color-primary)" fill="url(#dashClicks)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Recent activity</CardTitle>
        </CardHeader>
        <CardContent>
          {recent.isLoading ? (
            <div className="flex flex-col gap-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : recent.isError ? (
            <div className="flex flex-col items-center gap-3 py-8 text-center">
              <p className="text-sm text-muted-foreground">Couldn’t load recent activity.</p>
              <Button variant="outline" size="sm" onClick={() => recent.refetch()}>Retry</Button>
            </div>
          ) : !recent.data?.items.length ? (
            <EmptyState icon={MousePointerClick} title="No activity yet" description="Clicks on your links will show up here in real time." />
          ) : (
            <ul className="flex flex-col divide-y divide-border">
              {recent.data.items.map((item) => (
                <li key={item.id} className="flex items-center gap-3 py-2.5 text-sm">
                  <span aria-hidden="true">{countryFlag(item.country)}</span>
                  <RouterLink className="font-mono text-primary hover:underline" to={`/app/links/${item.linkId}`}>
                    {item.code}
                  </RouterLink>
                  <span className="text-muted-foreground">from {item.referrerHost || "direct"}</span>
                  <span className="ml-auto text-xs text-muted-foreground">
                    <LocalTime iso={item.ts} relative />
                  </span>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatTile({
  icon: Icon,
  label,
  value,
  delta,
  loading,
  isText,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value?: number | string;
  delta?: number | null;
  loading?: boolean;
  isText?: boolean;
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Icon className="size-3.5" />
          {label}
        </div>
        {loading ? (
          <Skeleton className="mt-2 h-7 w-20" />
        ) : (
          <div className="mt-1 flex items-baseline gap-1.5">
            <span className="text-2xl font-semibold tabular-nums">
              {isText ? value : typeof value === "number" ? value.toLocaleString() : value ?? "—"}
            </span>
            {typeof delta === "number" && (
              <span className={`flex items-center gap-0.5 text-xs ${delta >= 0 ? "text-success" : "text-destructive"}`}>
                {delta >= 0 ? <TrendingUp className="size-3" aria-hidden="true" /> : <TrendingDown className="size-3" aria-hidden="true" />}
                <span className="sr-only">{delta >= 0 ? "up" : "down"}</span>
                {Math.abs(delta).toFixed(0)}%<span className="sr-only"> vs. previous period</span>
              </span>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
