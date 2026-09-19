import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useMe } from "@/hooks/use-me";
import { useAuthStatus } from "@/hooks/use-auth-status";
import { FullPageSpinner } from "@/components/full-page-spinner";

export function RequireSetup() {
  const status = useAuthStatus();
  if (status.isLoading) return <FullPageSpinner />;
  if (status.data?.setupRequired) return <Outlet />;
  return <Navigate to="/app/login" replace />;
}

export function RequireAuth() {
  const me = useMe();
  const status = useAuthStatus();
  const location = useLocation();

  if (me.isLoading || status.isLoading) return <FullPageSpinner />;

  if (status.data?.setupRequired) {
    return <Navigate to="/app/setup" replace />;
  }

  if (!me.data) {
    const next = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/app/login?next=${next}`} replace />;
  }

  return <Outlet />;
}

export function RequireGuest() {
  const me = useMe();
  const status = useAuthStatus();
  if (me.isLoading || status.isLoading) return <FullPageSpinner />;
  if (status.data?.setupRequired) return <Navigate to="/app/setup" replace />;
  if (me.data) return <Navigate to="/app" replace />;
  return <Outlet />;
}

export function RequireAdmin() {
  const me = useMe();
  if (me.isLoading) return <FullPageSpinner />;
  if (!me.data) return <Navigate to="/app/login" replace />;
  if (me.data.role !== "admin") return <Navigate to="/app" replace />;
  return <Outlet />;
}
