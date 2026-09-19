import { Link } from "react-router-dom";
import { useForm, Controller, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { FieldError } from "@/components/field-error";
import { AdminNav } from "@/features/admin/admin-nav";
import { useAdminSettings, useUpdateAdminSettings } from "@/features/admin/api";
import { adminSettingsSchema, type AdminSettingsInput } from "@/lib/schemas";
import { applyServerErrors } from "@/lib/apply-server-errors";

export default function AdminSettingsPage() {
  const { data, isLoading } = useAdminSettings();
  const update = useUpdateAdminSettings();

  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<AdminSettingsInput>({
    resolver: zodResolver(adminSettingsSchema),
    values: data
      ? {
          siteName: data.siteName,
          registration: data.registration,
          defaultRedirectStatus: data.defaultRedirectStatus,
          countBots: data.countBots,
          ipMode: data.ipMode,
          blockedDomains: data.blockedDomains ?? [],
          maxLinksPerUser: data.maxLinksPerUser,
          fetchTitles: data.fetchTitles,
          clickRetentionDays: data.clickRetentionDays,
          ipLocationEnabled: data.ipLocationEnabled,
          ipLocationBaseUrl: data.ipLocationBaseUrl,
          mcpEnabled: data.mcpEnabled,
        }
      : undefined,
  });
  const ipLocationEnabled = useWatch({ control, name: "ipLocationEnabled" });

  async function onSubmit(values: AdminSettingsInput) {
    try {
      await update.mutateAsync(values);
      toast.success("Settings saved");
    } catch (err) {
      applyServerErrors(err, setError);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Admin</h1>
        <AdminNav />
      </div>

      {isLoading || !data ? (
        <Skeleton className="h-96 w-full" />
      ) : (
        <form onSubmit={handleSubmit(onSubmit, (errs) => toast.error(`Fix these fields: ${Object.keys(errs).join(", ")}`))} className="flex flex-col gap-5" noValidate>
          <Card className="max-w-2xl">
            <CardHeader>
              <CardTitle className="text-base">General</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="siteName">Site name</Label>
                <Input id="siteName" {...register("siteName")} />
                <FieldError message={errors.siteName?.message} />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label>Base URL</Label>
                <Input value={data.baseUrl} disabled />
                <p className="text-xs text-muted-foreground">Set via SHORTR_BASE_URL; not editable here.</p>
              </div>
            </CardContent>
          </Card>

          <Card className="max-w-2xl">
            <CardHeader>
              <CardTitle className="text-base">Link defaults</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="defaultRedirectStatus">Default redirect status</Label>
                <Controller
                  control={control}
                  name="defaultRedirectStatus"
                  render={({ field }) => (
                    <Select value={String(field.value)} onValueChange={(v) => v && field.onChange(Number(v))}>
                      <SelectTrigger id="defaultRedirectStatus" className="w-40">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {[301, 302, 307, 308].map((s) => (
                          <SelectItem key={s} value={String(s)}>
                            {s}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
              </div>
              <div className="flex items-center justify-between">
                <Label htmlFor="fetchTitles">Fetch page titles on create</Label>
                <Controller control={control} name="fetchTitles" render={({ field }) => <Switch id="fetchTitles" checked={field.value} onCheckedChange={field.onChange} />} />
              </div>
              <div className="flex items-center justify-between">
                <Label htmlFor="countBots">Count bot clicks</Label>
                <Controller control={control} name="countBots" render={({ field }) => <Switch id="countBots" checked={field.value} onCheckedChange={field.onChange} />} />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="maxLinksPerUser">Max links per user (0 = unlimited)</Label>
                <Input id="maxLinksPerUser" type="number" min={0} {...register("maxLinksPerUser", { valueAsNumber: true })} />
                <FieldError message={errors.maxLinksPerUser?.message} />
              </div>
            </CardContent>
          </Card>

          <Card className="max-w-2xl">
            <CardHeader>
              <CardTitle className="text-base">Privacy</CardTitle>
              <CardDescription>Controls how click IP addresses are stored.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ipMode">IP storage mode</Label>
                <Controller
                  control={control}
                  name="ipMode"
                  render={({ field }) => (
                    <Select value={field.value} onValueChange={(v) => v && field.onChange(v)}>
                      <SelectTrigger id="ipMode" className="w-52">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="full">Full IP</SelectItem>
                        <SelectItem value="anonymize">Anonymized (/24, /48)</SelectItem>
                        <SelectItem value="hash">Hashed (HMAC, daily salt)</SelectItem>
                        <SelectItem value="none">Don't store</SelectItem>
                      </SelectContent>
                    </Select>
                  )}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="clickRetentionDays">Click retention (days, 0 = forever)</Label>
                <Input id="clickRetentionDays" type="number" min={0} {...register("clickRetentionDays", { valueAsNumber: true })} />
              </div>
            </CardContent>
          </Card>

          <Card className="max-w-2xl">
            <CardHeader>
              <CardTitle className="text-base">Registration & SSO</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="registration">Registration mode</Label>
                <Controller
                  control={control}
                  name="registration"
                  render={({ field }) => (
                    <Select value={field.value} onValueChange={(v) => v && field.onChange(v)}>
                      <SelectTrigger id="registration" className="w-44">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="closed">Closed</SelectItem>
                        <SelectItem value="open">Open</SelectItem>
                        <SelectItem value="invite">Invite only</SelectItem>
                      </SelectContent>
                    </Select>
                  )}
                />
              </div>
              <p className="text-xs text-muted-foreground">
                SSO (OIDC) is configured via environment variables and is currently {data.oidcEnabled ? "enabled" : "disabled"}.
              </p>
            </CardContent>
          </Card>

          <Card className="max-w-2xl">
            <CardHeader>
              <CardTitle className="text-base">IP location checker</CardTitle>
              <CardDescription>
                Optional: point at a self-hosted or third-party HTTP service (GET &lt;url&gt;/&lt;ip&gt;) to look up
                geolocation and ASN info for any IP address, on demand.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="flex items-center justify-between">
                <Label htmlFor="ipLocationEnabled">Enable IP location checker</Label>
                <Controller
                  control={control}
                  name="ipLocationEnabled"
                  render={({ field }) => <Switch id="ipLocationEnabled" checked={field.value} onCheckedChange={field.onChange} />}
                />
              </div>
              {ipLocationEnabled && (
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="ipLocationBaseUrl">Service base URL</Label>
                  <Input id="ipLocationBaseUrl" placeholder="https://ipinfo.example.com" {...register("ipLocationBaseUrl")} />
                  <FieldError message={errors.ipLocationBaseUrl?.message} />
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="max-w-2xl">
            <CardHeader>
              <CardTitle className="text-base">MCP server</CardTitle>
              <CardDescription>Let AI agents create and manage links using this instance, authenticated with an API key.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <div className="flex items-center justify-between">
                <Label htmlFor="mcpEnabled">Enable MCP server</Label>
                <Controller control={control} name="mcpEnabled" render={({ field }) => <Switch id="mcpEnabled" checked={field.value} onCheckedChange={field.onChange} />} />
              </div>
              <Link to="/app/settings/mcp" className="inline-flex w-fit items-center gap-1 text-sm text-primary hover:underline">
                Connection details & setup guide <ArrowRight className="size-3.5" />
              </Link>
            </CardContent>
          </Card>

          <Card className="max-w-2xl">
            <CardHeader>
              <CardTitle className="text-base">Blocked domains</CardTitle>
              <CardDescription>One per line. Exact or suffix match.</CardDescription>
            </CardHeader>
            <CardContent>
              <Controller
                control={control}
                name="blockedDomains"
                render={({ field }) => (
                  <Textarea
                    rows={4}
                    value={(field.value ?? []).join("\n")}
                    onChange={(e) => field.onChange(e.target.value.split("\n").map((s) => s.trim()).filter(Boolean))}
                  />
                )}
              />
            </CardContent>
          </Card>

          <div>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Saving…" : "Save settings"}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}
