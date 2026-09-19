import * as React from "react";
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { format, parseISO } from "date-fns";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useLinkStats } from "@/features/links/api";
import { RangePicker, rangeToDates, type RangeKey } from "@/features/links/detail/range-picker";

export function OverviewTab({ linkId }: { linkId: string }) {
  const [range, setRange] = React.useState<RangeKey>("7d");
  const { from, to, bucket } = rangeToDates(range);
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const { data, isLoading } = useLinkStats(linkId, { from, to, bucket, tz });

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Stat label="Clicks" value={data?.totals.clicks} loading={isLoading} />
        <Stat label="Uniques (approx.)" value={data?.totals.uniques} loading={isLoading} />
        <Stat label="Bots" value={data?.totals.bots} loading={isLoading} />
        <Stat
          label="Top country"
          value={data?.byCountry[0]?.key || "—"}
          loading={isLoading}
          isText
        />
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-sm">Clicks over time</CardTitle>
          <RangePicker value={range} onChange={setRange} />
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-64 w-full" />
          ) : !data?.series.length ? (
            <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">No data for this range yet.</div>
          ) : (
            <ResponsiveContainer width="100%" height={260}>
              <AreaChart data={data.series} margin={{ left: -20 }}>
                <defs>
                  <linearGradient id="clicksGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="var(--color-primary)" stopOpacity={0.35} />
                    <stop offset="95%" stopColor="var(--color-primary)" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" vertical={false} className="stroke-border" />
                <XAxis
                  dataKey="bucket"
                  tickFormatter={(v) => format(parseISO(v), bucket === "hour" ? "ha" : "MMM d")}
                  tick={{ fontSize: 12 }}
                  stroke="var(--color-muted-foreground)"
                />
                <YAxis tick={{ fontSize: 12 }} stroke="var(--color-muted-foreground)" allowDecimals={false} width={36} />
                <Tooltip
                  contentStyle={{
                    background: "var(--color-popover)",
                    border: "1px solid var(--color-border)",
                    borderRadius: 8,
                    fontSize: 12,
                  }}
                  labelFormatter={(v) => format(parseISO(String(v)), "PPp")}
                />
                <Area type="monotone" dataKey="clicks" stroke="var(--color-primary)" fill="url(#clicksGradient)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function Stat({ label, value, loading, isText }: { label: string; value?: number | string; loading?: boolean; isText?: boolean }) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-xs text-muted-foreground">{label}</p>
        {loading ? (
          <Skeleton className="mt-1 h-6 w-16" />
        ) : (
          <p className="mt-1 text-xl font-semibold tabular-nums">
            {isText ? value : typeof value === "number" ? value.toLocaleString() : value ?? "—"}
          </p>
        )}
      </CardContent>
    </Card>
  );
}
