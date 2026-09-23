import * as React from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Plus, Search, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, linkRowProps } from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { FieldError } from "@/components/field-error";
import { EmptyState } from "@/components/empty-state";
import { LocalTime } from "@/components/local-time";
import { AdminNav } from "@/features/admin/admin-nav";
import { useAdminUsers, useCreateUser } from "@/features/admin/api";
import { inviteUserSchema, type InviteUserInput } from "@/lib/schemas";
import { applyServerErrors } from "@/lib/apply-server-errors";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useCursorPager } from "@/hooks/use-cursor-pager";
import { copyText } from "@/lib/clipboard";

export default function AdminUsersPage() {
  const navigate = useNavigate();
  const [q, setQ] = React.useState("");
  const debouncedQ = useDebouncedValue(q, 300);
  const pager = useCursorPager();
  const { data, isLoading } = useAdminUsers({ q: debouncedQ || undefined, cursor: pager.cursor, limit: 50 });
  const create = useCreateUser();
  const [dialogOpen, setDialogOpen] = React.useState(false);

  const {
    register,
    control,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<InviteUserInput>({ resolver: zodResolver(inviteUserSchema), defaultValues: { role: "user" } });

  async function onSubmit(values: InviteUserInput) {
    try {
      const created = await create.mutateAsync(values);
      if (created.generatedPassword) {
        const copied = await copyText(created.generatedPassword);
        toast.success(copied ? "User created — temporary password copied" : "User created", {
          description: created.generatedPassword,
          duration: 30000,
        });
      } else {
        toast.success("User created");
      }
      setDialogOpen(false);
      reset();
    } catch (err) {
      applyServerErrors(err, setError);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Admin</h1>
        <AdminNav />
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative max-w-sm flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              pager.reset();
            }}
            placeholder="Search users…"
            className="pl-9"
            aria-label="Search users"
          />
        </div>
        <Button className="gap-1.5" onClick={() => setDialogOpen(true)}>
          <Plus className="size-4" /> Add user
        </Button>
      </div>

      <div className="rounded-xl border border-border">
        {isLoading ? (
          <div className="flex flex-col gap-2 p-4">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        ) : !data?.items.length ? (
          <EmptyState icon={Users} title="No users found" />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>User</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Links</TableHead>
                <TableHead>Last login</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.items.map((u) => (
                <TableRow key={u.id} {...linkRowProps(() => navigate(`/app/admin/users/${u.id}`))}>
                  <TableCell>
                    <div className="font-medium">{u.name}</div>
                    <div className="text-xs text-muted-foreground">{u.email}</div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={u.role === "admin" ? "default" : "outline"}>{u.role}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={u.status === "active" ? "success" : "destructive"}>{u.status}</Badge>
                  </TableCell>
                  <TableCell>{u.linksCount ?? "—"}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    <LocalTime iso={u.lastLoginAt} relative />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      {(pager.hasPrevious || data?.nextCursor) && (
        <nav className="flex justify-between" aria-label="Pagination">
          <Button variant="outline" size="sm" disabled={!pager.hasPrevious} onClick={pager.previous}>
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!data?.nextCursor}
            onClick={() => data?.nextCursor && pager.next(data.nextCursor)}
          >
            Next
          </Button>
        </nav>
      )}

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add user</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="u-name">Name</Label>
              <Input id="u-name" autoFocus {...register("name")} />
              <FieldError message={errors.name?.message} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="u-email">Email</Label>
              <Input id="u-email" type="email" {...register("email")} />
              <FieldError message={errors.email?.message} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="u-role">Role</Label>
              <Controller
                control={control}
                name="role"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={(v) => v && field.onChange(v)}>
                    <SelectTrigger id="u-role">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="user">User</SelectItem>
                      <SelectItem value="admin">Admin</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="u-password">Temporary password (optional)</Label>
              <Input id="u-password" type="password" placeholder="Auto-generated if left blank" {...register("password")} />
              <FieldError message={errors.password?.message} />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? "Creating…" : "Create user"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
