import { useMutation } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { IPLocation } from "@/lib/types";

export function useIPLookup() {
  return useMutation({
    mutationFn: (ip: string) => api.get<IPLocation>(`/api/v1/tools/ip-lookup/${encodeURIComponent(ip)}`),
  });
}
