import { useQuery } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";

interface OpenApiPath {
  [method: string]: { summary?: string; description?: string };
}
interface OpenApiSpec {
  info?: { title?: string; version?: string; description?: string };
  paths?: Record<string, OpenApiPath>;
}

export default function DocsPage() {
  const { data, isLoading } = useQuery({
    queryKey: ["openapi"],
    queryFn: async () => {
      const res = await fetch("/api/v1/openapi.json");
      if (!res.ok) throw new Error("Not available");
      return (await res.json()) as OpenApiSpec;
    },
    retry: false,
  });

  const entries = data?.paths
    ? Object.entries(data.paths).flatMap(([path, methods]) =>
        Object.entries(methods).map(([method, info]) => ({ path, method, ...info })),
      )
    : [];

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">API Reference</h1>
        <p className="text-sm text-muted-foreground">
          {data?.info?.title || "Shortr API"} {data?.info?.version && `· v${data.info.version}`}
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">OpenAPI specification</CardTitle>
          <CardDescription>Full machine-readable spec for building your own client or generating types.</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="outline" asChild className="gap-1.5">
            <a href="/api/v1/openapi.json" target="_blank" rel="noreferrer">
              View raw OpenAPI JSON <ExternalLink className="size-3.5" />
            </a>
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Endpoints</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className="h-64 w-full" />
          ) : entries.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              OpenAPI spec isn't available yet. See <code className="font-mono">docs/openapi.yaml</code> in the repository, or PLAN.md §13
              for the full endpoint contract.
            </p>
          ) : (
            <ul className="flex flex-col divide-y divide-border">
              {entries.map((e, i) => (
                <li key={i} className="flex items-center gap-3 py-2 text-sm">
                  <Badge variant="outline" className="w-16 shrink-0 justify-center uppercase">
                    {e.method}
                  </Badge>
                  <code className="font-mono text-xs">{e.path}</code>
                  {e.summary && <span className="ml-auto truncate text-muted-foreground">{e.summary}</span>}
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
