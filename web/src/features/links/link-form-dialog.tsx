import * as React from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Check, Eye, EyeOff, RefreshCw, X, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { TagInput } from "@/components/ui/tag-input";
import { FieldError } from "@/components/field-error";
import { copyText } from "@/lib/clipboard";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { createLinkSchema, type CreateLinkInput } from "@/lib/schemas";
import { applyServerErrors } from "@/lib/apply-server-errors";
import { useCreateLink, useUpdateLink, useCheckAlias, usePreviewTitle } from "@/features/links/api";
import type { Link } from "@/lib/types";
import { useDebouncedValue } from "@/hooks/use-debounced-value";

const EXPIRY_PRESETS = [
  { label: "1 hour", ms: 3600_000 },
  { label: "1 day", ms: 86_400_000 },
  { label: "7 days", ms: 7 * 86_400_000 },
  { label: "30 days", ms: 30 * 86_400_000 },
];

const REDIRECT_OPTIONS: { value: 301 | 302 | 307 | 308; label: string; hint: string }[] = [
  { value: 302, label: "302 Found", hint: "Temporary redirect (default, best for tracking)." },
  { value: 301, label: "301 Moved Permanently", hint: "Permanent; browsers may cache it long-term." },
  { value: 307, label: "307 Temporary Redirect", hint: "Like 302 but preserves the request method." },
  { value: 308, label: "308 Permanent Redirect", hint: "Like 301 but preserves the request method." },
];

/** 10 characters from an unambiguous alphabet, from the CSPRNG (not Math.random). */
function generatePassword() {
  const alphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789";
  const bytes = crypto.getRandomValues(new Uint8Array(10));
  return Array.from(bytes, (b) => alphabet[b % alphabet.length]).join("");
}

