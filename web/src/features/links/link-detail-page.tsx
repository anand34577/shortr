import * as React from "react";
import { useParams, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { ArrowLeft, Copy, ExternalLink, Ban, Trash2, CheckCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { LocalTime } from "@/components/local-time";
import { useLink, useDeleteLink, useRestoreLink, useUpdateLink } from "@/features/links/api";
import { QrPanel } from "@/features/links/qr-panel";
import { LinkFormDialog } from "@/features/links/link-form-dialog";
import { OverviewTab } from "@/features/links/detail/overview-tab";
import { AudienceTab } from "@/features/links/detail/audience-tab";
import { ReferrersTab } from "@/features/links/detail/referrers-tab";
import { ClicksTab } from "@/features/links/detail/clicks-tab";

export default function LinkDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: link, isLoading } = useLink(id);
  const del = useDeleteLink();
  const restore = useRestoreLink();
  const update = useUpdateLink(id ?? "");
  const [editOpen, setEditOpen] = React.useState(false);

  if (isLoading || !link) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  async function copyShort() {
    await navigator.clipboard.writeText(link!.shortUrl);
    toast.success("Copied to clipboard");
  }

  async function handleDelete() {
    await del.mutateAsync(link!.id);
    toast("Link deleted", {
      duration: 8000,
      action: { label: "Undo", onClick: () => restore.mutate(link!.id) },
    });
    navigate("/app/links");
  }

  async function toggleStatus() {
    const next = link!.status === "active" ? "disabled" : "active";
    await update.mutateAsync({ status: next } as never);
    toast.success(next === "active" ? "Link enabled" : "Link disabled");
  }

  return (
    <div className="flex flex-col gap-5">
      <Button variant="ghost" size="sm" className="w-fit gap-1.5 text-muted-foreground" onClick={() => navigate("/app/links")}>
        <ArrowLeft className="size-4" /> Back to links
      </Button>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <button onClick={copyShort} className="flex items-center gap-1.5 font-mono text-lg font-semibold text-primary">
              {link.shortUrl.replace(/^https?:\/\//, "")}
              <Copy className="size-4" />
            </button>
            <Badge variant={link.status === "active" ? "success" : "secondary"}>{link.status}</Badge>
            {link.hasPassword && <Badge variant="outline">Password protected</Badge>}
          </div>
          <a
            href={link.targetUrl}
            target="_blank"
            rel="noreferrer"
            className="mt-1 flex max-w-xl items-center gap-1.5 truncate text-sm text-muted-foreground hover:text-foreground"
          >
            <span className="truncate">{link.targetUrl}</span>
            <ExternalLink className="size-3.5 shrink-0" />
          </a>
          <p className="mt-1 text-xs text-muted-foreground">
            Created <LocalTime iso={link.createdAt} relative /> · {link.clickCount.toLocaleString()} clicks
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setEditOpen(true)}>
            Edit
          </Button>
          <Button variant="outline" onClick={toggleStatus} className="gap-1.5">
            {link.status === "active" ? <Ban className="size-4" /> : <CheckCircle className="size-4" />}
            {link.status === "active" ? "Disable" : "Enable"}
          </Button>
          <Button variant="outline" className="gap-1.5 text-destructive" onClick={handleDelete}>
            <Trash2 className="size-4" /> Delete
          </Button>
        </div>
      </div>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="audience">Audience</TabsTrigger>
          <TabsTrigger value="referrers">Referrers</TabsTrigger>
          <TabsTrigger value="clicks">Clicks</TabsTrigger>
          <TabsTrigger value="settings">Settings</TabsTrigger>
          <TabsTrigger value="qr">QR</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <OverviewTab linkId={link.id} />
        </TabsContent>
        <TabsContent value="audience">
          <AudienceTab linkId={link.id} />
        </TabsContent>
        <TabsContent value="referrers">
          <ReferrersTab linkId={link.id} />
        </TabsContent>
        <TabsContent value="clicks">
          <ClicksTab linkId={link.id} />
        </TabsContent>
        <TabsContent value="settings">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Link settings</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3 text-sm">
              <Row label="Title" value={link.title || "—"} />
              <Row label="Description" value={link.description || "—"} />
              <Row label="Redirect status" value={String(link.redirectStatus)} />
              <Row label="Pass-through query" value={link.passQuery ? "Yes" : "No"} />
              <Row label="Expires" value={link.expiresAt ? <LocalTime iso={link.expiresAt} /> : "Never"} />
              <Row label="Click limit" value={link.maxClicks?.toLocaleString() ?? "Unlimited"} />
              <Row
                label="Tags"
                value={
                  link.tags.length ? (
                    <div className="flex flex-wrap gap-1">
                      {link.tags.map((t) => (
                        <Badge key={t} variant="outline">
                          {t}
                        </Badge>
                      ))}
                    </div>
                  ) : (
                    "—"
                  )
                }
              />
              <Button variant="outline" className="mt-2 w-fit" onClick={() => setEditOpen(true)}>
                Edit link
              </Button>
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="qr">
          <Card>
            <CardContent className="pt-6">
              <QrPanel link={link} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <LinkFormDialog open={editOpen} onOpenChange={setEditOpen} link={link} />
    </div>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-border pb-2 last:border-0 last:pb-0">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right font-medium">{value}</span>
    </div>
  );
}
