import * as React from "react";
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useLinkStats } from "@/features/links/api";
import { rangeToDates } from "@/features/links/detail/range-picker";
import { BreakdownBars, countryFlag } from "@/features/links/detail/breakdown-bars";

const DEVICE_COLORS = ["var(--color-primary)", "var(--color-success)", "var(--color-warning)", "var(--color-muted-foreground)"];

export function AudienceTab({ linkId }: { linkId: string }) {
  const { from, to } = rangeToDates("30d");
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const { data, isLoading } = useLinkStats(linkId, { from, to, bucket: "day", tz });
  const [tableView, setTableView] = React.useState(false);

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Countries</CardTitle>
        </CardHeader>
        <CardContent>
          <BreakdownBars items={data?.byCountry} loading={isLoading} labelPrefix={(k) => `${countryFlag(k)} ${k || "Unknown"}`} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-sm">Devices</CardTitle>
          <Button variant="ghost" size="sm" onClick={() => setTableView((v) => !v)}>
            {tableView ? "Show chart" : "Show as table"}
          </Button>
        </CardHeader>
        <CardContent>
          {tableView || !data?.byDevice.length ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Device</TableHead>
                  <TableHead>Clicks</TableHead>
                  <TableHead>%</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(data?.byDevice ?? []).map((d) => (
                  <TableRow key={d.key}>
                    <TableCell className="capitalize">{d.key || "Unknown"}</TableCell>
                    <TableCell>{d.clicks.toLocaleString()}</TableCell>
                    <TableCell>{d.pct.toFixed(1)}%</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <ResponsiveContainer width="100%" height={220}>
              <PieChart>
                <Pie data={data.byDevice} dataKey="clicks" nameKey="key" innerRadius={50} outerRadius={80} paddingAngle={2}>
                  {data.byDevice.map((_, i) => (
                    <Cell key={i} fill={DEVICE_COLORS[i % DEVICE_COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{
                    background: "var(--color-popover)",
                    border: "1px solid var(--color-border)",
                    borderRadius: 8,
                    fontSize: 12,
                  }}
                />
              </PieChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Operating system</CardTitle>
        </CardHeader>
        <CardContent>
          <BreakdownBars items={data?.byOS} loading={isLoading} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Browser</CardTitle>
        </CardHeader>
        <CardContent>
          <BreakdownBars items={data?.byBrowser} loading={isLoading} />
        </CardContent>
      </Card>
    </div>
  );
}
