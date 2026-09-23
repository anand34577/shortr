import * as React from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { AlertTriangle, Copy, KeyRound, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from "@/components/ui/dialog";
import { FieldError } from "@/components/field-error";
import { LocalTime } from "@/components/local-time";
import { EmptyState } from "@/components/empty-state";
import { SettingsNav } from "@/features/settings/settings-nav";
import { createApiKeySchema, apiKeyScopes, type CreateApiKeyInput } from "@/lib/schemas";
import { applyServerErrors, toastError } from "@/lib/apply-server-errors";
import { confirm } from "@/components/confirm-dialog";
import { useApiKeys, useCreateApiKey, useRevokeApiKey } from "@/features/settings/api";
import type { ApiKeyCreated } from "@/lib/types";
import { copyText } from "@/lib/clipboard";

export default function ApiKeysPage() {
  const { data, isLoading } = useApiKeys();
  const create = useCreateApiKey();
  const revoke = useRevokeApiKey();
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [created, setCreated] = React.useState<ApiKeyCreated | null>(null);

  const {
    register,
    control,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<CreateApiKeyInput>({
    resolver: zodResolver(createApiKeySchema),
    defaultValues: { name: "", scopes: ["links:read", "links:write", "stats:read"] },
  });

  async function handleRevoke(id: string, name: string) {
    const ok = await confirm({
      title: `Revoke “${name}”?`,
      description: "Anything using this key stops working immediately. This cannot be undone.",
      confirmLabel: "Revoke key",
      variant: "destructive",
    });
    if (!ok) return;
    try {
      await revoke.mutateAsync(id);
      toast.success("API key revoked");
    } catch (err) {
      toastError(err, "Couldn't revoke the key");
    }
  }

  async function onSubmit(values: CreateApiKeyInput) {
    try {
      const key = await create.mutateAsync(values);
      setCreated(key);
      setDialogOpen(false);
      reset();
    } catch (err) {
      applyServerErrors(err, setError);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
        <SettingsNav />
      </div>

      <Card className="max-w-2xl">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base">API keys</CardTitle>
            <CardDescription>For scripts, the Android app, or the browser extension.</CardDescription>
          </div>
          <Button size="sm" className="gap-1.5" onClick={() => setDialogOpen(true)}>
            <Plus className="size-4" /> New key
          </Button>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-24 w-full" />
          ) : !data?.items.length ? (
            <EmptyState icon={KeyRound} title="No API keys yet" description="Create one to use the API from scripts or other clients." />
          ) : (
            <div className="flex flex-col divide-y divide-border">
              {data.items.map((k) => (
                <div key={k.id} className="flex items-center gap-3 py-3">
                  <KeyRound className="size-4 text-muted-foreground" />
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">{k.name}</p>
                    <p className="font-mono text-xs text-muted-foreground">{k.prefix}••••••••</p>
                    <div className="mt-1 flex flex-wrap gap-1">
                      {k.scopes.map((s) => (
                        <Badge key={s} variant="outline" className="text-[10px]">
                          {s}
                        </Badge>
                      ))}
                    </div>
                  </div>
                  <div className="text-right text-xs text-muted-foreground">
                    {k.lastUsedAt ? (
                      <>
                        Last used <LocalTime iso={k.lastUsedAt} relative />
                      </>
                    ) : (
                      "Never used"
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Revoke ${k.name}`}
                    title="Revoke key"
                    onClick={() => handleRevoke(k.id, k.name)}
                  >
                    <Trash2 className="size-4 text-destructive" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create API key</DialogTitle>
            <DialogDescription>The key is shown once — copy it somewhere safe.</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="key-name">Name</Label>
              <Input id="key-name" placeholder="e.g. Android phone" autoFocus {...register("name")} />
              <FieldError message={errors.name?.message} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Scopes</Label>
              <Controller
                control={control}
                name="scopes"
                render={({ field }) => (
                  <div className="flex flex-col gap-2">
                    {apiKeyScopes.map((scope) => (
                      <label key={scope} className="flex min-h-8 cursor-pointer items-center gap-2 text-sm">
                        <Checkbox
                          checked={field.value.includes(scope)}
                          onCheckedChange={(c) =>
                            field.onChange(c ? [...field.value, scope] : field.value.filter((s) => s !== scope))
                          }
                        />
                        {scope}
                      </label>
                    ))}
                  </div>
                )}
              />
              <FieldError message={errors.scopes?.message as string | undefined} />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? "Creating…" : "Create key"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={!!created} onOpenChange={(o) => !o && setCreated(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>API key created</DialogTitle>
            <DialogDescription className="flex items-center gap-1.5 text-warning">
              <AlertTriangle className="size-4" /> This is shown only once. Copy it now.
            </DialogDescription>
          </DialogHeader>
          {created && (
            <div className="flex items-center gap-2 rounded-lg border border-border bg-muted px-3 py-2">
              <code className="flex-1 overflow-x-auto text-sm">{created.key}</code>
              <Button
                variant="ghost"
                size="icon"
                aria-label="Copy API key"
                onClick={async () => {
                  const copied = await copyText(created.key);
                  toast[copied ? "success" : "error"](copied ? "Copied to clipboard" : "Copy failed", {
                    description: copied ? undefined : created.key,
                  });
                }}
              >
                <Copy className="size-4" />
              </Button>
            </div>
          )}
          <DialogFooter>
            <Button onClick={() => setCreated(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
