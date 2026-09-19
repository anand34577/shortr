import * as React from "react";
import { useParams, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { ArrowLeft, Copy, KeyRound, LogOut, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { LocalTime } from "@/components/local-time";
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
  const { data: user, isLoading } = useAdminUser(id);
  const { data: links } = useAdminUserLinks(id);
  const update = useUpdateUser(id ?? "");
  const del = useDeleteUser();
  const resetPassword = useResetUserPassword();
  const revokeSessions = useRevokeUserSessions();

  if (isLoading || !user) {
    return <Skeleton className="h-64 w-full" />;
  }

  async function toggleStatus() {
    const next = user!.status === "active" ? "disabled" : "active";
    await update.mutateAsync({ status: next });
    toast.success(next === "active" ? "User enabled" : "User disabled");
  }

  async function handleReset() {
    const res = await resetPassword.mutateAsync(user!.id);
    await navigator.clipboard.writeText(res.password).catch(() => {});
    toast.success("One-time password generated & copied to clipboard", { duration: 10000 });
  }

  async function handleDelete() {
    if (!confirm(`Delete ${user!.email}? This cannot be undone.`)) return;
    await del.mutateAsync(user!.id);
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
          <Button variant="outline" size="sm" className="gap-1.5" onClick={handleReset}>
            <KeyRound className="size-3.5" /> Reset password
          </Button>
          <Button variant="outline" size="sm" className="gap-1.5" onClick={() => revokeSessions.mutate(user.id)}>
            <LogOut className="size-3.5" /> Revoke sessions
          </Button>
          <Button variant="outline" size="sm" onClick={toggleStatus}>
            {user.status === "active" ? "Disable" : "Enable"}
          </Button>
          <Button variant="outline" size="sm" className="gap-1.5 text-destructive" onClick={handleDelete}>
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
                  <TableRow key={l.id} className="cursor-pointer" onClick={() => navigate(`/app/links/${l.id}`)}>
                    <TableCell className="flex items-center gap-1.5 font-mono">
                      {l.code} <Copy className="size-3 text-muted-foreground" />
                    </TableCell>
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
