import * as React from "react";
import { MapPin, Search, Building2, Clock, Globe2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/empty-state";
import { FieldError } from "@/components/field-error";
import { useIPLookup } from "@/features/tools/api";
import { isValidIP } from "@/lib/utils";
import { ApiError } from "@/lib/api";

function Stat({ icon: Icon, label, value }: { icon: React.ComponentType<{ className?: string }>; label: string; value: string }) {
  return (
    <div className="flex items-start gap-2.5">
      <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div>
        <p className="text-xs text-muted-foreground">{label}</p>
        <p className="text-sm font-medium">{value || "—"}</p>
      </div>
    </div>
  );
}

export default function IPLookupPage() {
  const [ip, setIp] = React.useState("");
  const [touched, setTouched] = React.useState(false);
  const lookup = useIPLookup();

  const invalid = touched && ip.trim() !== "" && !isValidIP(ip.trim());

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setTouched(true);
    const trimmed = ip.trim();
    if (!trimmed || !isValidIP(trimmed)) return;
    lookup.mutate(trimmed);
  }

  const notConfigured = lookup.error instanceof ApiError && lookup.error.code === "NOT_CONFIGURED";

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">IP Lookup</h1>
        <p className="text-sm text-muted-foreground">Look up geolocation and network info for any IP address.</p>
      </div>

      <Card className="max-w-2xl">
        <CardHeader>
          <CardTitle className="text-base">Check an IP address</CardTitle>
          <CardDescription>Uses the IP location checker service configured by your admin.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <form onSubmit={onSubmit} className="flex items-start gap-2" noValidate>
            <div className="flex-1">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={ip}
                  onChange={(e) => setIp(e.target.value)}
                  onBlur={() => setTouched(true)}
                  placeholder="e.g. 1.1.1.1 or 2606:4700:4700::1111"
                  className="pl-9 font-mono"
                  maxLength={45}
                  aria-label="IP address"
                  aria-invalid={invalid}
                />
              </div>
              {invalid && <FieldError message="Enter a valid IPv4 or IPv6 address" />}
            </div>
            <Button type="submit" disabled={lookup.isPending || !ip.trim()}>
              {lookup.isPending ? "Looking up…" : "Look up"}
            </Button>
          </form>

          {notConfigured ? (
            <EmptyState
              icon={MapPin}
              title="Not configured"
              description="Ask an admin to enable the IP location checker in Admin → Settings."
            />
          ) : lookup.isError ? (
            <p className="text-sm text-destructive">{lookup.error instanceof Error ? lookup.error.message : "Lookup failed"}</p>
          ) : lookup.data ? (
            <div className="grid grid-cols-1 gap-4 rounded-lg border border-border p-4 sm:grid-cols-2">
              <Stat
                icon={MapPin}
                label="Location"
                value={[lookup.data.city, lookup.data.subdivision, lookup.data.country].filter(Boolean).join(", ")}
              />
              <Stat icon={Globe2} label="Coordinates" value={lookup.data.latitude || lookup.data.longitude ? `${lookup.data.latitude}, ${lookup.data.longitude}` : ""} />
              <Stat icon={Clock} label="Timezone" value={lookup.data.timezone} />
              <Stat icon={Building2} label="Network" value={lookup.data.asnOrganization ? `AS${lookup.data.asn} · ${lookup.data.asnOrganization}` : ""} />
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
