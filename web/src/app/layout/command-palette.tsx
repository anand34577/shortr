import * as React from "react";
import { useNavigate } from "react-router-dom";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import {
  LayoutDashboard,
  Link2,
  Plus,
  Settings,
  ShieldCheck,
  KeyRound,
  Users,
  Server,
  Search,
} from "lucide-react";
import { cn } from "@/lib/utils";

interface Command {
  id: string;
  label: string;
  hint?: string;
  icon: React.ComponentType<{ className?: string }>;
  action: (navigate: ReturnType<typeof useNavigate>) => void;
  adminOnly?: boolean;
}

const commands: Command[] = [
  { id: "dashboard", label: "Go to Dashboard", icon: LayoutDashboard, action: (n) => n("/app") },
  { id: "links", label: "Go to Links", icon: Link2, action: (n) => n("/app/links") },
  { id: "new-link", label: "Create new link", hint: "N", icon: Plus, action: (n) => n("/app/links/new") },
  { id: "settings", label: "Profile settings", icon: Settings, action: (n) => n("/app/settings/profile") },
  { id: "security", label: "Security settings", icon: ShieldCheck, action: (n) => n("/app/settings/security") },
  { id: "keys", label: "API keys", icon: KeyRound, action: (n) => n("/app/settings/api-keys") },
  { id: "admin-users", label: "Admin: Users", icon: Users, action: (n) => n("/app/admin/users"), adminOnly: true },
  { id: "admin-system", label: "Admin: System", icon: Server, action: (n) => n("/app/admin/system"), adminOnly: true },
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

  const filtered = React.useMemo(
    () =>
      commands
        .filter((c) => isAdmin || !c.adminOnly)
        .filter((c) => c.label.toLowerCase().includes(query.toLowerCase())),
    [query, isAdmin],
  );

  React.useEffect(() => {
    if (open) {
      setQuery("");
      setActiveIndex(0);
    }
  }, [open]);

  function run(cmd: Command) {
    cmd.action(navigate);
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent hideClose className="top-[20%] max-w-lg translate-y-0 p-0" aria-label="Command palette">
        <div className="flex items-center gap-2 border-b border-border px-4 py-3">
          <Search className="size-4 text-muted-foreground" />
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") {
                e.preventDefault();
                setActiveIndex((i) => Math.min(i + 1, filtered.length - 1));
              } else if (e.key === "ArrowUp") {
                e.preventDefault();
                setActiveIndex((i) => Math.max(i - 1, 0));
              } else if (e.key === "Enter" && filtered[activeIndex]) {
                run(filtered[activeIndex]);
              }
            }}
            placeholder="Type a command or search..."
            className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            aria-label="Command search"
          />
          <kbd className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground">Esc</kbd>
        </div>
        <div className="max-h-80 overflow-y-auto p-2">
          {filtered.length === 0 && (
            <p className="px-2 py-6 text-center text-sm text-muted-foreground">No matching commands.</p>
          )}
          {filtered.map((cmd, i) => (
            <button
              key={cmd.id}
              onClick={() => run(cmd)}
              onMouseEnter={() => setActiveIndex(i)}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm",
                i === activeIndex ? "bg-accent text-accent-foreground" : "text-foreground",
              )}
            >
              <cmd.icon className="size-4 text-muted-foreground" />
              <span>{cmd.label}</span>
              {cmd.hint && <kbd className="ml-auto text-xs text-muted-foreground">{cmd.hint}</kbd>}
            </button>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}