function toDatetimeLocal(iso: string | null | undefined) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function LinkFormDialog({
  open,
  onOpenChange,
  link,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  link?: Link;
  onCreated?: (link: Link) => void;
}) {
  const isEdit = !!link;
  const create = useCreateLink();
  const update = useUpdateLink(link?.id ?? "");
  const previewTitle = usePreviewTitle();
  const [showPassword, setShowPassword] = React.useState(false);

  const {
    register,
    control,
    handleSubmit,
    watch,
    getValues,
    setValue,
    setError,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<CreateLinkInput>({
    resolver: zodResolver(createLinkSchema),
    defaultValues: {
      targetUrl: "",
      code: "",
      title: "",
      description: "",
      redirectStatus: 302,
      password: "",
      expiresAt: null,
      maxClicks: null,
      tags: [],
      passQuery: true,
      utm: { source: "", medium: "", campaign: "", term: "", content: "" },
    },
  });

  React.useEffect(() => {
    if (open) {
      reset(
        link
          ? {
              targetUrl: link.targetUrl,
              code: link.code,
              title: link.title,
              description: link.description,
              redirectStatus: link.redirectStatus,
              password: "",
              expiresAt: link.expiresAt,
              maxClicks: link.maxClicks,
              tags: link.tags,
              passQuery: link.passQuery,
              utm: link.utm,
            }
          : {
              targetUrl: "",
              code: "",
              title: "",
              description: "",
              redirectStatus: 302,
              password: "",
              expiresAt: null,
              maxClicks: null,
              tags: [],
              passQuery: true,
              utm: { source: "", medium: "", campaign: "", term: "", content: "" },
            },
      );
      setShowPassword(false);
    }
  }, [open, link, reset]);

  const targetUrl = watch("targetUrl");
  const targetUrlField = register("targetUrl");
  const code = watch("code") || "";
  const debouncedCode = useDebouncedValue(code, 400);
  const aliasCheck = useCheckAlias(debouncedCode, debouncedCode.length > 0 && (!isEdit || debouncedCode !== link?.code));

  async function handleFetchTitle() {
    // Only fill an empty title, and only for something that looks like a URL:
    // never overwrite a title the user typed or that the link already has.
    if (!/^https?:\/\/\S+$/i.test(targetUrl?.trim() ?? "") || getValues("title")) return;
    try {
      const res = await previewTitle.mutateAsync(targetUrl);
      if (res.title && !getValues("title")) setValue("title", res.title);
    } catch {
      // Title fetch is best-effort; silently ignore.
    }
  }

  async function onSubmit(values: CreateLinkInput) {
    const payload = {
      ...values,
      code: values.code || undefined,
      password: values.password === "" ? (isEdit ? "" : undefined) : values.password,
      expiresAt: values.expiresAt || null,
      title: values.title || "",
      description: values.description || "",
    };
    try {
      if (isEdit && link) {
        const updated = await update.mutateAsync(payload);
        toast.success("Link updated");
        onOpenChange(false);
        onCreated?.(updated);
      } else {
        const created = await create.mutateAsync(payload);
        const copied = await copyText(created.shortUrl);
        toast.success(copied ? "Link created and copied to clipboard" : "Link created", {
          description: created.shortUrl,
        });
        onOpenChange(false);
        onCreated?.(created);
      }
    } catch (err) {
      applyServerErrors(err, setError);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit link" : "Create a short link"}</DialogTitle>
          <DialogDescription>
            {isEdit ? "Update the link and its options." : "Paste a URL, customize if you like, and shorten it."}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="targetUrl">Destination URL</Label>
            <div className="flex gap-2">
              <Input
                id="targetUrl"
                type="url"
                inputMode="url"
                autoFocus
                placeholder="https://example.com/very/long/path"
                aria-invalid={!!errors.targetUrl}
                {...targetUrlField}
                onBlur={(e) => {
                  targetUrlField.onBlur(e);
                  handleFetchTitle();
                }}
              />
              {previewTitle.isPending && (
                <Loader2 className="mt-3 size-4 shrink-0 animate-spin text-muted-foreground" aria-label="Fetching page title" />
              )}
            </div>
            <FieldError message={errors.targetUrl?.message} />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="code">Custom alias (optional)</Label>
            <div className="flex items-center gap-2 rounded-lg border border-input bg-background px-3 shadow-sm focus-within:ring-2 focus-within:ring-ring">
              <span className="max-w-[45%] shrink-0 truncate text-sm text-muted-foreground">{window.location.host}/</span>
              <input
                id="code"
                className="h-9 min-w-0 flex-1 bg-transparent text-sm outline-none"
                placeholder="my-link"
                autoComplete="off"
                spellCheck={false}
                aria-invalid={!!errors.code || aliasCheck.data?.available === false}
                {...register("code")}
              />
              {debouncedCode && debouncedCode !== link?.code && (
                <span className="shrink-0" aria-live="polite">
                  {aliasCheck.isFetching ? (
                    <Loader2 className="size-4 animate-spin text-muted-foreground" />
                  ) : aliasCheck.data?.available ? (
                    <Check className="size-4 text-success" aria-label="Available" />
                  ) : aliasCheck.data ? (
                    <X className="size-4 text-destructive" aria-label="Taken" />
                  ) : null}
                </span>
              )}
            </div>
            <FieldError message={errors.code?.message || (aliasCheck.data && !aliasCheck.data.available ? aliasCheck.data.reason || "Alias is taken" : undefined)} />
          </div>

          <Accordion type="single" collapsible>
            <AccordionItem value="more" className="border-none">
              <AccordionTrigger className="text-sm text-muted-foreground hover:no-underline">
                More options
              </AccordionTrigger>
              <AccordionContent className="flex flex-col gap-4 pt-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="title">Title</Label>
                  <Input id="title" {...register("title")} />
                  <FieldError message={errors.title?.message} />
                </div>

                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="description">Description</Label>
                  <Textarea id="description" rows={2} {...register("description")} />
                  <FieldError message={errors.description?.message} />
                </div>

                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="tags">Tags</Label>
                  <Controller
                    control={control}
                    name="tags"
                    render={({ field }) => (
                      <TagInput id="tags" value={field.value ?? []} onChange={field.onChange} aria-label="Tags" />
                    )}
                  />
                  <FieldError message={errors.tags?.message as string | undefined} />
                </div>

                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="expiresAt">Expires</Label>
                    <Input
                      id="expiresAt"
                      type="datetime-local"
                      value={toDatetimeLocal(watch("expiresAt"))}
                      onChange={(e) =>
                        setValue("expiresAt", e.target.value ? new Date(e.target.value).toISOString() : null)
                      }
                    />
                    <div className="flex flex-wrap gap-1.5">
                      {EXPIRY_PRESETS.map((p) => (
                        <button
                          key={p.label}
                          type="button"
                          onClick={() => setValue("expiresAt", new Date(Date.now() + p.ms).toISOString())}
                          className="rounded-full border border-border px-2.5 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        >
                          {p.label}
                        </button>
                      ))}
                      <button
                        type="button"
                        onClick={() => setValue("expiresAt", null)}
                        className="rounded-full border border-border px-2.5 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        Clear
                      </button>
                    </div>
                    <FieldError message={errors.expiresAt?.message as string | undefined} />
                  </div>

                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="maxClicks">Click limit</Label>
                    <Input
                      id="maxClicks"
                      type="number"
                      min={1}
                      placeholder="Unlimited"
                      value={watch("maxClicks") ?? ""}
                      onChange={(e) => setValue("maxClicks", e.target.value ? Number(e.target.value) : null)}
                    />
                    <FieldError message={errors.maxClicks?.message as string | undefined} />
                  </div>
                </div>

                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="password">Password protect</Label>
                  <div className="flex gap-2">
                    <div className="relative flex-1">
                      <Input
                        id="password"
                        type={showPassword ? "text" : "password"}
                        placeholder={isEdit && link?.hasPassword ? "•••••••• (unchanged)" : "No password"}
                        autoComplete="new-password"
                        {...register("password")}
                        className="pr-10"
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword((s) => !s)}
                        className="absolute right-1 top-1/2 flex size-8 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        aria-label={showPassword ? "Hide password" : "Show password"}
                        aria-pressed={showPassword}
                      >
                        {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                      </button>
                    </div>
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      aria-label="Generate password"
                      title="Generate password"
                      onClick={() => {
                        setValue("password", generatePassword(), { shouldDirty: true });
                        setShowPassword(true);
                      }}
                    >
                      <RefreshCw className="size-4" />
                    </Button>
                  </div>
                  <FieldError message={errors.password?.message} />
                  {isEdit && <p className="text-xs text-muted-foreground">Leave blank to keep the current password. Save an empty value to remove it.</p>}
                </div>

                <div className="flex flex-col gap-2">
                  <Label>Redirect type</Label>
                  <Controller
                    control={control}
                    name="redirectStatus"
                    render={({ field }) => (
                      <RadioGroup
                        value={String(field.value)}
                        onValueChange={(v) => field.onChange(Number(v))}
                        className="gap-2"
                      >
                        {REDIRECT_OPTIONS.map((opt) => (
                          <label key={opt.value} className="flex cursor-pointer items-start gap-2 rounded-lg border border-border p-2 text-sm hover:bg-accent/50">
                            <RadioGroupItem value={String(opt.value)} className="mt-0.5" />
                            <span>
                              <span className="font-medium">{opt.label}</span>
                              <span className="block text-xs text-muted-foreground">{opt.hint}</span>
                            </span>
                          </label>
                        ))}
                      </RadioGroup>
                    )}
                  />
                </div>

                <div className="rounded-lg border border-border p-3">
                  <p className="mb-2 text-sm font-medium">UTM parameters</p>
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                    {(["source", "medium", "campaign", "term", "content"] as const).map((k) => (
                      <div key={k} className="flex flex-col gap-1">
                        <Label htmlFor={`utm-${k}`} className="text-xs capitalize text-muted-foreground">
                          {k}
                        </Label>
                        <Input id={`utm-${k}`} {...register(`utm.${k}` as const)} />
                      </div>
                    ))}
                  </div>
                </div>

                <div className="flex items-center justify-between rounded-lg border border-border p-3">
                  <div>
                    <p className="text-sm font-medium">Pass through query params</p>
                    <p className="text-xs text-muted-foreground">Forward ?params from the short URL to the destination.</p>
                  </div>
                  <Controller
                    control={control}
                    name="passQuery"
                    render={({ field }) => (
                      <Switch checked={field.value} onCheckedChange={field.onChange} aria-label="Pass through query params" />
                    )}
                  />
                </div>
              </AccordionContent>
            </AccordionItem>
          </Accordion>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Saving…" : isEdit ? "Save changes" : "Shorten"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
