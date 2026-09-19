import * as React from "react";
import { WifiOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { queryClient } from "@/app/providers";

/** Shown while the browser reports no network; offers a retry of failed queries. */
export function OfflineBanner() {
  const [online, setOnline] = React.useState(() => navigator.onLine);
  React.useEffect(() => {
    const up = () => setOnline(true);
    const down = () => setOnline(false);
    window.addEventListener("online", up);
    window.addEventListener("offline", down);
    return () => {
      window.removeEventListener("online", up);
      window.removeEventListener("offline", down);
    };
  }, []);
  if (online) return null;
  return (
    <div role="alert" className="flex items-center gap-2 bg-destructive px-4 py-2 text-sm text-destructive-foreground">
      <WifiOff className="size-4" /> You're offline. Changes won't be saved until the connection is back.
      <Button size="sm" variant="secondary" className="ml-auto" onClick={() => queryClient.refetchQueries()}>
        Retry
      </Button>
    </div>
  );
}
