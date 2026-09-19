import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Link, useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { FieldError } from "@/components/field-error";
import { emailSchema, nameSchema, userPasswordSchema } from "@/lib/schemas";
import { applyServerErrors } from "@/lib/apply-server-errors";
import { useRegister } from "@/features/auth/api";
import { useAuthStatus } from "@/hooks/use-auth-status";

const registerSchema = z.object({
  name: nameSchema,
  email: emailSchema,
  password: userPasswordSchema,
});
type RegisterInput = z.infer<typeof registerSchema>;

export default function RegisterPage() {
  const register_ = useRegister();
  const authStatus = useAuthStatus();
  const registrationOpen = authStatus.data?.registration === "open";
  const navigate = useNavigate();

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<RegisterInput>({ resolver: zodResolver(registerSchema) });

  async function onSubmit(values: RegisterInput) {
    try {
      await register_.mutateAsync(values);
      navigate("/app");
    } catch (err) {
      applyServerErrors(err, setError);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Create an account</CardTitle>
          <CardDescription>
            {authStatus.isLoading
              ? "Checking registration…"
              : registrationOpen
                ? "Registration is open on this instance."
                : "Registration is not open on this instance. Ask an administrator for an account."}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {registrationOpen && (
          <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="name">Name</Label>
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
            </div>
            <Button type="submit" disabled={isSubmitting} className="w-full">
              {isSubmitting ? "Creating…" : "Create account"}
            </Button>
          </form>
          )}
          <p className="mt-4 text-center text-sm text-muted-foreground">
            Already have an account? <Link to="/app/login" className="text-primary hover:underline">Sign in</Link>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
