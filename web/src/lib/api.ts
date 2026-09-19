import type { ApiErrorBody } from "./types";

/**
 * Typed fetch wrapper for the Shortr API (PLAN.md §13.1).
 *
 * - Success responses are returned as-is (either the resource or a
 *   `{items,next_cursor,total}` page envelope).
 * - Error responses are the `{error:{code,message,fields,requestId}}`
 *   envelope; they are thrown as `ApiError`.
 * - Mutating requests get `X-CSRF-Token` from the cached `/api/v1/me`
 *   response (session-cookie auth). Bearer/API-key auth skips CSRF
 *   (not used by this browser app, but the helper stays agnostic).
 * - A 401 redirects to `/app/login?next=<path>` (except for the
 *   `/auth/*` and `/api/v1/me` calls that are used to *determine*
 *   auth state, to avoid redirect loops).
 */

export class ApiError extends Error {
  code: string;
  fields?: Record<string, string>;
  requestId?: string;
  status: number;

  constructor(status: number, body: ApiErrorBody["error"]) {
    super(body.message || body.code);
    this.name = "ApiError";
    this.status = status;
    this.code = body.code;
    this.fields = body.fields;
    this.requestId = body.requestId;
  }
}

let csrfToken: string | null = null;
export function setCsrfToken(token: string | null) {
  csrfToken = token;
}
export function getCsrfToken() {
  return csrfToken;
}

type SudoPrompt = () => Promise<string | null>;
let sudoPrompt: SudoPrompt | null = null;
/** Registers the UI that asks the user to re-enter their password (SUDO_REQUIRED). */
export function setSudoPrompt(fn: SudoPrompt | null) {
  sudoPrompt = fn;
}

const MUTATING = new Set(["POST", "PUT", "PATCH", "DELETE"]);

function isAuthBootstrapPath(path: string) {
  return (
    path.startsWith("/auth/") ||
    path === "/api/v1/me" ||
    path.startsWith("/api/v1/me?")
  );
}

export interface ApiFetchInit extends Omit<RequestInit, "body"> {
  body?: unknown;
  /** Skip the automatic 401 -> /app/login redirect (used for probing auth state). */
  skipAuthRedirect?: boolean;
  /** Raw body (e.g. FormData); bypasses JSON encoding. */
  rawBody?: BodyInit;
  /** Internal: set on the automatic retry after a successful re-authentication. */
  sudoRetried?: boolean;
}

export async function apiFetch<T>(path: string, init: ApiFetchInit = {}): Promise<T> {
  const method = (init.method || "GET").toUpperCase();
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");

  let body: BodyInit | undefined;
  if (init.rawBody !== undefined) {
    body = init.rawBody;
  } else if (init.body !== undefined) {
    headers.set("Content-Type", "application/json; charset=utf-8");
    body = JSON.stringify(init.body);
  }

  if (MUTATING.has(method) && csrfToken) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const res = await fetch(path, {
    ...init,
    method,
    headers,
    body,
    credentials: "same-origin",
  });

  if (res.status === 204) {
    return undefined as T;
  }

  const contentType = res.headers.get("content-type") || "";
  const isJson = contentType.includes("application/json");
  const data = isJson ? await res.json().catch(() => null) : null;

  if (!res.ok) {
    if (res.status === 401 && !init.skipAuthRedirect && !isAuthBootstrapPath(path)) {
      const next = encodeURIComponent(location.pathname + location.search);
      location.href = `/app/login?next=${next}`;
    }
    if (res.status === 403 && data?.error?.code === "SUDO_REQUIRED" && sudoPrompt && !init.sudoRetried) {
      const password = await sudoPrompt();
      if (password !== null) {
        await apiFetch<void>("/auth/sudo", { method: "POST", body: { password }, skipAuthRedirect: true });
        return apiFetch<T>(path, { ...init, sudoRetried: true });
      }
    }
    if (data && data.error) {
      throw new ApiError(res.status, data.error);
    }
    throw new ApiError(res.status, {
      code: "INTERNAL",
      message: `Request failed with status ${res.status}`,
    });
  }

  return data as T;
}

export function buildQuery(params: object): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params) as [string, unknown][]) {
    if (v === undefined || v === null || v === "") continue;
    if (Array.isArray(v)) {
      for (const item of v) sp.append(k, String(item));
    } else {
      sp.set(k, String(v));
    }
  }
  const qs = sp.toString();
  return qs ? `?${qs}` : "";
}

export const api = {
  get: <T>(path: string, init?: ApiFetchInit) => apiFetch<T>(path, { ...init, method: "GET" }),
  post: <T>(path: string, body?: unknown, init?: ApiFetchInit) =>
    apiFetch<T>(path, { ...init, method: "POST", body }),
  patch: <T>(path: string, body?: unknown, init?: ApiFetchInit) =>
    apiFetch<T>(path, { ...init, method: "PATCH", body }),
  put: <T>(path: string, body?: unknown, init?: ApiFetchInit) =>
    apiFetch<T>(path, { ...init, method: "PUT", body }),
  delete: <T>(path: string, init?: ApiFetchInit) => apiFetch<T>(path, { ...init, method: "DELETE" }),
};
