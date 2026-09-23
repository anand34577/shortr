import * as React from "react";
import { FileClock } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { LocalTime } from "@/components/local-time";
import { EmptyState } from "@/components/empty-state";
import { AdminNav } from "@/features/admin/admin-nav";
import { useAuditLog } from "@/features/admin/api";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useCursorPager } from "@/hooks/use-cursor-pager";

export default function AuditPage() {
  const [action, setAction] = React.useState("");
  const debouncedAction = useDebouncedValue(action, 300);
  const pager = useCursorPager();
  const { data, isLoading } = useAuditLog({ cursor: pager.cursor, action: debouncedAction || undefined });

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Admin</h1>
        <AdminNav />
      </div>

      <Input
        value={action}
        onChange={(e) => {
          setAction(e.target.value);
          pager.reset();
        }}
        placeholder="Filter by action (e.g. link.create, user.disable)"
        className="max-w-sm"
        aria-label="Filter by action"
      />

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="p-4">
              <Skeleton className="h-64 w-full" />
            </div>
          ) : !data?.items.length ? (
            <EmptyState
              icon={FileClock}
              title={debouncedAction ? "No entries match that action" : "No audit entries yet"}
              description={debouncedAction ? "Check the spelling, or clear the filter to see everything." : undefined}
            />
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Time</TableHead>
                    <TableHead>Actor</TableHead>
                    <TableHead>Action</TableHead>
                    <TableHead>Target</TableHead>
                    <TableHead>IP</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.items.map((entry) => (
                    <TableRow key={entry.id}>
                      <TableCell className="whitespace-nowrap text-sm">
                        <LocalTime iso={entry.ts} />
                      </TableCell>
                      <TableCell className="text-sm">{entry.actorEmail || entry.actorUserId || "system"}</TableCell>
                      <TableCell>
                        <Badge variant="outline">{entry.action}</Badge>
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {entry.targetType} {entry.targetId?.slice(0, 8)}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">{entry.actorIp}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <nav className="flex justify-between p-3" aria-label="Pagination">
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
    </div>
  );
}
