import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Link, useSearchParams } from "react-router-dom";
import { ArrowLeft, Link2, ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { FieldError } from "@/components/field-error";
import { Separator } from "@/components/ui/separator";
import { loginSchema, type LoginInput } from "@/lib/schemas";
import { applyServerErrors } from "@/lib/apply-server-errors";
import { useLogin, useLoginMfa, oidcStartUrl } from "@/features/auth/api";
import { useAuthStatus } from "@/hooks/use-auth-status";
import { ApiError, safeNextPath } from "@/lib/api";

export default function LoginPage() {
  const [params] = useSearchParams();
  const next = safeNextPath(params.get("next"));
  const status = useAuthStatus();
  const login = useLogin();
  const [mfaToken, setMfaToken] = React.useState<string | null>(null);
  const ssoError = params.get("message");

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<LoginInput>({ resolver: zodResolver(loginSchema) });

  async function onSubmit(values: LoginInput) {
    try {
      const res = await login.mutateAsync(values);
      if ("mfaRequired" in res) {
        setMfaToken(res.mfaToken);
        return;
      }
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
            {mfaToken ? <ShieldCheck className="size-5" /> : <Link2 className="size-5" />}
          </div>
          <CardTitle>{mfaToken ? "Two-factor check" : status.data?.siteName || "Shortr"}</CardTitle>
          <CardDescription>
            {mfaToken ? "Enter the 6-digit code from your authenticator app." : "Sign in to manage your short links"}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {mfaToken ? (
            <MfaStep token={mfaToken} next={next} onRestart={() => setMfaToken(null)} />
          ) : (
            <>
              {ssoError && (
                <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {ssoError}
                </p>
              )}
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
                    <Input id="email" type="email" autoComplete="username" autoFocus {...register("email")} />
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
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function MfaStep({ token, next, onRestart }: { token: string; next: string; onRestart: () => void }) {
  const verify = useLoginMfa();
  const [useRecovery, setUseRecovery] = React.useState(false);
  const [value, setValue] = React.useState("");
  const [error, setError] = React.useState<string | null>(null);
  const inputRef = React.useRef<HTMLInputElement>(null);

  async function submit(v = value) {
    if (verify.isPending) return;
    setError(null);
    try {
      await verify.mutateAsync(useRecovery ? { mfaToken: token, recoveryCode: v } : { mfaToken: token, code: v });
      window.location.href = next;
    } catch (err) {
      if (err instanceof ApiError && err.code === "MFA_EXPIRED") {
        onRestart();
        return;
      }
      setError(err instanceof ApiError ? err.message : "Something went wrong. Try again.");
      setValue("");
      inputRef.current?.focus();
    }
  }

  return (
    <form
      className="flex flex-col gap-4"
      noValidate
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="mfa-code">{useRecovery ? "Recovery code" : "Authentication code"}</Label>
        {useRecovery ? (
          <Input
            ref={inputRef}
            id="mfa-code"
            autoFocus
            autoComplete="off"
            spellCheck={false}
            placeholder="xxxx-xxxx-xxxx"
            className="font-mono"
            value={value}
            onChange={(e) => setValue(e.target.value)}
          />
        ) : (
          <Input
            ref={inputRef}
            id="mfa-code"
            autoFocus
            inputMode="numeric"
            autoComplete="one-time-code"
            pattern="[0-9]*"
            maxLength={6}
            placeholder="123456"
            className="text-center font-mono text-2xl tracking-[0.5em]"
            value={value}
            onChange={(e) => {
              const digits = e.target.value.replace(/\D/g, "").slice(0, 6);
              setValue(digits);
              if (digits.length === 6) void submit(digits);
            }}
          />
        )}
        <FieldError message={error ?? undefined} />
      </div>
      <Button type="submit" className="w-full" disabled={verify.isPending || (!useRecovery && value.length !== 6) || !value}>
        {verify.isPending ? "Checking…" : "Verify"}
      </Button>
      <div className="flex items-center justify-between text-sm">
        <button type="button" className="flex items-center gap-1 text-muted-foreground hover:text-foreground" onClick={onRestart}>
          <ArrowLeft className="size-3.5" /> Back
        </button>
        <button
          type="button"
          className="text-primary underline-offset-4 hover:underline"
          onClick={() => {
            setUseRecovery((v) => !v);
            setValue("");
            setError(null);
          }}
        >
          {useRecovery ? "Use authenticator app" : "Use a recovery code"}
        </button>
      </div>
    </form>
  );
}
