import * as React from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Notification, Page } from "@/lib/types";
import { useMe } from "@/hooks/use-me";

const POLL_MS = 20_000;
const seenIds = new Set<string>();
let primed = false;

export function useNotifications() {
  const me = useMe();
  const enabled = !!me.data;

  const query = useQuery({
    queryKey: ["notifications"],
    queryFn: () => api.get<Page<Notification>>("/api/v1/notifications?limit=20"),
    enabled,
    refetchInterval: POLL_MS,
    refetchIntervalInBackground: true,
  });

  React.useEffect(() => {
    const items = query.data?.items;
    if (!items) return;

    if (!primed) {
      // First load after mount: mark everything as already-seen so we don't
      // spam browser notifications for history on page load.
      for (const n of items) seenIds.add(n.id);
      primed = true;
      return;
    }

    for (const n of items) {
      if (seenIds.has(n.id)) continue;
      seenIds.add(n.id);
      if (n.priority === "high") {
        notifyBrowser(n);
      }
    }
  }, [query.data]);

  return query;
}

function notifyBrowser(n: Notification) {
  if (typeof Notification === "undefined") return;
  if (Notification.permission !== "granted") return;
  // Only surface a native OS notification when the user isn't already
  // looking at the tab (visible tabs show the in-app bell + toast instead).
  if (document.visibilityState === "visible") return;
  try {
    const browserNotif = new Notification(n.title, {
      body: n.body,
      tag: n.id,
    });
    browserNotif.onclick = () => {
      window.focus();
      browserNotif.close();
    };
  } catch {
    /* ignore */
  }
}

export function useRequestNotificationPermission() {
  return React.useCallback(async () => {
    if (typeof Notification === "undefined") return "unsupported" as const;
    if (Notification.permission === "granted") return "granted" as const;
    if (Notification.permission === "denied") return "denied" as const;
    return Notification.requestPermission();
  }, []);
}

export function useMarkNotificationRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.patch(`/api/v1/notifications/${id}/read`, {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}

export function useMarkAllNotificationsRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post(`/api/v1/notifications/read-all`, {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }),
  });
}
