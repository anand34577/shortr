import * as React from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import {
  type ColumnDef,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import {
  Search,
  Plus,
  Copy,
  Globe,
  ArrowUpDown,
  Ban,
  Trash2,
  Tag as TagIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { LocalTime } from "@/components/local-time";
import { EmptyState } from "@/components/empty-state";
import { useLinks, useBulkLinks, type LinkFilters } from "@/features/links/api";
import { LinkFormDialog } from "@/features/links/link-form-dialog";
import { LinkRowMenu } from "@/features/links/link-row-menu";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { QrPanel } from "@/features/links/qr-panel";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useMe } from "@/hooks/use-me";
import type { Link } from "@/lib/types";

export default function LinksPage() {
  const navigate = useNavigate();
  const [search, setSearch] = React.useState("");
  const debouncedSearch = useDebouncedValue(search, 300);
  const [status, setStatus] = React.useState<string>("all");
  const [tag, setTag] = React.useState("");
  const debouncedTag = useDebouncedValue(tag, 300);
  const [allUsers, setAllUsers] = React.useState(false);
  const [bulkTag, setBulkTag] = React.useState("");
  const isAdmin = useMe().data?.role === "admin";
  const [sort, setSort] = React.useState<"created_at" | "clicks" | "title">("created_at");
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [createOpen, setCreateOpen] = React.useState(false);
  const [editLink, setEditLink] = React.useState<Link | null>(null);
  const [qrLink, setQrLink] = React.useState<Link | null>(null);
  const searchRef = React.useRef<HTMLInputElement>(null);
  const [cursor, setCursor] = React.useState<string | undefined>();
  const [history, setHistory] = React.useState<string[]>([]);

  function resetPaging() {
    setCursor(undefined);
    setHistory([]);
  }

  const filters: LinkFilters = {
    q: debouncedSearch || undefined,
    status: status === "all" ? undefined : status,
    tag: debouncedTag.trim() || undefined,
    scope: isAdmin && allUsers ? "all" : undefined,
    sort,
    order: "desc",
    cursor,
    limit: 25,
  };
  const { data, isLoading, isError, refetch } = useLinks(filters);
  const bulk = useBulkLinks();

  const links = data?.items ?? [];

  const columns = React.useMemo<ColumnDef<Link>[]>(
    () => [
      {
        id: "select",
        header: () => (
          <Checkbox
            checked={links.length > 0 && selected.size === links.length}
            onCheckedChange={(c) => toggleAll(!!c)}
            aria-label="Select all links"
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={selected.has(row.original.id)}
            onCheckedChange={(c) => toggleOne(row.original.id, !!c)}
            aria-label={`Select ${row.original.code}`}
          />
        ),
      },
      {
        id: "short",
        header: "Short link",
        cell: ({ row }) => {
          const link = row.original;
          return (
            <div>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  copyShortUrl(link);
                }}
                className="group flex items-center gap-1.5 font-mono text-sm text-primary"
                title={link.shortUrl}
              >
                {link.shortUrl.replace(/^https?:\/\//, "")}
                <Copy className="size-3.5 opacity-0 group-hover:opacity-100" />
              </button>
              {link.title && <div className="max-w-56 truncate text-xs text-muted-foreground">{link.title}</div>}
            </div>
          );
        },
      },
      {
        id: "target",
        header: "Target",
        cell: ({ row }) => (
          <div className="flex max-w-64 items-center gap-1.5 truncate text-sm text-muted-foreground">
            <Globe className="size-3.5 shrink-0" />
            <span className="truncate">{row.original.targetUrl}</span>
          </div>
        ),
      },
      {
        id: "clicks",
        header: "Clicks",
        cell: ({ row }) => <span className="tabular-nums">{row.original.clickCount.toLocaleString()}</span>,
      },
      {
        id: "status",
        header: "Status",
        cell: ({ row }) => (
          <Badge variant={row.original.status === "active" ? "success" : "secondary"}>{row.original.status}</Badge>
        ),
      },
      {
        id: "tags",
        header: "Tags",
        cell: ({ row }) =>
          row.original.tags?.length ? (
            <div className="flex flex-wrap gap-1">
              {row.original.tags.slice(0, 3).map((t) => (
                <Badge key={t} variant="outline">
                  {t}
                </Badge>
              ))}
            </div>
          ) : (
            <span className="text-xs text-muted-foreground">—</span>
          ),
      },
      {
        id: "created",
        header: "Created",
        cell: ({ row }) => <LocalTime iso={row.original.createdAt} relative />,
      },
      {
        id: "actions",
        header: "",
        cell: ({ row }) => (
          <div onClick={(e) => e.stopPropagation()}>
            <LinkRowMenu link={row.original} onEdit={() => setEditLink(row.original)} onShowQr={() => setQrLink(row.original)} />
          </div>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [links, selected],
  );

  const table = useReactTable({
    data: links,
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const target = e.target as HTMLElement;
      const typing = ["INPUT", "TEXTAREA"].includes(target.tagName);
      if (e.key === "/" && !typing) {
        e.preventDefault();
        searchRef.current?.focus();
      } else if (e.key.toLowerCase() === "c" && !typing) {
        setCreateOpen(true);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  function toggleAll(checked: boolean) {
    setSelected(checked ? new Set(links.map((l) => l.id)) : new Set());
  }
  function toggleOne(id: string, checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  async function copyShortUrl(link: Link) {
    await navigator.clipboard.writeText(link.shortUrl);
    toast.success("Copied to clipboard");
  }

  async function bulkAction(action: "disable" | "enable" | "delete" | "tag", tagName?: string) {
    const ids = Array.from(selected);
    try {
      const res = await bulk.mutateAsync({ ids, action, tag: tagName });
      const failed = res.items.filter((r) => !r.ok).length;
      const done = ids.length - failed;
      const verb = { disable: "disabled", enable: "enabled", delete: "deleted", tag: "tagged" }[action];
      if (done > 0) toast.success(`${done} link${done === 1 ? "" : "s"} ${verb}`);
      if (failed > 0) toast.error(`${failed} link${failed === 1 ? "" : "s"} could not be ${verb}`);
      setSelected(new Set());
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Bulk action failed");
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Links</h1>
          <p className="text-sm text-muted-foreground">{data?.total ?? links.length} links</p>
        </div>
        <Button onClick={() => setCreateOpen(true)} className="gap-2">
          <Plus className="size-4" /> New link
        </Button>
      </div>

      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            ref={searchRef}
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              resetPaging();
            }}
            placeholder="Search code, title, target… (press /)"
            className="pl-9"
            aria-label="Search links"
          />
        </div>
        <Input
          value={tag}
          onChange={(e) => {
            setTag(e.target.value);
            resetPaging();
          }}
          placeholder="Tag"
          className="w-full sm:w-32"
          aria-label="Filter by tag"
        />
        {isAdmin && (
          <Button
            variant={allUsers ? "default" : "outline"}
            onClick={() => {
              setAllUsers((v) => !v);
              resetPaging();
            }}
            aria-pressed={allUsers}
          >
            All users
          </Button>
        )}
        <Select
          value={status}
          onValueChange={(v) => {
            if (v) {
              setStatus(v);
              resetPaging();
            }
          }}
        >
          <SelectTrigger className="w-full sm:w-36">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="disabled">Disabled</SelectItem>
            <SelectItem value="deleted">Deleted</SelectItem>
          </SelectContent>
        </Select>
        <Select
          value={sort}
          onValueChange={(v) => {
            if (v) {
              setSort(v as typeof sort);
              resetPaging();
            }
          }}
        >
          <SelectTrigger className="w-full sm:w-44">
            <ArrowUpDown className="size-3.5" />
            <SelectValue placeholder="Sort" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="created_at">Newest</SelectItem>
            <SelectItem value="clicks">Most clicks</SelectItem>
            <SelectItem value="title">Title</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {selected.size > 0 && (
        <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/40 px-3 py-2 text-sm">
          <span className="font-medium">{selected.size} selected</span>
          <Button variant="outline" size="sm" className="gap-1.5" onClick={() => bulkAction("disable")}>
            <Ban className="size-3.5" /> Disable
          </Button>
          <Button variant="outline" size="sm" onClick={() => bulkAction("enable")}>
            Enable
          </Button>
          <Input
            value={bulkTag}
            onChange={(e) => setBulkTag(e.target.value)}
            placeholder="Add tag"
            className="h-8 w-28"
            aria-label="Tag to add to selected links"
          />
          <Button
            variant="outline"
            size="sm"
            disabled={!bulkTag.trim()}
            onClick={async () => {
              await bulkAction("tag", bulkTag.trim());
              setBulkTag("");
            }}
          >
            Tag
          </Button>
          <Button variant="outline" size="sm" className="gap-1.5 text-destructive" onClick={() => bulkAction("delete")}>
            <Trash2 className="size-3.5" /> Delete
          </Button>
          <Button variant="ghost" size="sm" className="ml-auto" onClick={() => setSelected(new Set())}>
            Clear
          </Button>
        </div>
      )}

      <div className="rounded-xl border border-border">
        {isLoading ? (
          <div className="flex flex-col gap-2 p-4">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        ) : isError ? (
          <div className="flex flex-col items-center gap-3 p-10 text-center">
            <p className="text-sm text-muted-foreground">Couldn't load links.</p>
            <Button variant="outline" onClick={() => refetch()}>
              Retry
            </Button>
          </div>
        ) : links.length === 0 ? (
          <EmptyState
            icon={TagIcon}
            title={debouncedSearch ? "No links match your search" : "No links yet"}
            description={debouncedSearch ? "Try a different search term or clear filters." : "Create your first short link to get started."}
            action={!debouncedSearch ? { label: "New link", onClick: () => setCreateOpen(true) } : undefined}
          />
        ) : (
          <>
            {/* Desktop table */}
            <div className="hidden sm:block">
              <Table>
                <TableHeader>
                  {table.getHeaderGroups().map((hg) => (
                    <TableRow key={hg.id}>
                      {hg.headers.map((header) => (
                        <TableHead key={header.id}>
                          {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                        </TableHead>
                      ))}
                    </TableRow>
                  ))}
                </TableHeader>
                <TableBody>
                  {table.getRowModel().rows.map((row) => (
                    <TableRow
                      key={row.id}
                      className="cursor-pointer"
                      onClick={() => navigate(`/app/links/${row.original.id}`)}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
                      ))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>

            {/* Mobile cards */}
            <div className="flex flex-col divide-y divide-border sm:hidden">
              {links.map((link) => (
                <button
                  key={link.id}
                  onClick={() => navigate(`/app/links/${link.id}`)}
                  className="flex flex-col gap-1 p-4 text-left"
                >
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-sm text-primary">{link.code}</span>
                    <Badge variant={link.status === "active" ? "success" : "secondary"}>{link.status}</Badge>
                  </div>
                  <span className="truncate text-sm text-muted-foreground">{link.targetUrl}</span>
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span>{link.clickCount.toLocaleString()} clicks</span>
                    <LocalTime iso={link.createdAt} relative />
                  </div>
                </button>
              ))}
            </div>
          </>
        )}
      </div>

      {(history.length > 0 || data?.nextCursor) && (
        <div className="flex justify-between">
          <Button
            variant="outline"
            size="sm"
            disabled={history.length === 0}
            onClick={() => {
              const h = [...history];
              const prev = h.pop();
              setHistory(h);
              setCursor(prev);
            }}
          >
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!data?.nextCursor}
            onClick={() => {
              if (cursor) setHistory((h) => [...h, cursor]);
              setCursor(data?.nextCursor ?? undefined);
            }}
          >
            Next
          </Button>
        </div>
      )}

      <LinkFormDialog open={createOpen} onOpenChange={setCreateOpen} />
      <LinkFormDialog open={!!editLink} onOpenChange={(o) => !o && setEditLink(null)} link={editLink ?? undefined} />

      <Dialog open={!!qrLink} onOpenChange={(o) => !o && setQrLink(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>QR code</DialogTitle>
          </DialogHeader>
          {qrLink && <QrPanel link={qrLink} />}
        </DialogContent>
      </Dialog>
    </div>
  );
}
