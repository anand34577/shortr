import { useMutation, useQuery, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { api, buildQuery } from "@/lib/api";
import type { User, UserCreated, AdminSettings, AdminSystemInfo, AuditEntry, Link, Page } from "@/lib/types";
import type { InviteUserInput, AdminSettingsInput } from "@/lib/schemas";

export function useAdminUsers(params: { q?: string; cursor?: string; limit?: number }) {
  return useQuery({
    queryKey: ["admin-users", params],
    queryFn: () => api.get<Page<User>>(`/api/v1/users${buildQuery(params)}`),
    placeholderData: keepPreviousData,
  });
}

export function useAdminUser(id: string | undefined) {
  return useQuery({
    queryKey: ["admin-user", id],
    queryFn: () => api.get<User>(`/api/v1/users/${id}`),
    enabled: !!id,
  });
}

export function useAdminUserLinks(id: string | undefined) {
  return useQuery({
    queryKey: ["admin-user-links", id],
    queryFn: () => api.get<Page<Link>>(`/api/v1/admin/links${buildQuery({ user_id: id })}`),
    enabled: !!id,
  });
}

export function useCreateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: InviteUserInput) => api.post<UserCreated>("/api/v1/users", input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-users"] }),
  });
}

export function useUpdateUser(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<User>) => api.patch<User>(`/api/v1/users/${id}`, input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-users"] });
      qc.invalidateQueries({ queryKey: ["admin-user", id] });
    },
  });
}

export function useDeleteUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/users/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-users"] }),
  });
}

export function useResetUserPassword() {
  return useMutation({
    mutationFn: (id: string) => api.post<{ password: string }>(`/api/v1/users/${id}/reset-password`, {}),
  });
}

export function useRevokeUserSessions() {
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/users/${id}/sessions`),
  });
}

export function useAdminSettings() {
  return useQuery({
    queryKey: ["admin-settings"],
    queryFn: () => api.get<AdminSettings>("/api/v1/settings"),
  });
}

export function useUpdateAdminSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminSettingsInput) => api.put<AdminSettings>("/api/v1/settings", input),
    onSuccess: (data) => qc.setQueryData(["admin-settings"], data),
  });
}

export function useAuditLog(params: { cursor?: string; action?: string }) {
  return useQuery({
    queryKey: ["audit", params],
    queryFn: () => api.get<Page<AuditEntry>>(`/api/v1/audit${buildQuery(params)}`),
    placeholderData: keepPreviousData,
  });
}

export function useAdminSystem() {
  return useQuery({
    queryKey: ["admin-system"],
    queryFn: () => api.get<AdminSystemInfo>("/api/v1/admin/system"),
    refetchInterval: 15_000,
  });
}

export function useTriggerBackup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post("/api/v1/admin/backup", {}),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-system"] }),
  });
}

export function useAdminAllLinks(params: { q?: string; cursor?: string }) {
  return useQuery({
    queryKey: ["admin-all-links", params],
    queryFn: () => api.get<Page<Link>>(`/api/v1/admin/links${buildQuery(params)}`),
    placeholderData: keepPreviousData,
  });
}
