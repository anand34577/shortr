import { useMutation, useQuery, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { api, buildQuery } from "@/lib/api";
import type { Link, LinkStats, Click, Page } from "@/lib/types";
import type { CreateLinkInput, EditLinkInput } from "@/lib/schemas";

export interface LinkFilters {
  q?: string;
  tag?: string;
  status?: string;
  scope?: "all";
  sort?: "created_at" | "clicks" | "title";
  order?: "asc" | "desc";
  cursor?: string;
  limit?: number;
}

export function useLinks(filters: LinkFilters) {
  return useQuery({
    queryKey: ["links", filters],
    queryFn: () => api.get<Page<Link>>(`/api/v1/links${buildQuery(filters)}`),
    placeholderData: keepPreviousData,
  });
}

export function useLink(id: string | undefined) {
  return useQuery({
    queryKey: ["link", id],
    queryFn: () => api.get<Link>(`/api/v1/links/${id}`),
    enabled: !!id,
  });
}

export function useCreateLink() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<CreateLinkInput>) => api.post<Link>("/api/v1/links", input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["links"] }),
  });
}

export function useUpdateLink(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<EditLinkInput>) => api.patch<Link>(`/api/v1/links/${id}`, input),
    onSuccess: (link) => {
      qc.invalidateQueries({ queryKey: ["links"] });
      qc.setQueryData(["link", id], link);
    },
  });
}

export function useDeleteLink() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/links/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["links"] }),
  });
}

export function useRestoreLink() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/api/v1/links/${id}/restore`, {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["links"] }),
  });
}

export function usePurgeLink() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/api/v1/admin/links/${id}/purge`, {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["links"] }),
  });
}

export interface BulkResult {
  id: string;
  ok: boolean;
  error?: string;
}

export function useBulkLinks() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { ids: string[]; action: "disable" | "enable" | "delete" | "tag"; tag?: string }) =>
      api.post<{ items: BulkResult[] }>("/api/v1/links/bulk", {
        items: input.ids.map((id) => ({ id, action: input.action, tag: input.tag })),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["links"] }),
  });
}

export function useCheckAlias(code: string, enabled: boolean) {
  return useQuery({
    queryKey: ["check-alias", code],
    queryFn: () => api.get<{ available: boolean; reason?: string }>(`/api/v1/links/check${buildQuery({ code })}`),
    enabled: enabled && code.length > 0,
    staleTime: 0,
    retry: false,
  });
}

export function useLinkStats(id: string | undefined, params: { from?: string; to?: string; bucket?: string; tz?: string }) {
  return useQuery({
    queryKey: ["stats", id, params],
    queryFn: () => api.get<LinkStats>(`/api/v1/links/${id}/stats${buildQuery(params)}`),
    enabled: !!id,
  });
}

export function useLinkClicks(id: string | undefined, cursor?: string) {
  return useQuery({
    queryKey: ["link-clicks", id, cursor],
    queryFn: () => api.get<Page<Click>>(`/api/v1/links/${id}/clicks${buildQuery({ cursor, limit: 25 })}`),
    enabled: !!id,
    placeholderData: keepPreviousData,
  });
}

export function usePreviewTitle() {
  return useMutation({
    mutationFn: (url: string) => api.post<{ title: string; finalUrl: string }>("/api/v1/links/preview", { url }),
  });
}

export function linkClicksExportUrl(id: string, format: "csv" | "json" = "csv") {
  return `/api/v1/links/${id}/clicks/export${buildQuery({ format })}`;
}

export function linkQrUrl(id: string, opts: { size?: number; fg?: string; bg?: string; ext?: "png" | "svg" } = {}) {
  const { ext = "png", ...rest } = opts;
  return `/api/v1/links/${id}/qr.${ext}${buildQuery(rest)}`;
}
