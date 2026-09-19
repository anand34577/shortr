import * as React from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

interface ConfirmOptions {
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: "default" | "destructive";
}

type Resolver = (value: boolean) => void;
let requestConfirm: ((opts: ConfirmOptions) => Promise<boolean>) | null = null;

/** Promise-based replacement for window.confirm, rendered as a themed dialog. */
export function confirm(opts: ConfirmOptions): Promise<boolean> {
  if (!requestConfirm) return Promise.resolve(false);
  return requestConfirm(opts);
}

export function ConfirmDialogHost() {
  const [opts, setOpts] = React.useState<ConfirmOptions | null>(null);
  const resolver = React.useRef<Resolver | null>(null);

  React.useEffect(() => {
    requestConfirm = (o) =>
      new Promise<boolean>((resolve) => {
        resolver.current = resolve;
        setOpts(o);
      });
    return () => {
      requestConfirm = null;
    };
  }, []);

  function finish(value: boolean) {
    resolver.current?.(value);
    resolver.current = null;
    setOpts(null);
  }

  return (
    <Dialog open={!!opts} onOpenChange={(o) => !o && finish(false)}>
      <DialogContent className="max-w-sm">
        {opts && (
          <>
            <DialogHeader>
              <DialogTitle>{opts.title}</DialogTitle>
              {opts.description && <DialogDescription>{opts.description}</DialogDescription>}
            </DialogHeader>
            <DialogFooter>
              <Button variant="ghost" onClick={() => finish(false)}>
                {opts.cancelLabel ?? "Cancel"}
              </Button>
              <Button variant={opts.variant === "destructive" ? "destructive" : "default"} onClick={() => finish(true)} autoFocus>
                {opts.confirmLabel ?? "Continue"}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
