import type { LucideIcon } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import type { StatsBreakdownItem } from "@/lib/types";

export function BreakdownBars({ items, loading, icon: Icon }: { items?: StatsBreakdownItem[]; loading?: boolean; icon?: LucideIcon }) {
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
          <span className="flex w-28 shrink-0 items-center gap-1.5 truncate text-muted-foreground">
            {Icon && <Icon className="size-3.5 shrink-0" aria-hidden="true" />}
            <span className="truncate">{item.key || "Unknown"}</span>
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

