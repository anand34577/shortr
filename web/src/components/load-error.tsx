import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/** Inline error state for a failed query, with a retry. */
export function LoadError({ what, onRetry, className }: { what: string; onRetry: () => void; className?: string }) {
  return (
    <div role="alert" className={cn("flex flex-col items-center gap-3 py-10 text-center", className)}>
      <p className="text-sm text-muted-foreground">Couldn’t load {what}.</p>
      <Button variant="outline" size="sm" onClick={onRetry}>
        Retry
      </Button>
    </div>
  );
}
