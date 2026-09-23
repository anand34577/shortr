import * as React from "react";
import { useParams, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { ArrowLeft, Copy, ExternalLink, Ban, Trash2, CheckCircle, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { LocalTime } from "@/components/local-time";
import { useLink, useDeleteLink, useRestoreLink, useUpdateLink } from "@/features/links/api";
import { QrPanel } from "@/features/links/qr-panel";
import { LinkFormDialog } from "@/features/links/link-form-dialog";
import { confirm } from "@/components/confirm-dialog";
import { copyText } from "@/lib/clipboard";
import { toastError } from "@/lib/apply-server-errors";
import { OverviewTab } from "@/features/links/detail/overview-tab";
import { AudienceTab } from "@/features/links/detail/audience-tab";
import { ReferrersTab } from "@/features/links/detail/referrers-tab";
import { ClicksTab } from "@/features/links/detail/clicks-tab";

export default function LinkDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: link, isLoading, isError, refetch } = useLink(id);
  const del = useDeleteLink();
  const restore = useRestoreLink();
  const update = useUpdateLink(id ?? "");
  const [editOpen, setEditOpen] = React.useState(false);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-4" role="status" aria-label="Loading link">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  if (isError || !link) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <h1 className="text-lg font-semibold">Couldn’t load this link</h1>
        <p className="max-w-md text-sm text-muted-foreground">The link may have been removed or the request failed.</p>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => refetch()}>Retry</Button>
          <Button onClick={() => navigate("/app/links")}>Back to links</Button>
        </div>
      </div>
    );
  }

  const currentLink = link;

  async function copyShort() {
    const copied = await copyText(currentLink.shortUrl);
    toast[copied ? "success" : "error"](copied ? "Copied to clipboard" : "Copy failed", {
      description: copied ? undefined : currentLink.shortUrl,
    });
  }

  async function handleDelete() {
    const confirmed = await confirm({
      title: "Delete this link?",
      description: "The link will stop redirecting. You can restore it from the Links page.",
      confirmLabel: "Delete link",
      variant: "destructive",
    });
    if (!confirmed) return;
    try {
      await del.mutateAsync(currentLink.id);
    } catch (err) {
      toastError(err, "Couldn't delete the link");
      return;
    }
    toast("Link deleted", {
      duration: 8000,
      action: { label: "Undo", onClick: () => handleRestore() },
    });
    navigate("/app/links");
  }

  async function handleRestore() {
    try {
      await restore.mutateAsync(currentLink.id);
      toast.success("Link restored");
    } catch (err) {
      toastError(err, "Couldn't restore the link");
    }
  }

  async function toggleStatus() {
    const next = currentLink.status === "active" ? "disabled" : "active";
    try {
      await update.mutateAsync({ status: next } as never);
      toast.success(next === "active" ? "Link enabled" : "Link disabled");
    } catch (err) {
      toastError(err, `Couldn't ${next === "active" ? "enable" : "disable"} the link`);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <Button variant="ghost" size="sm" className="w-fit gap-1.5 text-muted-foreground" onClick={() => navigate("/app/links")}>
        <ArrowLeft className="size-4" /> Back to links
      </Button>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1>
              <button
                onClick={copyShort}
                className="flex items-center gap-1.5 rounded-sm font-mono text-lg font-semibold text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                aria-label={`Copy ${link.shortUrl}`}
                title="Copy short link"
              >
                {link.shortUrl.replace(/^https?:\/\//, "")}
                <Copy className="size-4" aria-hidden="true" />
              </button>
            </h1>
            {link.deletedAt ? (
              <Badge variant="destructive">deleted</Badge>
            ) : (
              <Badge variant={link.status === "active" ? "success" : "secondary"}>{link.status}</Badge>
            )}
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
        {link.deletedAt ? (
          <Button variant="outline" onClick={handleRestore} disabled={restore.isPending} className="gap-1.5">
            <RotateCcw className="size-4" /> Restore
          </Button>
        ) : (
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={() => setEditOpen(true)}>
              Edit
            </Button>
            <Button variant="outline" onClick={toggleStatus} disabled={update.isPending} className="gap-1.5">
              {link.status === "active" ? <Ban className="size-4" /> : <CheckCircle className="size-4" />}
              {link.status === "active" ? "Disable" : "Enable"}
            </Button>
            <Button variant="outline" className="gap-1.5 text-destructive" onClick={handleDelete} disabled={del.isPending}>
              <Trash2 className="size-4" /> Delete
            </Button>
          </div>
        )}
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
