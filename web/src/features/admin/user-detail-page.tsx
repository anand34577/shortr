import * as React from "react";
import { useParams, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { ArrowLeft, KeyRound, LogOut, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, linkRowProps } from "@/components/ui/table";
import { LocalTime } from "@/components/local-time";
import { copyText } from "@/lib/clipboard";
import { toastError } from "@/lib/apply-server-errors";
import { confirm as confirmDialog } from "@/components/confirm-dialog";
import {
  useAdminUser,
  useAdminUserLinks,
  useUpdateUser,
  useDeleteUser,
  useResetUserPassword,
  useRevokeUserSessions,
} from "@/features/admin/api";

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: user, isLoading, isError, refetch } = useAdminUser(id);
  const { data: links } = useAdminUserLinks(id);
  const update = useUpdateUser(id ?? "");
  const del = useDeleteUser();
  const resetPassword = useResetUserPassword();
  const revokeSessions = useRevokeUserSessions();

  if (isLoading) {
    return (
      <div role="status" aria-label="Loading user">
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (isError || !user) {
    return (
      <div className="flex flex-col items-center gap-4 py-16 text-center">
        <h1 className="text-lg font-semibold">Couldn’t load this user</h1>
        <p className="max-w-md text-sm text-muted-foreground">The user may have been deleted or the request failed.</p>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => refetch()}>Retry</Button>
          <Button onClick={() => navigate("/app/admin/users")}>Back to users</Button>
        </div>
      </div>
    );
  }

  async function toggleStatus() {
    const next = user!.status === "active" ? "disabled" : "active";
    try {
      await update.mutateAsync({ status: next });
      toast.success(next === "active" ? "User enabled" : "User disabled");
    } catch (err) {
      toastError(err, `Couldn't ${next === "active" ? "enable" : "disable"} the user`);
    }
  }

  async function handleReset() {
    const confirmed = await confirmDialog({
      title: "Reset this user's password?",
      description: `${user!.email} will be signed out everywhere. Their current password stops working; share the one-time password with them.`,
      confirmLabel: "Reset password",
    });
    if (!confirmed) return;
    try {
      const res = await resetPassword.mutateAsync(user!.id);
      const copied = await copyText(res.password);
      toast.success(copied ? "One-time password generated and copied" : "One-time password generated", {
        description: copied ? undefined : res.password,
        duration: 10000,
      });
    } catch (err) {
      toastError(err, "Couldn't reset the password");
    }
  }

  async function handleRevokeSessions() {
    try {
      await revokeSessions.mutateAsync(user!.id);
      toast.success("Signed out of all sessions");
    } catch (err) {
      toastError(err, "Couldn't revoke sessions");
    }
  }

  async function handleDelete() {
    const confirmed = await confirmDialog({
      title: "Delete this user?",
      description: `${user!.email} will lose access immediately. This cannot be undone.`,
      confirmLabel: "Delete user",
      variant: "destructive",
    });
    if (!confirmed) return;
    try {
      await del.mutateAsync(user!.id);
    } catch (err) {
      toastError(err, "Couldn't delete the user");
      return;
    }
    toast.success("User deleted");
    navigate("/app/admin/users");
  }

  return (
    <div className="flex flex-col gap-5">
      <Button variant="ghost" size="sm" className="w-fit gap-1.5 text-muted-foreground" onClick={() => navigate("/app/admin/users")}>
        <ArrowLeft className="size-4" /> Back to users
      </Button>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold">{user.name}</h1>
          <p className="text-sm text-muted-foreground">{user.email}</p>
          <div className="mt-2 flex gap-1.5">
            <Badge variant={user.role === "admin" ? "default" : "outline"}>{user.role}</Badge>
            <Badge variant={user.status === "active" ? "success" : "destructive"}>{user.status}</Badge>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" className="gap-1.5" onClick={handleReset} disabled={resetPassword.isPending}>
            <KeyRound className="size-3.5" /> Reset password
          </Button>
          <Button variant="outline" size="sm" className="gap-1.5" onClick={handleRevokeSessions} disabled={revokeSessions.isPending}>
            <LogOut className="size-3.5" /> Revoke sessions
          </Button>
          <Button variant="outline" size="sm" onClick={toggleStatus} disabled={update.isPending}>
            {user.status === "active" ? "Disable" : "Enable"}
          </Button>
          <Button variant="outline" size="sm" className="gap-1.5 text-destructive" onClick={handleDelete} disabled={del.isPending}>
            <Trash2 className="size-3.5" /> Delete
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Links ({links?.items.length ?? 0})</CardTitle>
        </CardHeader>
        <CardContent>
          {!links?.items.length ? (
            <p className="py-4 text-center text-sm text-muted-foreground">No links owned by this user.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Code</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Clicks</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {links.items.map((l) => (
                  <TableRow key={l.id} {...linkRowProps(() => navigate(`/app/links/${l.id}`))}>
                    <TableCell className="font-mono">{l.code}</TableCell>
                    <TableCell className="max-w-64 truncate">{l.targetUrl}</TableCell>
                    <TableCell>{l.clickCount.toLocaleString()}</TableCell>
                    <TableCell>
                      <LocalTime iso={l.createdAt} relative />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
