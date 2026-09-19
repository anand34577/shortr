import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { AuthStatus } from "@/lib/types";

export function useAuthStatus() {
  return useQuery({
    queryKey: ["auth-status"],
    queryFn: () => api.get<AuthStatus>("/auth/status", { skipAuthRedirect: true }),
    staleTime: 30_000,
    retry: false,
  });
}
