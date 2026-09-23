import { Download } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { LocalTime } from "@/components/local-time";
import { useLinkClicks, linkClicksExportUrl } from "@/features/links/api";
import { countryFlag } from "@/features/links/detail/breakdown-bars";
import { useCursorPager } from "@/hooks/use-cursor-pager";
import { LoadError } from "@/components/load-error";

export function ClicksTab({ linkId }: { linkId: string }) {
  const pager = useCursorPager();
  const { data, isLoading, isError, refetch } = useLinkClicks(linkId, pager.cursor);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-sm">Raw clicks</CardTitle>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" asChild className="gap-1.5">
            <a href={linkClicksExportUrl(linkId, "csv")} download>
              <Download className="size-3.5" /> CSV
            </a>
          </Button>
          <Button variant="outline" size="sm" asChild className="gap-1.5">
            <a href={linkClicksExportUrl(linkId, "json")} download>
              <Download className="size-3.5" /> JSON
            </a>
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-64 w-full" />
        ) : isError ? (
          <LoadError what="clicks" onRetry={() => refetch()} />
        ) : !data?.items.length ? (
          <p className="py-6 text-center text-sm text-muted-foreground">No clicks recorded yet.</p>
        ) : (
          <>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Time</TableHead>
                    <TableHead>Country</TableHead>
                    <TableHead>Device</TableHead>
                    <TableHead>Browser</TableHead>
                    <TableHead>Referrer</TableHead>
                    <TableHead>Bot</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.items.map((c) => (
                    <TableRow key={c.id}>
                      <TableCell className="whitespace-nowrap text-sm">
                        <LocalTime iso={c.ts} />
                      </TableCell>
                      <TableCell>
                        {countryFlag(c.country)} {c.country || "—"}
                      </TableCell>
                      <TableCell className="capitalize">{c.device}</TableCell>
                      <TableCell>{c.browser || "—"}</TableCell>
                      <TableCell className="max-w-40 truncate">{c.referrerHost || "Direct"}</TableCell>
                      <TableCell>{c.isBot && <Badge variant="secondary">Bot</Badge>}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <nav className="mt-3 flex justify-between" aria-label="Pagination">
              <Button variant="outline" size="sm" disabled={!pager.hasPrevious} onClick={pager.previous}>
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={!data.nextCursor}
                onClick={() => data.nextCursor && pager.next(data.nextCursor)}
              >
                Next
              </Button>
            </nav>
          </>
        )}
      </CardContent>
    </Card>
  );
}
