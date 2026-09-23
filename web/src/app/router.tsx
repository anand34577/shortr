import * as React from "react";
import { createBrowserRouter, Navigate, Outlet, useMatches } from "react-router-dom";
import { Loader2 } from "lucide-react";
import { AppLayout } from "@/app/layout/app-layout";
import { RequireAuth, RequireAdmin, RequireSetup, RequireGuest } from "@/app/guards";

// Auth pages are on the critical path (first paint before login) so they're
// eagerly bundled. Everything behind auth — especially the chart-heavy link
// detail page and the admin section — is route-split.
import LoginPage from "@/features/auth/login-page";
import SetupPage from "@/features/auth/setup-page";
import RegisterPage from "@/features/auth/register-page";
import NotFoundPage from "@/features/not-found-page";

function lazyPage(loader: () => Promise<{ default: React.ComponentType }>) {
  const LazyComponent = React.lazy(loader);
  return (
    // Rendered inside the app shell, so it fills the content area rather
    // than the viewport (a full-screen spinner there caused a scrollbar flash).
    <React.Suspense
      fallback={
        <div className="flex min-h-[50vh] items-center justify-center" role="status" aria-label="Loading">
          <Loader2 className="size-6 animate-spin text-muted-foreground" aria-hidden="true" />
        </div>
      }
    >
      <LazyComponent />
    </React.Suspense>
  );
}

/** Route `handle.title` → document.title, so tabs, history and screen readers name each page. */
function RouteTitle() {
  const matches = useMatches();
  const title = [...matches].reverse().find((m) => (m.handle as { title?: string } | undefined)?.title)?.handle as
    | { title: string }
    | undefined;
  React.useEffect(() => {
    document.title = title ? `${title.title} · Shortr` : "Shortr";
  }, [title]);
  return <Outlet />;
}

const t = (title: string) => ({ title });

export const router = createBrowserRouter([
  {
    element: <RouteTitle />,
    children: [
      { path: "/", element: <Navigate to="/app" replace /> },
      {
        path: "/app",
        children: [
          {
            element: <RequireSetup />,
            children: [{ path: "setup", element: <SetupPage />, handle: t("Set up") }],
          },
          {
            element: <RequireGuest />,
            children: [
              { path: "login", element: <LoginPage />, handle: t("Sign in") },
              { path: "register", element: <RegisterPage />, handle: t("Create account") },
            ],
          },
          {
            element: <RequireAuth />,
            children: [
              {
                element: <AppLayout />,
                children: [
                  { index: true, element: lazyPage(() => import("@/features/dashboard/dashboard-page")), handle: t("Dashboard") },
                  { path: "links", element: lazyPage(() => import("@/features/links/links-page")), handle: t("Links") },
                  { path: "links/new", element: lazyPage(() => import("@/features/links/new-link-page")), handle: t("New link") },
                  { path: "links/:id", element: lazyPage(() => import("@/features/links/link-detail-page")), handle: t("Link details") },
                  { path: "tools/ip-lookup", element: lazyPage(() => import("@/features/tools/ip-lookup-page")), handle: t("IP lookup") },
                  { path: "settings/profile", element: lazyPage(() => import("@/features/settings/profile-page")), handle: t("Profile") },
                  { path: "settings/security", element: lazyPage(() => import("@/features/settings/security-page")), handle: t("Security") },
                  { path: "settings/api-keys", element: lazyPage(() => import("@/features/settings/api-keys-page")), handle: t("API keys") },
                  { path: "settings/mcp", element: lazyPage(() => import("@/features/settings/mcp-page")), handle: t("MCP") },
                  {
                    path: "settings/notifications",
                    element: lazyPage(() => import("@/features/settings/notifications-page")),
                    handle: t("Notifications"),
                  },
                  { path: "docs", element: lazyPage(() => import("@/features/docs/docs-page")), handle: t("API docs") },
                  {
                    element: <RequireAdmin />,
                    children: [
                      { path: "admin/users", element: lazyPage(() => import("@/features/admin/users-page")), handle: t("Users") },
                      { path: "admin/users/:id", element: lazyPage(() => import("@/features/admin/user-detail-page")), handle: t("User") },
                      { path: "admin/settings", element: lazyPage(() => import("@/features/admin/settings-page")), handle: t("Admin settings") },
                      { path: "admin/audit", element: lazyPage(() => import("@/features/admin/audit-page")), handle: t("Audit log") },
                      { path: "admin/system", element: lazyPage(() => import("@/features/admin/system-page")), handle: t("System") },
                    ],
                  },
                  { path: "*", element: <NotFoundPage />, handle: t("Not found") },
                ],
              },
            ],
          },
        ],
      },
      { path: "*", element: <NotFoundPage />, handle: t("Not found") },
    ],
  },
]);
