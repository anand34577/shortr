import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link, useSearchParams } from "react-router-dom";
import { Link2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { FieldError } from "@/components/field-error";
import { Separator } from "@/components/ui/separator";
import { loginSchema, type LoginInput } from "@/lib/schemas";
import { applyServerErrors } from "@/lib/apply-server-errors";
import { useLogin, oidcStartUrl } from "@/features/auth/api";
import { useAuthStatus } from "@/hooks/use-auth-status";
import { safeNextPath } from "@/lib/api";

export default function LoginPage() {
  const [params] = useSearchParams();
  const next = safeNextPath(params.get("next"));
  const status = useAuthStatus();
  const login = useLogin();

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<LoginInput>({ resolver: zodResolver(loginSchema) });

  async function onSubmit(values: LoginInput) {
    try {
      await login.mutateAsync(values);
      window.location.href = next;
    } catch (err) {
      applyServerErrors(err, setError);
    }
  }

  const showLocal = status.data?.localLogin ?? true;
  const showSso = status.data?.oidcEnabled;

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="items-center text-center">
          <div className="mb-2 flex size-10 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Link2 className="size-5" />
          </div>
          <CardTitle>{status.data?.siteName || "Shortr"}</CardTitle>
          <CardDescription>Sign in to manage your short links</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {showSso && (
            <>
              <Button asChild variant="default" size="lg" className="w-full">
                <a href={oidcStartUrl(next)}>Sign in with {status.data?.oidcDisplayName || "SSO"}</a>
              </Button>
              {showLocal && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Separator className="flex-1" />
                  or
                  <Separator className="flex-1" />
                </div>
              )}
            </>
          )}

          {showLocal && (
            <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="email">Email</Label>
                <Input id="email" type="email" autoComplete="email" autoFocus {...register("email")} />
                <FieldError message={errors.email?.message} />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="password">Password</Label>
                <Input id="password" type="password" autoComplete="current-password" {...register("password")} />
                <FieldError message={errors.password?.message} />
              </div>
              <Button type="submit" disabled={isSubmitting} className="w-full">
                {isSubmitting ? "Signing in…" : "Sign in"}
              </Button>
            </form>
          )}

          {status.data?.registration === "open" && (
            <p className="text-center text-sm text-muted-foreground">
              No account? <Link to="/app/register" className="text-primary underline-offset-4 hover:underline">Register</Link>
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
