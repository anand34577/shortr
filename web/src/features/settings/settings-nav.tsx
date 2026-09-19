import { NavLink } from "react-router-dom";
import { cn } from "@/lib/utils";

const items = [
  { to: "/app/settings/profile", label: "Profile" },
  { to: "/app/settings/security", label: "Security" },
  { to: "/app/settings/api-keys", label: "API Keys" },
  { to: "/app/settings/mcp", label: "MCP" },
  { to: "/app/settings/notifications", label: "Notifications" },
];

export function SettingsNav() {
  return (
    <nav className="flex gap-1 overflow-x-auto border-b border-border pb-px" aria-label="Settings">
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          className={({ isActive }) =>
            cn(
              "whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium",
              isActive ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground",
            )
          }
        >
          {item.label}
        </NavLink>
      ))}
    </nav>
  );
}
