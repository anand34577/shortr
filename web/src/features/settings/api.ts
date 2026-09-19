import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { meQueryKey } from "@/hooks/use-me";
import type { Me, SessionInfo, OIDCIdentity, ApiKey, ApiKeyCreated, NotificationPreferences } from "@/lib/types";
import type { ProfileInput, ChangePasswordInput, CreateApiKeyInput } from "@/lib/schemas";

export function useDeleteAccount() {
  return useMutation({
    mutationFn: (links: "keep" | "delete") => api.delete(`/api/v1/me?links=${links}`),
  });
}

export function useUpdateProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<ProfileInput>) => api.patch<Me>("/api/v1/me", input),
    onSuccess: (me) => qc.setQueryData(meQueryKey, (old: Me | null | undefined) => (old ? { ...old, ...me } : me)),
  });
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (input: ChangePasswordInput) =>
      api.put("/api/v1/me/password", { current: input.current || undefined, new: input.next }),
  });
}

export function useSessions() {
  return useQuery({
    queryKey: ["sessions"],
    queryFn: () => api.get<{ items: SessionInfo[] }>("/api/v1/me/sessions"),
  });
}

export function useRevokeSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/me/sessions/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["sessions"] }),
  });
}

export function useIdentities() {
  return useQuery({
    queryKey: ["identities"],
    queryFn: () => api.get<{ items: OIDCIdentity[] }>("/api/v1/me/identities"),
  });
}

export function useUnlinkIdentity() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/me/identities/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["identities"] }),
  });
}

export function useApiKeys() {
  return useQuery({
    queryKey: ["apikeys"],
    queryFn: () => api.get<{ items: ApiKey[] }>("/api/v1/apikeys"),
  });
}

export function useCreateApiKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateApiKeyInput) => api.post<ApiKeyCreated>("/api/v1/apikeys", input),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["apikeys"] }),
  });
}

export function useRevokeApiKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api/v1/apikeys/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["apikeys"] }),
  });
}

export function useNotificationPreferences() {
  return useQuery({
    queryKey: ["notification-preferences"],
    queryFn: () => api.get<NotificationPreferences>("/api/v1/notifications/preferences"),
  });
}

export function useUpdateNotificationPreferences() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<NotificationPreferences>) => api.put<NotificationPreferences>("/api/v1/notifications/preferences", input),
    onSuccess: (data) => qc.setQueryData(["notification-preferences"], data),
  });
}
