import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { api, setCsrfToken, ApiError } from "@/lib/api";
import type { Me } from "@/lib/types";

export const meQueryKey = ["me"] as const;

export function useMe() {
  const query = useQuery({
    queryKey: meQueryKey,
    queryFn: async () => {
      try {
        return await api.get<Me>("/api/v1/me", { skipAuthRedirect: true });
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) return null;
        throw e;
      }
    },
    staleTime: 60_000,
    retry: false,
  });

  useEffect(() => {
    setCsrfToken(query.data?.csrfToken ?? null);
  }, [query.data?.csrfToken]);

  return query;
}

export function useInvalidateMe() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: meQueryKey });
}
