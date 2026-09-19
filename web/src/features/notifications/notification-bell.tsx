import { Bell, CheckCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { LocalTime } from "@/components/local-time";
import { useMarkAllNotificationsRead, useMarkNotificationRead, useNotifications } from "@/hooks/use-notifications";
import { cn } from "@/lib/utils";

export function NotificationBell() {
  const { data } = useNotifications();
  const markRead = useMarkNotificationRead();
  const markAllRead = useMarkAllNotificationsRead();

  const items = data?.items ?? [];
  const unreadCount = items.filter((n) => !n.readAt).length;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={`Notifications${unreadCount ? `, ${unreadCount} unread` : ""}`} className="relative">
          <Bell className="size-4" />
          {unreadCount > 0 && (
            <span className="absolute right-1.5 top-1.5 flex size-2 rounded-full bg-destructive" aria-hidden="true" />
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80 p-0">
        <div className="flex items-center justify-between px-4 py-3">
          <span className="text-sm font-semibold">Notifications</span>
          {unreadCount > 0 && (
            <Button variant="ghost" size="sm" className="h-auto gap-1 px-1.5 py-0.5 text-xs" onClick={() => markAllRead.mutate()}>
              <CheckCheck className="size-3.5" /> Mark all read
            </Button>
          )}
        </div>
        <Separator />
        <div className="max-h-96 overflow-y-auto">
          {items.length === 0 ? (
            <p className="px-4 py-8 text-center text-sm text-muted-foreground">You're all caught up.</p>
          ) : (
            items.map((n) => (
              <button
                key={n.id}
                onClick={() => !n.readAt && markRead.mutate(n.id)}
                className={cn(
                  "flex w-full flex-col gap-0.5 border-b border-border px-4 py-3 text-left text-sm last:border-0 hover:bg-accent",
                  !n.readAt && "bg-accent/40",
                )}
              >
                <div className="flex items-center gap-2">
                  {!n.readAt && <span className="size-1.5 shrink-0 rounded-full bg-primary" aria-hidden="true" />}
                  <span className="font-medium">{n.title}</span>
                  {n.priority === "high" && (
                    <Badge variant="destructive" className="ml-auto">
                      Important
                    </Badge>
                  )}
                </div>
                <p className="text-muted-foreground">{n.body}</p>
                <LocalTime iso={n.createdAt} relative className="text-xs text-muted-foreground" />
              </button>
            ))
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
