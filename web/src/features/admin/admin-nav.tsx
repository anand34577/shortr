import { NavLink } from "react-router-dom";
import { cn } from "@/lib/utils";

const items = [
  { to: "/app/admin/users", label: "Users" },
  { to: "/app/admin/settings", label: "Settings" },
  { to: "/app/admin/audit", label: "Audit Log" },
  { to: "/app/admin/system", label: "System" },
];

export function AdminNav() {
  return (
    <nav className="flex gap-1 overflow-x-auto border-b border-border pb-px" aria-label="Admin">
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
