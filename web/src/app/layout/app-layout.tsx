import * as React from "react";
import { Outlet } from "react-router-dom";
import { Sidebar } from "./sidebar";
import { Topbar } from "./topbar";
import { MobileTabBar } from "./mobile-tab-bar";
import { CommandPalette } from "./command-palette";
import { useMe } from "@/hooks/use-me";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { OfflineBanner } from "@/components/offline-banner";

export function AppLayout() {
  const me = useMe();
  const [collapsed, setCollapsed] = React.useState(() => {
    try {
      return localStorage.getItem("shortr-sidebar-collapsed") === "1";
    } catch {
      return false;
    }
  });
  const [mobileOpen, setMobileOpen] = React.useState(false);
  const [paletteOpen, setPaletteOpen] = React.useState(false);

  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen((o) => !o);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  function toggleCollapsed() {
    setCollapsed((c) => {
      const next = !c;
      try {
        localStorage.setItem("shortr-sidebar-collapsed", next ? "1" : "0");
      } catch {
        /* ignore */
      }
      return next;
    });
  }

  return (
    <div className="flex h-dvh w-full overflow-hidden bg-background">
      <a
        href="#main"
        className="sr-only z-50 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground focus:not-sr-only focus:fixed focus:left-3 focus:top-3"
      >
        Skip to content
      </a>
      <div className="hidden md:block">
        <Sidebar me={me.data} collapsed={collapsed} onToggle={toggleCollapsed} />
      </div>

      <Dialog open={mobileOpen} onOpenChange={setMobileOpen}>
        <DialogContent
          hideClose
          aria-describedby={undefined}
          className="left-0 top-0 h-full max-h-none w-64 max-w-[80vw] translate-x-0 translate-y-0 rounded-none border-r p-0 data-[state=closed]:zoom-out-100 data-[state=open]:zoom-in-100 data-[state=closed]:slide-out-to-left data-[state=open]:slide-in-from-left"
        >
          <DialogTitle className="sr-only">Navigation</DialogTitle>
          <Sidebar me={me.data} collapsed={false} onToggle={() => {}} onNavigate={() => setMobileOpen(false)} />
        </DialogContent>
      </Dialog>

      <div className="flex min-w-0 flex-1 flex-col">
        <OfflineBanner />
        <Topbar me={me.data} onOpenSidebar={() => setMobileOpen(true)} onOpenCommandPalette={() => setPaletteOpen(true)} />
        <main id="main" tabIndex={-1} className="flex-1 overflow-y-auto pb-16 focus:outline-none sm:pb-0">
          <div className="mx-auto w-full max-w-screen-2xl px-4 py-6 sm:px-6 lg:px-8">
            <Outlet />
          </div>
        </main>
        <MobileTabBar />
      </div>

      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} isAdmin={me.data?.role === "admin"} />
    </div>
  );
}
