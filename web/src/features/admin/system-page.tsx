import type { ComponentType } from "react";
import { toast } from "sonner";
import { Database, HardDrive, RefreshCcw, Server, Zap, ShieldCheck, Mail, Bell, MapPin, Plug } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { LocalTime } from "@/components/local-time";
import { AdminNav } from "@/features/admin/admin-nav";
import { useAdminSystem, useTriggerBackup } from "@/features/admin/api";
import { toastError } from "@/lib/apply-server-errors";
import { LoadError } from "@/components/load-error";

function formatBytes(bytes: number) {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function formatUptime(seconds: number) {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return [d && `${d}d`, h && `${h}h`, `${m}m`].filter(Boolean).join(" ");
}

export default function SystemPage() {
  const { data, isLoading, isError, refetch } = useAdminSystem();
  const backup = useTriggerBackup();

  async function handleBackup() {
    try {
      await backup.mutateAsync();
      toast.success("Backup started");
    } catch (err) {
      toastError(err, "Couldn't start a backup");
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Admin</h1>
        <AdminNav />
      </div>

      {isLoading ? (
        <Skeleton className="h-64 w-full" />
      ) : isError || !data ? (
        <LoadError what="system status" onRetry={() => refetch()} />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <MetricCard icon={Server} label="Version" value={`${data.version} (${data.commit.slice(0, 7)})`} />
            <MetricCard icon={Database} label="Database" value={`${data.dbDriver} · ${formatBytes(data.dbSizeBytes)}`} />
            <MetricCard icon={Zap} label="Cache hit ratio" value={`${(data.cacheHitRatio * 100).toFixed(1)}%`} />
            <MetricCard icon={HardDrive} label="Uptime" value={formatUptime(data.uptimeSeconds)} />
          </div>

          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <MetricCard label="Cache entries" value={data.cacheEntries.toLocaleString()} />
            <MetricCard label="Click queue depth" value={data.queueDepth.toLocaleString()} />
            <MetricCard label="Dropped clicks" value={data.droppedClicks.toLocaleString()} />
            <MetricCard label="Detected proxy IP" value={data.detectedProxyIp || "—"} />
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Integrations</CardTitle>
              <CardDescription>Optional services this instance can use. Configure them in Admin → Settings.</CardDescription>
            </CardHeader>
            <CardContent className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
              <IntegrationBadge icon={ShieldCheck} label="SSO (OIDC)" enabled={data.oidcEnabled} />
              <IntegrationBadge icon={Mail} label="SMTP email" enabled={data.smtpEnabled} />
              <IntegrationBadge icon={Bell} label="Gotify push" enabled={data.gotifyEnabled} />
              <IntegrationBadge icon={MapPin} label="IP location checker" enabled={data.ipLocationEnabled} />
              <IntegrationBadge icon={Plug} label="MCP server" enabled={data.mcpEnabled} />
            </CardContent>
          </Card>

          <Card className="max-w-xl">
            <CardHeader>
              <CardTitle className="text-base">Backups</CardTitle>
            </CardHeader>
            <CardContent className="flex items-center justify-between">
              <div className="text-sm text-muted-foreground">
                Last backup: {data.lastBackupAt ? <LocalTime iso={data.lastBackupAt} relative /> : "never"}
              </div>
              <Button variant="outline" className="gap-1.5" onClick={handleBackup} disabled={backup.isPending}>
                <RefreshCcw className="size-4" /> Backup now
              </Button>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}

function IntegrationBadge({ icon: Icon, label, enabled }: { icon: ComponentType<{ className?: string }>; label: string; enabled: boolean }) {
  return (
    <div className="flex items-center gap-2 rounded-lg border border-border p-2.5">
      <Icon className={enabled ? "size-4 shrink-0 text-primary" : "size-4 shrink-0 text-muted-foreground"} />
      <div className="min-w-0 flex-1">
        <p className="truncate text-xs font-medium">{label}</p>
        <Badge variant={enabled ? "success" : "secondary"} className="mt-0.5 text-[10px]">
          {enabled ? "Enabled" : "Disabled"}
        </Badge>
      </div>
    </div>
  );
}

function MetricCard({ icon: Icon, label, value }: { icon?: ComponentType<{ className?: string }>; label: string; value: string }) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          {Icon && <Icon className="size-3.5" />}
          {label}
        </div>
        <p className="mt-1 truncate text-lg font-semibold">{value}</p>
      </CardContent>
    </Card>
  );
}
