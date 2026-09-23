import * as React from "react";
import { toast } from "sonner";
import { Bell, BellRing, Mail, Radio } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { SettingsNav } from "@/features/settings/settings-nav";
import { useNotificationPreferences, useUpdateNotificationPreferences } from "@/features/settings/api";
import { useRequestNotificationPermission } from "@/hooks/use-notifications";
import type { NotificationChannel, NotificationKind } from "@/lib/types";
import { toastError } from "@/lib/apply-server-errors";
import { LoadError } from "@/components/load-error";

const KIND_LABELS: Record<NotificationKind, string> = {
  "link.expiring_soon": "A link is about to expire",
  "security.password_changed": "Your password was changed",
  "user.registered": "A new user signed up (admin)",
  "system.backup_failed": "Backup failed (admin)",
};

const CHANNELS: { key: NotificationChannel; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { key: "browser", label: "Browser", icon: Bell },
  { key: "email", label: "Email", icon: Mail },
  { key: "gotify", label: "Gotify", icon: Radio },
];

export default function NotificationsSettingsPage() {
  const { data, isLoading, isError, refetch } = useNotificationPreferences();
  const update = useUpdateNotificationPreferences();
  const requestPermission = useRequestNotificationPermission();
  const [permission, setPermission] = React.useState<NotificationPermission | "unsupported">(
    typeof Notification !== "undefined" ? Notification.permission : "unsupported",
  );

  async function toggle(kind: NotificationKind, channel: NotificationChannel, enabled: boolean) {
    if (!data) return;
    const current = data.channels[kind] ?? [];
    const next = enabled ? Array.from(new Set([...current, channel])) : current.filter((c) => c !== channel);
    try {
      await update.mutateAsync({ channels: { ...data.channels, [kind]: next } });
    } catch (err) {
      toastError(err, "Couldn't save your notification preferences");
    }
  }

  async function handleEnableBrowser() {
    const result = await requestPermission();
    setPermission(result);
    if (result === "granted") toast.success("Browser notifications enabled");
    else if (result === "denied") toast.error("Browser notifications were blocked. Enable them in your browser settings.");
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
        <SettingsNav />
      </div>

      {permission !== "granted" && permission !== "unsupported" && (
        <Card className="max-w-2xl border-warning/40 bg-warning/5">
          <CardContent className="flex items-center gap-3 p-4">
            <BellRing className="size-5 text-warning" />
            <div className="flex-1">
              <p className="text-sm font-medium">Enable browser notifications</p>
              <p className="text-xs text-muted-foreground">Get notified even when this tab isn't focused.</p>
            </div>
            <Button size="sm" onClick={handleEnableBrowser}>
              Enable
            </Button>
          </CardContent>
        </Card>
      )}

      <Card className="max-w-2xl">
        <CardHeader>
          <CardTitle className="text-base">Notification preferences</CardTitle>
          <CardDescription>Choose how you're notified for each type of event.</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-64 w-full" />
          ) : isError || !data ? (
            <LoadError what="your preferences" onRetry={() => refetch()} />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs text-muted-foreground">
                    <th className="py-2 font-medium">Event</th>
                    {CHANNELS.map((c) => (
                      <th key={c.key} className="px-3 py-2 text-center font-medium">
                        <span className="flex items-center justify-center gap-1">
                          <c.icon className="size-3.5" /> {c.label}
                        </span>
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {(Object.keys(KIND_LABELS) as NotificationKind[]).map((kind) => (
                    <tr key={kind} className="border-t border-border">
                      <td className="py-2.5 pr-3">{KIND_LABELS[kind]}</td>
                      {CHANNELS.map((c) => (
                        <td key={c.key} className="px-3 py-2.5 text-center">
                          <Switch
                            checked={(data.channels[kind] ?? []).includes(c.key)}
                            // each toggle sends the full map, so wait for the last save to land
                            disabled={update.isPending || (c.key === "gotify" && !data.gotifyConfigured)}
                            onCheckedChange={(checked) => toggle(kind, c.key, checked)}
                            aria-label={`${c.label} notifications for ${KIND_LABELS[kind]}`}
                          />
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
              {!data.gotifyConfigured && (
                <p className="mt-3 text-xs text-muted-foreground">
                  <Badge variant="outline" className="mr-1">
                    Gotify
                  </Badge>
                  Not configured by your administrator.
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
