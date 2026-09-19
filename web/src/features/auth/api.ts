import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { api } from "@/lib/api";
import type { Me } from "@/lib/types";
import type { LoginInput, SetupInput } from "@/lib/schemas";
import { meQueryKey } from "@/hooks/use-me";

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: LoginInput) => api.post<Me>("/auth/login", input, { skipAuthRedirect: true }),
    onSuccess: (me) => {
      qc.setQueryData(meQueryKey, me);
      qc.invalidateQueries({ queryKey: ["auth-status"] });
    },
  });
}

export function useSetup() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: SetupInput) => api.post<Me>("/auth/setup", input, { skipAuthRedirect: true }),
    onSuccess: (me) => {
      qc.setQueryData(meQueryKey, me);
      qc.invalidateQueries({ queryKey: ["auth-status"] });
    },
  });
}

export function useLogout() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  return useMutation({
    mutationFn: () => api.post("/auth/logout", {}),
    onSettled: () => {
      qc.setQueryData(meQueryKey, null);
      qc.clear();
      navigate("/app/login");
      toast.success("Signed out");
    },
  });
}

export function useRegister() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { email: string; name: string; password: string }) =>
      api.post<Me>("/auth/register", input, { skipAuthRedirect: true }),
    onSuccess: (me) => qc.setQueryData(meQueryKey, me),
  });
}

export function oidcStartUrl(next?: string) {
  const qs = next ? `?next=${encodeURIComponent(next)}` : "";
  return `/auth/oidc/start${qs}`;
}
