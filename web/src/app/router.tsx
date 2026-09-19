import * as React from "react";
import { createBrowserRouter, Navigate } from "react-router-dom";
import { FullPageSpinner } from "@/components/full-page-spinner";
import { AppLayout } from "@/app/layout/app-layout";
import { RequireAuth, RequireAdmin, RequireSetup, RequireGuest } from "@/app/guards";

// Auth pages are on the critical path (first paint before login) so they're
// eagerly bundled. Everything behind auth — especially the chart-heavy link
// detail page and the admin section — is route-split (PLAN.md §14.3).
import LoginPage from "@/features/auth/login-page";
import SetupPage from "@/features/auth/setup-page";
import RegisterPage from "@/features/auth/register-page";
import NotFoundPage from "@/features/not-found-page";

function lazyPage(loader: () => Promise<{ default: React.ComponentType }>) {
  const LazyComponent = React.lazy(loader);
  return (
    <React.Suspense fallback={<FullPageSpinner />}>
      <LazyComponent />
    </React.Suspense>
  );
}

export const router = createBrowserRouter([
  { path: "/", element: <Navigate to="/app" replace /> },
  {
    path: "/app",
    children: [
      {
        element: <RequireSetup />,
        children: [{ path: "setup", element: <SetupPage /> }],
      },
      {
        element: <RequireGuest />,
        children: [
          { path: "login", element: <LoginPage /> },
          { path: "register", element: <RegisterPage /> },
        ],
      },
      {
        element: <RequireAuth />,
        children: [
          {
            element: <AppLayout />,
            children: [
              { index: true, element: lazyPage(() => import("@/features/dashboard/dashboard-page")) },
              { path: "links", element: lazyPage(() => import("@/features/links/links-page")) },
              { path: "links/new", element: lazyPage(() => import("@/features/links/new-link-page")) },
              { path: "links/:id", element: lazyPage(() => import("@/features/links/link-detail-page")) },
              { path: "tools/ip-lookup", element: lazyPage(() => import("@/features/tools/ip-lookup-page")) },
              { path: "settings/profile", element: lazyPage(() => import("@/features/settings/profile-page")) },
              { path: "settings/security", element: lazyPage(() => import("@/features/settings/security-page")) },
              { path: "settings/api-keys", element: lazyPage(() => import("@/features/settings/api-keys-page")) },
              { path: "settings/mcp", element: lazyPage(() => import("@/features/settings/mcp-page")) },
              { path: "settings/notifications", element: lazyPage(() => import("@/features/settings/notifications-page")) },
              { path: "docs", element: lazyPage(() => import("@/features/docs/docs-page")) },
              {
                element: <RequireAdmin />,
                children: [
                  { path: "admin/users", element: lazyPage(() => import("@/features/admin/users-page")) },
                  { path: "admin/users/:id", element: lazyPage(() => import("@/features/admin/user-detail-page")) },
                  { path: "admin/settings", element: lazyPage(() => import("@/features/admin/settings-page")) },
                  { path: "admin/audit", element: lazyPage(() => import("@/features/admin/audit-page")) },
                  { path: "admin/system", element: lazyPage(() => import("@/features/admin/system-page")) },
                ],
              },
              { path: "*", element: <NotFoundPage /> },
            ],
          },
        ],
      },
    ],
  },
  { path: "*", element: <NotFoundPage /> },
]);
