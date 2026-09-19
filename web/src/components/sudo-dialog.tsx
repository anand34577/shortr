import * as React from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { setSudoPrompt } from "@/lib/api";

/** Asks for the account password when the API answers SUDO_REQUIRED, then lets the request retry. */
export function SudoDialog() {
  const [open, setOpen] = React.useState(false);
  const [password, setPassword] = React.useState("");
  const resolver = React.useRef<((value: string | null) => void) | null>(null);

  React.useEffect(() => {
    setSudoPrompt(
      () =>
        new Promise<string | null>((resolve) => {
          resolver.current = resolve;
          setPassword("");
          setOpen(true);
        }),
    );
    return () => setSudoPrompt(null);
  }, []);

  function finish(value: string | null) {
    resolver.current?.(value);
    resolver.current = null;
    setOpen(false);
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && finish(null)}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Confirm your password</DialogTitle>
          <DialogDescription>This action needs you to re-enter your password.</DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (password) finish(password);
          }}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="sudo-password">Password</Label>
            <Input
              id="sudo-password"
              type="password"
              autoFocus
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => finish(null)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!password}>
              Continue
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
