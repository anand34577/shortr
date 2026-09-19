import { NavLink, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  Link2,
  Settings,
  ShieldCheck,
  Users,
  FileClock,
  Server,
  KeyRound,
  UserCog,
  BookOpen,
  ChevronsLeft,
  ChevronsRight,
  MapPin,
  Plug,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import type { Me } from "@/lib/types";

const mainNav = [
  { to: "/app", label: "Dashboard", icon: LayoutDashboard, end: true },
  { to: "/app/links", label: "Links", icon: Link2 },
  { to: "/app/tools/ip-lookup", label: "IP Lookup", icon: MapPin },
];

const settingsNav = [
  { to: "/app/settings/profile", label: "Profile", icon: UserCog },
  { to: "/app/settings/security", label: "Security", icon: ShieldCheck },
  { to: "/app/settings/api-keys", label: "API Keys", icon: KeyRound },
  { to: "/app/settings/mcp", label: "MCP", icon: Plug },
  { to: "/app/settings/notifications", label: "Notifications", icon: Settings },
];

const adminNav = [
  { to: "/app/admin/users", label: "Users", icon: Users },
  { to: "/app/admin/settings", label: "Settings", icon: Settings },
  { to: "/app/admin/audit", label: "Audit Log", icon: FileClock },
  { to: "/app/admin/system", label: "System", icon: Server },
];

export function Sidebar({
  me,
  collapsed,
  onToggle,
  onNavigate,
}: {
  me?: Me | null;
  collapsed: boolean;
  onToggle: () => void;
  onNavigate?: () => void;
}) {
  return (
    <nav
      className={cn(
        "flex h-full flex-col gap-1 overflow-y-auto border-r border-sidebar-border bg-sidebar p-3 text-sidebar-foreground transition-[width] duration-200",
        collapsed ? "w-16" : "w-60",
      )}
      aria-label="Primary"
    >
      <div className={cn("flex items-center gap-2 px-2 py-2", collapsed && "justify-center")}>
        <div className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground">
          S
        </div>
        {!collapsed && <span className="text-sm font-semibold">Shortr</span>}
      </div>

      <NavGroup items={mainNav} collapsed={collapsed} onNavigate={onNavigate} />

      <div className={cn("mt-4 px-2 text-xs font-semibold text-muted-foreground", collapsed && "sr-only")}>
        Settings
      </div>
      <NavGroup items={settingsNav} collapsed={collapsed} onNavigate={onNavigate} />

      {me?.role === "admin" && (
        <>
          <div className={cn("mt-4 px-2 text-xs font-semibold text-muted-foreground", collapsed && "sr-only")}>
            Admin
          </div>
          <NavGroup items={adminNav} collapsed={collapsed} onNavigate={onNavigate} />
        </>
      )}

      <div className="mt-auto flex flex-col gap-1 pt-2">
        <NavGroup items={[{ to: "/app/docs", label: "API Docs", icon: BookOpen }]} collapsed={collapsed} onNavigate={onNavigate} />
        <Button
          variant="ghost"
          size="sm"
          className="hidden justify-start gap-2 text-muted-foreground md:flex"
          onClick={onToggle}
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
        >
          {collapsed ? <ChevronsRight className="size-4" /> : <ChevronsLeft className="size-4" />}
          {!collapsed && "Collapse"}
        </Button>
      </div>
    </nav>
  );
}

function NavGroup({
  items,
  collapsed,
  onNavigate,
}: {
  items: { to: string; label: string; icon: React.ComponentType<{ className?: string }>; end?: boolean }[];
  collapsed: boolean;
  onNavigate?: () => void;
}) {
  const { pathname } = useLocation();
  const trimmed = pathname.replace(/\/+$/, "");
  return (
    <div className="flex flex-col gap-0.5">
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          onClick={onNavigate}
          className={({ isActive }) =>
            cn(
              "flex items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground",
              isActive || (item.end && trimmed === item.to) ? "bg-accent text-accent-foreground" : "text-muted-foreground",
              collapsed && "justify-center px-0",
            )
          }
          title={collapsed ? item.label : undefined}
        >
          <item.icon className="size-4 shrink-0" />
          {!collapsed && <span className="truncate">{item.label}</span>}
        </NavLink>
      ))}
    </div>
  );
}
