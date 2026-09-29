import * as React from "react";
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { format } from "date-fns";
import type { StatsSeriesPoint } from "@/lib/types";

// Buckets come from the server as UTC "YYYY-MM-DD" or "YYYY-MM-DDTHH".
// Hourly ones are shown in the viewer's local time; days stay calendar days.
function bucketDate(b: string): Date {
  return b.length > 10 ? new Date(`${b}:00:00Z`) : new Date(`${b}T00:00:00`);
}

export function ClicksChart({ series, height = 260 }: { series: StatsSeriesPoint[]; height?: number }) {
  const gradientId = "clicks" + React.useId().replace(/[^a-zA-Z0-9]/g, ""); // raw useId breaks url(#…)
  const hourly = series[0]?.bucket.length > 10;
  return (
    <ResponsiveContainer width="100%" height={height}>
      <AreaChart data={series} margin={{ left: -20, right: 8 }} aria-label="Clicks over time">
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="var(--color-primary)" stopOpacity={0.35} />
            <stop offset="95%" stopColor="var(--color-primary)" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" vertical={false} className="stroke-border" />
        <XAxis
          dataKey="bucket"
          tickFormatter={(v) => format(bucketDate(String(v)), hourly ? "ha" : "MMM d")}
          tick={{ fontSize: 12 }}
          stroke="var(--color-muted-foreground)"
          minTickGap={24}
        />
        <YAxis tick={{ fontSize: 12 }} stroke="var(--color-muted-foreground)" allowDecimals={false} width={36} />
        <Tooltip
          contentStyle={{
            background: "var(--color-popover)",
            border: "1px solid var(--color-border)",
            borderRadius: 8,
            fontSize: 12,
          }}
          labelFormatter={(v) => format(bucketDate(String(v)), hourly ? "PPp" : "PP")}
        />
        <Area
          type="monotone"
          dataKey="clicks"
          name="Clicks"
          stroke="var(--color-primary)"
          fill={`url(#${gradientId})`}
          strokeWidth={2}
        />
      </AreaChart>
    </ResponsiveContainer>
  );
}
