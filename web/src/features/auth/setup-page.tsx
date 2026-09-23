import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useNavigate } from "react-router-dom";
import { CheckCircle2, Link2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { FieldError } from "@/components/field-error";
import { setupSchema, type SetupInput } from "@/lib/schemas";
import { applyServerErrors } from "@/lib/apply-server-errors";
import { useSetup } from "@/features/auth/api";
import { cn } from "@/lib/utils";

const steps = ["Admin account", "Site basics", "Done"];

export default function SetupPage() {
  const [step, setStep] = React.useState(0);
  const navigate = useNavigate();
  const setup = useSetup();

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<SetupInput>({
    resolver: zodResolver(setupSchema),
    defaultValues: {
      siteName: "Shortr",
      baseUrl: typeof window !== "undefined" ? window.location.origin : "",
    },
  });

  async function onSubmit(values: SetupInput) {
    try {
      await setup.mutateAsync(values);
      setStep(2);
    } catch (err) {
      applyServerErrors(err, setError);
      setStep(0);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <Card className="w-full max-w-md">
        <CardHeader className="items-center text-center">
          <div className="mb-2 flex size-10 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Link2 className="size-5" />
          </div>
          <CardTitle>Welcome to Shortr</CardTitle>
          <CardDescription>Let's set up your instance</CardDescription>

          <ol className="mt-4 flex items-center gap-2" aria-label="Setup progress">
            {steps.map((label, i) => (
              <li key={label} className="flex items-center gap-2">
                <span
                  className={cn(
                    "flex size-6 items-center justify-center rounded-full text-xs font-semibold",
                    i < step ? "bg-success/10 text-success" : i === step ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground",
                  )}
                >
                  {i < step ? <CheckCircle2 className="size-4" /> : i + 1}
                </span>
                {i < steps.length - 1 && <span className="h-px w-6 bg-border" />}
              </li>
            ))}
          </ol>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit(onSubmit)} noValidate>
            {step === 0 && (
              <div className="flex flex-col gap-4">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="name">Your name</Label>
                  <Input id="name" autoFocus {...register("name")} />
                  <FieldError message={errors.name?.message} />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="email">Email</Label>
                  <Input id="email" type="email" {...register("email")} />
                  <FieldError message={errors.email?.message} />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="password">Password</Label>
                  <Input id="password" type="password" {...register("password")} />
                  <FieldError message={errors.password?.message} />
                  <p className="text-xs text-muted-foreground">At least 10 characters.</p>
                </div>
                <Button type="button" className="w-full" onClick={() => setStep(1)}>
                  Continue
                </Button>
              </div>
            )}

            {step === 1 && (
              <div className="flex flex-col gap-4">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="siteName">Site name</Label>
                  <Input id="siteName" {...register("siteName")} />
                  <FieldError message={errors.siteName?.message} />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="baseUrl">Base URL</Label>
                  <Input id="baseUrl" {...register("baseUrl")} />
                  <FieldError message={errors.baseUrl?.message} />
                  <p className="text-xs text-muted-foreground">
                    The public URL used to build short links. Confirm this matches your reverse proxy configuration.
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button type="button" variant="outline" className="flex-1" onClick={() => setStep(0)}>
                    Back
                  </Button>
                  <Button type="submit" className="flex-1" disabled={isSubmitting}>
                    {isSubmitting ? "Creating…" : "Create admin account"}
                  </Button>
                </div>
              </div>
            )}

            {step === 2 && (
              <div className="flex flex-col items-center gap-4 py-4 text-center">
                <CheckCircle2 className="size-10 text-success" />
                <p className="text-sm text-muted-foreground">
                  Your admin account is ready. Let's shorten your first link.
                </p>
                <Button className="w-full" onClick={() => navigate("/app")}>
                  Go to dashboard
                </Button>
              </div>
            )}
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
