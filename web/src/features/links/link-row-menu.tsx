import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { MoreHorizontal, Pencil, QrCode, BarChart3, Ban, CheckCircle, Trash2, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { confirm } from "@/components/confirm-dialog";
import { useDeleteLink, useUpdateLink, useRestoreLink, usePurgeLink } from "@/features/links/api";
import { useMe } from "@/hooks/use-me";
import { toastError } from "@/lib/apply-server-errors";
import type { Link } from "@/lib/types";

export function LinkRowMenu({
  link,
  onEdit,
  onShowQr,
}: {
  link: Link;
  onEdit: () => void;
  onShowQr: () => void;
}) {
  const navigate = useNavigate();
  const del = useDeleteLink();
  const update = useUpdateLink(link.id);
  const restore = useRestoreLink();
  const purge = usePurgeLink();
  const isAdmin = useMe().data?.role === "admin";

  async function toggleStatus() {
    const next = link.status === "active" ? "disabled" : "active";
    try {
      await update.mutateAsync({ status: next } as never);
      toast.success(next === "active" ? "Link enabled" : "Link disabled");
    } catch (err) {
      toastError(err, `Couldn't ${next === "active" ? "enable" : "disable"} the link`);
    }
  }

  async function handleDelete() {
    try {
      await del.mutateAsync(link.id);
    } catch (err) {
      toastError(err, "Couldn't delete the link");
      return;
    }
    toast("Link deleted", {
      duration: 8000,
      action: { label: "Undo", onClick: () => handleRestore() },
    });
  }

  // mutateAsync rather than mutate(..., callbacks): the row (and this menu)
  // usually unmounts once the delete lands, which would drop the callbacks.
  async function handleRestore() {
    try {
      await restore.mutateAsync(link.id);
      toast.success("Link restored");
    } catch (err) {
      toastError(err, "Couldn't restore the link");
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={`Actions for ${link.code}`}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={() => navigate(`/app/links/${link.id}`)}>
          <BarChart3 className="size-4" /> View stats
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onEdit}>
          <Pencil className="size-4" /> Edit
        </DropdownMenuItem>
        <DropdownMenuItem onClick={onShowQr}>
          <QrCode className="size-4" /> QR code
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {link.deletedAt ? (
          <>
            <DropdownMenuItem onClick={handleRestore}>
              <RotateCcw className="size-4" /> Restore
            </DropdownMenuItem>
            {isAdmin && (
              <DropdownMenuItem
                variant="destructive"
                onClick={async () => {
                  const ok = await confirm({
                    title: `Permanently delete /${link.code}?`,
                    description: "This cannot be undone.",
                    confirmLabel: "Delete permanently",
                    variant: "destructive",
                  });
                  if (!ok) return;
                  try {
                    await purge.mutateAsync(link.id);
                    toast.success("Link permanently deleted");
                  } catch (err) {
                    toastError(err, "Couldn't delete the link");
                  }
                }}
              >
                <Trash2 className="size-4" /> Purge permanently
              </DropdownMenuItem>
            )}
          </>
        ) : (
          <>
            <DropdownMenuItem onClick={toggleStatus}>
              {link.status === "active" ? <Ban className="size-4" /> : <CheckCircle className="size-4" />}
              {link.status === "active" ? "Disable" : "Enable"}
            </DropdownMenuItem>
            <DropdownMenuItem variant="destructive" onClick={handleDelete}>
              <Trash2 className="size-4" /> Delete
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
