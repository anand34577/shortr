import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { useLinkStats } from "@/features/links/api";
import { rangeToDates } from "@/features/links/detail/range-picker";

export function ReferrersTab({ linkId }: { linkId: string }) {
  const { from, to } = rangeToDates("30d");
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const { data, isLoading } = useLinkStats(linkId, { from, to, bucket: "day", tz });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">Referrers</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-48 w-full" />
        ) : !data?.byReferrer.length ? (
          <p className="py-6 text-center text-sm text-muted-foreground">No referrer data yet.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Referrer</TableHead>
                <TableHead>Clicks</TableHead>
                <TableHead>%</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.byReferrer.map((r) => (
                <TableRow key={r.key}>
                  <TableCell>{r.key || "Direct"}</TableCell>
                  <TableCell>{r.clicks.toLocaleString()}</TableCell>
                  <TableCell>{r.pct.toFixed(1)}%</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
