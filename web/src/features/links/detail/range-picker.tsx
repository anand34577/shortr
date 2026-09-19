import { cn } from "@/lib/utils";

export type RangeKey = "24h" | "7d" | "30d" | "90d";

const OPTIONS: { key: RangeKey; label: string }[] = [
  { key: "24h", label: "24h" },
  { key: "7d", label: "7d" },
  { key: "30d", label: "30d" },
  { key: "90d", label: "90d" },
];

export function RangePicker({ value, onChange }: { value: RangeKey; onChange: (v: RangeKey) => void }) {
  return (
    <div className="inline-flex rounded-lg border border-border p-0.5" role="tablist" aria-label="Date range">
      {OPTIONS.map((opt) => (
        <button
          key={opt.key}
          role="tab"
          aria-selected={value === opt.key}
          onClick={() => onChange(opt.key)}
          className={cn(
            "rounded-md px-3 py-1 text-xs font-medium transition-colors",
            value === opt.key ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-accent",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}

// `to` is rounded down to the minute so repeated calls within the same
// render cycle (or the same minute) return byte-identical strings. Callers
// feed this straight into a TanStack Query `queryKey`; a fresh `new
// Date().toISOString()` on every render previously produced a new key every
// render, which refetched, which re-rendered, which changed `to` again —
// an infinite request loop (caught via the click-count-style server log:
// /api/v1/stats/overview was hit 8000+ times in under two minutes).
export function rangeToDates(range: RangeKey): { from: string; to: string; bucket: string } {
  const to = new Date();
  to.setSeconds(0, 0);
  const from = new Date(to);
  const bucket = range === "24h" ? "hour" : "day";
  if (range === "24h") from.setHours(from.getHours() - 24);
  if (range === "7d") from.setDate(from.getDate() - 7);
  if (range === "30d") from.setDate(from.getDate() - 30);
  if (range === "90d") from.setDate(from.getDate() - 90);
  return { from: from.toISOString(), to: to.toISOString(), bucket };
}
