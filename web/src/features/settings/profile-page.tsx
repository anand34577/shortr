import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { FieldError } from "@/components/field-error";
import { SettingsNav } from "@/features/settings/settings-nav";
import { profileSchema, type ProfileInput } from "@/lib/schemas";
import { applyServerErrors } from "@/lib/apply-server-errors";
import { useMe } from "@/hooks/use-me";
import { useUpdateProfile, useDeleteAccount } from "@/features/settings/api";
import { queryClient } from "@/app/providers";

export default function ProfilePage() {
  const me = useMe();
  const update = useUpdateProfile();
  const deleteAccount = useDeleteAccount();
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleteLinks, setDeleteLinks] = React.useState(false);

  async function onDeleteAccount() {
    try {
      await deleteAccount.mutateAsync(deleteLinks ? "delete" : "keep");
      queryClient.clear();
      window.location.href = "/app/login";
    } catch (err) {
      applyServerErrors(err, () => {});
    }
  }

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting, isDirty },
  } = useForm<ProfileInput>({
    resolver: zodResolver(profileSchema),
    values: me.data ? { name: me.data.name, email: me.data.email, password: "" } : undefined,
  });

  async function onSubmit(values: ProfileInput) {
    try {
      await update.mutateAsync({
        name: values.name,
        email: values.email,
        password: values.password || undefined,
      });
      toast.success("Profile updated");
    } catch (err) {
      applyServerErrors(err, setError);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
        <SettingsNav />
      </div>

      <Card className="max-w-lg">
        <CardHeader>
          <CardTitle className="text-base">Profile</CardTitle>
          <CardDescription>Update your name and email address.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="name">Name</Label>
              <Input id="name" {...register("name")} />
              <FieldError message={errors.name?.message} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="email">Email</Label>
              <Input id="email" type="email" {...register("email")} />
              <FieldError message={errors.email?.message} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="password">Current password</Label>
              <Input id="password" type="password" placeholder="Required to change email" {...register("password")} />
              <FieldError message={errors.password?.message} />
            </div>
            <Button type="submit" disabled={isSubmitting || !isDirty} className="w-fit">
              {isSubmitting ? "Saving…" : "Save changes"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card className="max-w-lg border-destructive/40">
        <CardHeader>
          <CardTitle className="text-base text-destructive">Danger zone</CardTitle>
          <CardDescription>Permanently delete your account and sign out everywhere.</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="destructive" onClick={() => setDeleteOpen(true)} disabled={deleteAccount.isPending}>
            Delete account
          </Button>
        </CardContent>
      </Card>

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete your account?</DialogTitle>
            <DialogDescription>This permanently signs you out everywhere and cannot be undone.</DialogDescription>
          </DialogHeader>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={deleteLinks} onCheckedChange={(c) => setDeleteLinks(!!c)} />
            Also delete all of my links
          </label>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDeleteOpen(false)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={onDeleteAccount} disabled={deleteAccount.isPending}>
              Delete account
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
