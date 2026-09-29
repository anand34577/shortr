import * as React from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";
import { toast } from "sonner";
import { Copy, Download, KeyRound, ShieldAlert, ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { FieldError } from "@/components/field-error";
import { confirm } from "@/components/confirm-dialog";
import { ApiError, api } from "@/lib/api";
import { copyText } from "@/lib/clipboard";
import { toastError } from "@/lib/apply-server-errors";
import { meQueryKey } from "@/hooks/use-me";
import type { MfaSetup, MfaStatus } from "@/lib/types";

const mfaKey = ["mfa"];

function useMfaStatus() {
  return useQuery({ queryKey: mfaKey, queryFn: () => api.get<MfaStatus>("/api/v1/me/mfa") });
}

/** Two-factor sign-in (TOTP) for password accounts. */
export function TwoFactorCard({ highlight }: { highlight?: boolean }) {
  const qc = useQueryClient();
  const status = useMfaStatus();
  const [setup, setSetup] = React.useState<MfaSetup | null>(null);
  const [codes, setCodes] = React.useState<string[] | null>(null);

  const refresh = () => {
    qc.invalidateQueries({ queryKey: mfaKey });
    qc.invalidateQueries({ queryKey: meQueryKey });
  };

  const start = useMutation({ mutationFn: () => api.post<MfaSetup>("/api/v1/me/mfa/totp/setup") });
  const regen = useMutation({ mutationFn: () => api.post<{ recoveryCodes: string[] }>("/api/v1/me/mfa/recovery-codes") });
  const disable = useMutation({ mutationFn: () => api.delete("/api/v1/me/mfa") });

  async function handleStart() {
    try {
      setSetup(await start.mutateAsync());
    } catch (err) {
      toastError(err, "Couldn't start the setup");
    }
  }

  async function handleRegen() {
    const ok = await confirm({
      title: "Make new recovery codes?",
      description: "Your old recovery codes stop working as soon as the new ones are shown.",
      confirmLabel: "Make new codes",
    });
    if (!ok) return;
    try {
      setCodes((await regen.mutateAsync()).recoveryCodes);
      refresh();
    } catch (err) {
      toastError(err, "Couldn't make new codes");
    }
  }

  async function handleDisable() {
    const ok = await confirm({
      title: "Turn off two-factor sign-in?",
      description: status.data?.required
        ? "Your administrator requires it, so you'll be asked to set it up again straight away."
        : "Anyone who learns your password will be able to sign in.",
      confirmLabel: "Turn off",
      variant: "destructive",
    });
    if (!ok) return;
    try {
      await disable.mutateAsync();
      toast.success("Two-factor sign-in is off");
      refresh();
    } catch (err) {
      toastError(err, "Couldn't turn it off");
    }
  }

  const s = status.data;
  return (
    <Card className={highlight ? "border-primary ring-2 ring-primary/30" : undefined}>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle className="text-base">Two-factor sign-in</CardTitle>
          {s?.enabled && <Badge variant="success">On</Badge>}
          {s && !s.enabled && s.available && <Badge variant={s.required ? "warning" : "secondary"}>{s.required ? "Required" : "Off"}</Badge>}
        </div>
        <CardDescription>
          After your password, you also enter a code from an authenticator app such as Aegis, 2FAS, Google Authenticator or 1Password.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {status.isLoading ? (
          <Skeleton className="h-10 w-full" />
        ) : !s?.available ? (
          <p className="text-sm text-muted-foreground">
            You sign in with single sign-on, so two-factor sign-in is handled by your identity provider.
          </p>
        ) : s.enabled ? (
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <p className="flex items-center gap-2 text-sm">
              <ShieldCheck className="size-4 text-success" />
              {s.recoveryCodesLeft} recovery code{s.recoveryCodesLeft === 1 ? "" : "s"} left
              {s.recoveryCodesLeft <= 3 && <span className="text-warning">· make new ones soon</span>}
            </p>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={handleRegen} disabled={regen.isPending}>
                <KeyRound className="size-4" /> New recovery codes
              </Button>
              <Button variant="outline" size="sm" className="text-destructive" onClick={handleDisable} disabled={disable.isPending}>
                Turn off
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              <ShieldAlert className="size-4" />
              {s.required ? "Your administrator requires this before you can continue." : "Your account is protected by your password only."}
            </p>
            <Button onClick={handleStart} disabled={start.isPending}>
              {start.isPending ? "Starting…" : "Set up"}
            </Button>
          </div>
        )}
      </CardContent>

      <EnrollDialog
        setup={setup}
        onClose={() => setSetup(null)}
        onEnabled={(c) => {
          setSetup(null);
          setCodes(c);
          refresh();
        }}
      />
      <RecoveryCodesDialog codes={codes} onClose={() => setCodes(null)} />
    </Card>
  );
}

function EnrollDialog({ setup, onClose, onEnabled }: { setup: MfaSetup | null; onClose: () => void; onEnabled: (codes: string[]) => void }) {
  const [code, setCode] = React.useState("");
  const [error, setError] = React.useState<string | null>(null);
  const enable = useMutation({
    mutationFn: (c: string) => api.post<{ recoveryCodes: string[] }>("/api/v1/me/mfa/totp/enable", { code: c }),
  });

  React.useEffect(() => {
    setCode("");
    setError(null);
  }, [setup]);

  async function submit(c = code) {
    if (enable.isPending) return;
    try {
      onEnabled((await enable.mutateAsync(c)).recoveryCodes);
    } catch (err) {
      setError(err instanceof ApiError ? (err.fields?.code ?? err.message) : "Something went wrong");
      setCode("");
    }
  }

  const grouped = setup?.secret.replace(/(.{4})/g, "$1 ").trim();

  return (
    <Dialog open={!!setup} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Set up two-factor sign-in</DialogTitle>
          <DialogDescription>Scan the code with your authenticator app, then type the 6 digits it shows.</DialogDescription>
        </DialogHeader>
        {setup && (
          <div className="flex flex-col gap-4">
            <div className="flex flex-col items-center gap-3 sm:flex-row sm:items-start">
              <div className="rounded-md bg-white p-2">
                <QRCodeSVG value={setup.otpauthUrl} size={148} level="M" aria-label="Authenticator setup QR code" />
              </div>
              <div className="min-w-0 text-sm">
                <p className="text-muted-foreground">Can't scan? Enter this key instead:</p>
                <button
                  type="button"
                  className="mt-1 flex items-start gap-1.5 break-all rounded-md bg-muted px-2 py-1.5 text-left font-mono text-xs"
                  onClick={async () => toast[(await copyText(setup.secret)) ? "success" : "error"]("Key copied")}
                >
                  {grouped}
                  <Copy className="mt-0.5 size-3 shrink-0" />
                </button>
                <p className="mt-2 text-xs text-muted-foreground">Type: time-based · 6 digits · 30 seconds</p>
              </div>
            </div>
            <form
              className="flex flex-col gap-1.5"
              onSubmit={(e) => {
                e.preventDefault();
                void submit();
              }}
            >
              <Label htmlFor="enroll-code">Code from the app</Label>
              <Input
                id="enroll-code"
                autoFocus
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={6}
                placeholder="123456"
                className="text-center font-mono text-xl tracking-[0.4em]"
                value={code}
                onChange={(e) => {
                  const d = e.target.value.replace(/\D/g, "").slice(0, 6);
                  setCode(d);
                  setError(null);
                  if (d.length === 6) void submit(d);
                }}
              />
              <FieldError message={error ?? undefined} />
              <DialogFooter className="mt-2">
                <Button type="button" variant="ghost" onClick={onClose}>
                  Cancel
                </Button>
                <Button type="submit" disabled={code.length !== 6 || enable.isPending}>
                  {enable.isPending ? "Checking…" : "Turn on"}
                </Button>
              </DialogFooter>
            </form>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function RecoveryCodesDialog({ codes, onClose }: { codes: string[] | null; onClose: () => void }) {
  const [saved, setSaved] = React.useState(false);
  React.useEffect(() => setSaved(false), [codes]);
  const text = codes?.join("\n") ?? "";

  function download() {
    const blob = new Blob([`Shortr recovery codes (${new Date().toLocaleDateString()})\nEach code works once.\n\n${text}\n`], { type: "text/plain" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "shortr-recovery-codes.txt";
    a.click();
    URL.revokeObjectURL(a.href);
    setSaved(true);
  }

  return (
    <Dialog open={!!codes} onOpenChange={(o) => !o && saved && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Save your recovery codes</DialogTitle>
          <DialogDescription>
            If you lose your phone, each of these lets you sign in once. They won't be shown again.
          </DialogDescription>
        </DialogHeader>
        <ol className="grid grid-cols-2 gap-x-6 gap-y-1.5 rounded-lg border border-border bg-muted p-4 font-mono text-sm">
          {codes?.map((c) => (
            <li key={c}>{c}</li>
          ))}
        </ol>
        <div className="flex gap-2">
          <Button
            variant="outline"
            className="flex-1"
            onClick={async () => {
              const ok = await copyText(text);
              toast[ok ? "success" : "error"](ok ? "Codes copied" : "Copy failed");
              if (ok) setSaved(true);
            }}
          >
            <Copy className="size-4" /> Copy
          </Button>
          <Button variant="outline" className="flex-1" onClick={download}>
            <Download className="size-4" /> Download
          </Button>
        </div>
        <DialogFooter>
          <Button onClick={onClose} disabled={!saved}>
            {saved ? "Done" : "Copy or download first"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
