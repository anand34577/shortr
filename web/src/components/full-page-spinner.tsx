import { Loader2 } from "lucide-react";

export function FullPageSpinner() {
  return (
    <div className="flex h-screen w-full items-center justify-center bg-background" role="status" aria-label="Loading">
      <Loader2 className="size-6 animate-spin text-muted-foreground" aria-hidden="true" />
    </div>
  );
}
