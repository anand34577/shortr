import * as React from "react";
import { useQuery } from "@tanstack/react-query";
import { api, buildQuery } from "@/lib/api";
import type { StatsOverview, RecentActivityItem, Page } from "@/lib/types";

export function useStatsOverview(params: { from?: string; to?: string }) {
  return useQuery({
    queryKey: ["stats-overview", params],
    queryFn: () => api.get<StatsOverview>(`/api/v1/stats/overview${buildQuery(params)}`),
  });
}

/** Hook that returns whether the document is currently visible; used to pause polling. */
function useIsVisible() {
  const [visible, setVisible] = React.useState(() => document.visibilityState === "visible");
  React.useEffect(() => {
    const onChange = () => setVisible(document.visibilityState === "visible");
    document.addEventListener("visibilitychange", onChange);
    return () => document.removeEventListener("visibilitychange", onChange);
  }, []);
  return visible;
}

export function useRecentActivity(limit = 25) {
  const visible = useIsVisible();
  return useQuery({
    queryKey: ["stats-recent", limit],
    queryFn: () => api.get<Page<RecentActivityItem>>(`/api/v1/stats/recent${buildQuery({ limit })}`),
    refetchInterval: visible ? 10_000 : false,
  });
}
