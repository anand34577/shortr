import { z } from "zod";

// The server is authoritative; this is UX-only validation.

const PRIVATE_HOST_RE =
  /^(localhost|.*\.local|.*\.internal|127(?:\.\d{1,3}){3}|10(?:\.\d{1,3}){3}|192\.168(?:\.\d{1,3}){2}|172\.(1[6-9]|2\d|3[0-1])(?:\.\d{1,3}){2}|0\.0\.0\.0|::1)$/i;

export const targetUrlSchema = z
  .string()
  .trim()
  .min(1, "URL is required")
  .max(2048, "URL is too long")
  .refine((v) => !/\s/.test(v), "URL cannot contain spaces")
  .superRefine((v, ctx) => {
    let url: URL;
    try {
      url = new URL(v);
    } catch {
      ctx.addIssue({ code: "custom", message: "Enter a valid URL" });
      return;
    }
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      ctx.addIssue({ code: "custom", message: "Only http(s) URLs are allowed" });
    }
    if (url.username || url.password) {
      ctx.addIssue({ code: "custom", message: "URLs with credentials are not allowed" });
    }
    if (!url.hostname) {
      ctx.addIssue({ code: "custom", message: "URL must have a host" });
    } else if (PRIVATE_HOST_RE.test(url.hostname)) {
      ctx.addIssue({ code: "custom", message: "Private/local targets are not allowed" });
    }
  });

export const aliasSchema = z
  .string()
  .trim()
  .regex(/^[A-Za-z0-9_-]{1,64}$/, "1-64 letters, numbers, - or _")
  .refine((v) => !v.startsWith("-") && !v.startsWith("_"), "Cannot start with - or _")
  .refine((v) => !v.endsWith("-") && !v.endsWith("_"), "Cannot end with - or _")
  .optional()
  .or(z.literal(""));

export const codeLengthSchema = z.number().int().min(4).max(16);

export const titleSchema = z.string().max(200, "Max 200 characters").optional().or(z.literal(""));
export const descriptionSchema = z.string().max(1000, "Max 1000 characters").optional().or(z.literal(""));

export const redirectStatusSchema = z.union([
  z.literal(301),
  z.literal(302),
  z.literal(307),
  z.literal(308),
]);

export const linkPasswordSchema = z
  .string()
  .max(128, "Max 128 characters")
  .optional()
  .or(z.literal(""));

export const maxClicksSchema = z
  .number()
  .int()
  .min(1)
  .max(2147483647)
  .nullable()
  .optional();

export const tagSchema = z
  .string()
  .trim()
  .min(1)
  .max(32, "Max 32 characters")
  .regex(/^[\p{L}\p{N} _-]+$/u, "Letters, numbers, spaces, - or _ only");

export const tagsSchema = z.array(tagSchema).max(10, "Max 10 tags").optional();

export const utmFieldSchema = z
  .string()
  .max(255, "Max 255 characters")
  .refine((v) => !/[\r\n]/.test(v), "No line breaks allowed")
  .optional()
  .or(z.literal(""));

export const utmSchema = z.object({
  source: utmFieldSchema,
  medium: utmFieldSchema,
  campaign: utmFieldSchema,
  term: utmFieldSchema,
  content: utmFieldSchema,
});

export const expiresAtSchema = z
  .string()
  .optional()
  .nullable()
  .refine((v) => {
    if (!v) return true;
    const d = new Date(v);
    if (Number.isNaN(d.getTime())) return false;
    return d.getTime() > Date.now() + 60_000;
  }, "Expiry must be at least 1 minute in the future");

export const createLinkSchema = z.object({
  targetUrl: targetUrlSchema,
  code: aliasSchema,
  length: codeLengthSchema.optional(),
  title: titleSchema,
  description: descriptionSchema,
  redirectStatus: redirectStatusSchema,
  password: linkPasswordSchema,
  expiresAt: expiresAtSchema,
  maxClicks: maxClicksSchema,
  tags: tagsSchema,
  passQuery: z.boolean(),
  utm: utmSchema.optional(),
});
export type CreateLinkInput = z.infer<typeof createLinkSchema>;

