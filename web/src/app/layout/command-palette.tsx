import * as React from "react";
import { useNavigate } from "react-router-dom";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import {
  Bell,
  BookOpen,
  FileClock,
  LayoutDashboard,
  Link2,
  MapPin,
  Plug,
  Plus,
  Settings,
  ShieldCheck,
  KeyRound,
  UserCog,
  Users,
  Server,
  Search,
} from "lucide-react";
import { cn } from "@/lib/utils";

interface Command {
  id: string;
  label: string;
  /** extra words to match on, so "password" finds Security */
  keywords?: string;
  icon: React.ComponentType<{ className?: string }>;
  to: string;
  adminOnly?: boolean;
}

const commands: Command[] = [
  { id: "new-link", label: "Create new link", keywords: "shorten add", icon: Plus, to: "/app/links/new" },
  { id: "dashboard", label: "Go to Dashboard", keywords: "home stats", icon: LayoutDashboard, to: "/app" },
  { id: "links", label: "Go to Links", icon: Link2, to: "/app/links" },
  { id: "ip-lookup", label: "IP lookup", keywords: "geo location", icon: MapPin, to: "/app/tools/ip-lookup" },
  { id: "profile", label: "Profile settings", keywords: "name email account", icon: UserCog, to: "/app/settings/profile" },
  { id: "security", label: "Security settings", keywords: "password sessions sso", icon: ShieldCheck, to: "/app/settings/security" },
  { id: "keys", label: "API keys", keywords: "token", icon: KeyRound, to: "/app/settings/api-keys" },
  { id: "mcp", label: "MCP server", keywords: "ai assistant claude", icon: Plug, to: "/app/settings/mcp" },
  { id: "notifications", label: "Notification settings", keywords: "email gotify", icon: Bell, to: "/app/settings/notifications" },
  { id: "docs", label: "API docs", keywords: "openapi reference", icon: BookOpen, to: "/app/docs" },
  { id: "admin-users", label: "Admin: Users", icon: Users, to: "/app/admin/users", adminOnly: true },
  { id: "admin-settings", label: "Admin: Settings", keywords: "registration oidc smtp", icon: Settings, to: "/app/admin/settings", adminOnly: true },
  { id: "admin-audit", label: "Admin: Audit log", icon: FileClock, to: "/app/admin/audit", adminOnly: true },
  { id: "admin-system", label: "Admin: System", keywords: "backup version health", icon: Server, to: "/app/admin/system", adminOnly: true },
];

export function CommandPalette({
  open,
  onOpenChange,
  isAdmin,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  isAdmin: boolean;
}) {
  const navigate = useNavigate();
  const [query, setQuery] = React.useState("");
  const [activeIndex, setActiveIndex] = React.useState(0);
  const listRef = React.useRef<HTMLDivElement>(null);

  const filtered = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    return commands
      .filter((c) => isAdmin || !c.adminOnly)
      .filter((c) => !q || `${c.label} ${c.keywords ?? ""}`.toLowerCase().includes(q));
  }, [query, isAdmin]);

  React.useEffect(() => {
    if (open) {
      setQuery("");
      setActiveIndex(0);
    }
  }, [open]);

  React.useEffect(() => {
    listRef.current?.querySelector(`[data-index="${activeIndex}"]`)?.scrollIntoView({ block: "nearest" });
  }, [activeIndex]);

  function run(cmd: Command) {
    navigate(cmd.to);
    onOpenChange(false);
  }

  const active = filtered[activeIndex];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent hideClose className="top-[20%] max-w-lg translate-y-0 gap-0 p-0" aria-describedby={undefined}>
        <DialogTitle className="sr-only">Command palette</DialogTitle>
        <div className="flex items-center gap-2 border-b border-border px-4 py-3">
          <Search className="size-4 text-muted-foreground" aria-hidden="true" />
          <input
            autoFocus
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActiveIndex(0);
            }}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") {
                e.preventDefault();
                setActiveIndex((i) => (filtered.length ? (i + 1) % filtered.length : 0));
              } else if (e.key === "ArrowUp") {
                e.preventDefault();
                setActiveIndex((i) => (filtered.length ? (i - 1 + filtered.length) % filtered.length : 0));
              } else if (e.key === "Enter" && active) {
                e.preventDefault();
                run(active);
              }
            }}
            placeholder="Where do you want to go?"
            className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            role="combobox"
            aria-expanded="true"
            aria-controls="command-list"
            aria-activedescendant={active ? `command-${active.id}` : undefined}
            aria-label="Search commands"
            autoComplete="off"
          />
          <kbd className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground">Esc</kbd>
        </div>
        <div ref={listRef} id="command-list" role="listbox" aria-label="Commands" className="max-h-80 overflow-y-auto p-2">
          {filtered.length === 0 && (
            <p className="px-2 py-6 text-center text-sm text-muted-foreground">No matching commands.</p>
          )}
          {filtered.map((cmd, i) => (
            <div
              key={cmd.id}
              id={`command-${cmd.id}`}
              data-index={i}
              role="option"
              aria-selected={i === activeIndex}
              onClick={() => run(cmd)}
              onMouseMove={() => setActiveIndex(i)}
              className={cn(
                "flex w-full cursor-pointer items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm",
                i === activeIndex ? "bg-accent text-accent-foreground" : "text-foreground",
              )}
            >
              <cmd.icon className="size-4 text-muted-foreground" />
              <span>{cmd.label}</span>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
