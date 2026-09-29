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

/** Public short-link origin (SHORTR_BASE_URL). In split mode the console runs
 *  on a private address, so never build public URLs from window.location. */
export function usePublicOrigin(): string {
  const { data } = useAuthStatus();
  return data?.baseUrl || window.location.origin;
}