export const editLinkSchema = createLinkSchema.partial({ targetUrl: true }).extend({
  targetUrl: targetUrlSchema,
});
export type EditLinkInput = z.infer<typeof editLinkSchema>;

// --- Auth / users ---

export const emailSchema = z
  .string()
  .trim()
  .toLowerCase()
  .max(254)
  .refine((v) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v), "Enter a valid email address");

export const userPasswordSchema = z
  .string()
  .min(10, "At least 10 characters")
  .max(128, "Max 128 characters");

export const nameSchema = z.string().trim().min(1, "Required").max(100, "Max 100 characters");

export const loginSchema = z.object({
  email: emailSchema,
  password: z.string().min(1, "Password is required"),
});
export type LoginInput = z.infer<typeof loginSchema>;

export const setupSchema = z.object({
  email: emailSchema,
  name: nameSchema,
  password: userPasswordSchema,
  siteName: z.string().trim().min(1).max(100).optional(),
  baseUrl: z.string().trim().url().optional(),
});
export type SetupInput = z.infer<typeof setupSchema>;

export const changePasswordSchema = z
  .object({
    current: z.string().optional().or(z.literal("")),
    next: userPasswordSchema,
    confirm: z.string(),
  })
  .refine((v) => v.next === v.confirm, {
    message: "Passwords do not match",
    path: ["confirm"],
  });
export type ChangePasswordInput = z.infer<typeof changePasswordSchema>;

export const profileSchema = z.object({
  name: nameSchema,
  email: emailSchema,
  password: z.string().optional().or(z.literal("")),
});
export type ProfileInput = z.infer<typeof profileSchema>;

// --- API keys ---

export const apiKeyNameSchema = z.string().trim().min(1).max(64, "Max 64 characters");
export const apiKeyScopes = ["links:read", "links:write", "stats:read"] as const;
export const createApiKeySchema = z.object({
  name: apiKeyNameSchema,
  scopes: z.array(z.enum(apiKeyScopes)).min(1, "Select at least one scope"),
  expiresAt: z.string().optional().or(z.literal("")),
});
export type CreateApiKeyInput = z.infer<typeof createApiKeySchema>;

// --- Admin ---

export const inviteUserSchema = z.object({
  email: emailSchema,
  name: nameSchema,
  role: z.enum(["admin", "user"]),
  password: userPasswordSchema.optional().or(z.literal("")),
});
export type InviteUserInput = z.infer<typeof inviteUserSchema>;

export const adminSettingsSchema = z.object({
  siteName: z.string().trim().min(1).max(100),
  registration: z.enum(["closed", "open", "invite"]),
  defaultRedirectStatus: redirectStatusSchema,
  countBots: z.boolean(),
  ipMode: z.enum(["full", "anonymize", "hash", "none"]),
  blockedDomains: z.array(z.string()),
  maxLinksPerUser: z.number().int().min(0),
  fetchTitles: z.boolean(),
  clickRetentionDays: z.number().int().min(0),
  ipLocationEnabled: z.boolean(),
  ipLocationBaseUrl: z
    .string()
    .trim()
    .max(2048, "Max 2048 characters")
    .refine((v) => v === "" || /^https?:\/\/.+/i.test(v), "Must be a valid http(s) URL"),
  mcpEnabled: z.boolean(),
}).refine((v) => !v.ipLocationEnabled || v.ipLocationBaseUrl !== "", {
  message: "Base URL is required when enabled",
  path: ["ipLocationBaseUrl"],
});
export type AdminSettingsInput = z.infer<typeof adminSettingsSchema>;

// --- misc ---

export const dateRangeSchema = z
  .object({ from: z.string(), to: z.string() })
  .refine((v) => new Date(v.from).getTime() <= new Date(v.to).getTime(), {
    message: "'From' must be before 'to'",
    path: ["to"],
  });

export const nextPathSchema = z
  .string()
  .max(512)
  .refine((v) => v.startsWith("/app") && !v.startsWith("//") && !v.includes("\\"), {
    message: "Invalid redirect path",
  });
