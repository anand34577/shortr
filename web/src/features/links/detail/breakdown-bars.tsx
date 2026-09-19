import { Skeleton } from "@/components/ui/skeleton";
import type { StatsBreakdownItem } from "@/lib/types";

export function BreakdownBars({ items, loading, labelPrefix }: { items?: StatsBreakdownItem[]; loading?: boolean; labelPrefix?: (key: string) => string }) {
  if (loading) {
    return (
      <div className="flex flex-col gap-2">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-6 w-full" />
        ))}
      </div>
    );
  }

  if (!items || items.length === 0) {
    return <p className="py-6 text-center text-sm text-muted-foreground">No data yet.</p>;
  }

  const max = Math.max(...items.map((i) => i.clicks), 1);

  return (
    <ul className="flex flex-col gap-2">
      {items.map((item) => (
        <li key={item.key || "unknown"} className="flex items-center gap-3 text-sm">
          <span className="w-28 shrink-0 truncate text-muted-foreground">
            {labelPrefix ? labelPrefix(item.key) : item.key || "Unknown"}
          </span>
          <div className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-primary"
              style={{ width: `${Math.max((item.clicks / max) * 100, 3)}%` }}
            />
          </div>
          <span className="w-14 shrink-0 text-right tabular-nums text-muted-foreground">{item.clicks.toLocaleString()}</span>
        </li>
      ))}
    </ul>
  );
}

function countryFlag(code: string) {
  if (!code || code.length !== 2) return "🌐";
  const A = 0x1f1e6;
  const chars = code
    .toUpperCase()
    .split("")
    .map((c) => A + (c.charCodeAt(0) - 65));
  return String.fromCodePoint(...chars);
}

export { countryFlag };
