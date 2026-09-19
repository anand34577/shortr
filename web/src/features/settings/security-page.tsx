import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { Monitor, Smartphone, ShieldCheck, Unlink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { FieldError } from "@/components/field-error";
import { LocalTime } from "@/components/local-time";
import { SettingsNav } from "@/features/settings/settings-nav";
import { changePasswordSchema, type ChangePasswordInput } from "@/lib/schemas";
import { applyServerErrors } from "@/lib/apply-server-errors";
import { useMe } from "@/hooks/use-me";
import { useAuthStatus } from "@/hooks/use-auth-status";
import {
  useChangePassword,
  useSessions,
  useRevokeSession,
  useIdentities,
  useUnlinkIdentity,
} from "@/features/settings/api";
import { oidcStartUrl } from "@/features/auth/api";

export default function SecurityPage() {
  const me = useMe();
  const status = useAuthStatus();
  const changePassword = useChangePassword();
  const sessions = useSessions();
  const revokeSession = useRevokeSession();
  const identities = useIdentities();
  const unlink = useUnlinkIdentity();

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<ChangePasswordInput>({ resolver: zodResolver(changePasswordSchema) });

  async function onSubmit(values: ChangePasswordInput) {
    try {
      await changePassword.mutateAsync(values);
      toast.success("Password updated. Other sessions were signed out.");
      reset();
    } catch (err) {
      applyServerErrors(err, setError);
    }
  }

  const canUnlink = (identities.data?.items.length ?? 0) > 1 || me.data?.mustChangePassword === false;

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
        <SettingsNav />
      </div>

      <Card className="max-w-lg">
        <CardHeader>
          <CardTitle className="text-base">Password</CardTitle>
          <CardDescription>{me.data?.hasPassword ? "Change your password." : "Set a password for this account."}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
            {me.data?.hasPassword && (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="current">Current password</Label>
                <Input id="current" type="password" {...register("current")} />
                <FieldError message={errors.current?.message} />
              </div>
            )}
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="next">New password</Label>
              <Input id="next" type="password" {...register("next")} />
              <FieldError message={errors.next?.message} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="confirm">Confirm new password</Label>
              <Input id="confirm" type="password" {...register("confirm")} />
              <FieldError message={errors.confirm?.message} />
            </div>
            <Button type="submit" disabled={isSubmitting} className="w-fit">
              {isSubmitting ? "Updating…" : "Update password"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card className="max-w-lg">
        <CardHeader>
          <CardTitle className="text-base">Active sessions</CardTitle>
          <CardDescription>Devices currently signed in to your account.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col divide-y divide-border">
          {sessions.isLoading ? (
            <Skeleton className="h-16 w-full" />
          ) : (
            sessions.data?.items.map((s) => (
              <div key={s.id} className="flex items-center gap-3 py-3">
                {/mobile|android|iphone/i.test(s.userAgent) ? (
                  <Smartphone className="size-4 text-muted-foreground" />
                ) : (
                  <Monitor className="size-4 text-muted-foreground" />
                )}
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">
                    {s.userAgent.slice(0, 60) || "Unknown device"} {s.current && <Badge variant="outline">This device</Badge>}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {s.ip} · last seen <LocalTime iso={s.lastSeenAt} relative />
                  </p>
                </div>
                {!s.current && (
                  <Button variant="ghost" size="sm" onClick={() => revokeSession.mutate(s.id)}>
                    Revoke
                  </Button>
                )}
              </div>
            ))
          )}
        </CardContent>
      </Card>

      <Card className="max-w-lg">
        <CardHeader>
          <CardTitle className="text-base">Connected accounts</CardTitle>
          <CardDescription>Single sign-on identities linked to this account.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {identities.isLoading ? (
            <Skeleton className="h-12 w-full" />
          ) : identities.data?.items.length ? (
            identities.data.items.map((id) => (
              <div key={id.id} className="flex items-center gap-3 rounded-lg border border-border p-3">
                <ShieldCheck className="size-4 text-success" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{id.issuer}</p>
                  <p className="text-xs text-muted-foreground">{id.email} · linked <LocalTime iso={id.createdAt} relative /></p>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={!canUnlink}
                  title={!canUnlink ? "Set a password or link another provider before unlinking" : undefined}
                  onClick={() => unlink.mutate(id.id)}
                  className="gap-1.5"
                >
                  <Unlink className="size-3.5" /> Unlink
                </Button>
              </div>
            ))
          ) : (
            <p className="text-sm text-muted-foreground">No SSO accounts linked.</p>
          )}
          {status.data?.oidcEnabled && (
            <Button variant="outline" className="w-fit" asChild>
              <a href={oidcStartUrl("/app/settings/security")}>Link {status.data.oidcDisplayName || "SSO"} account</a>
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
