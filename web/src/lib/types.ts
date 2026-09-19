// Typed shapes for the Shortr HTTP API.
//
// NOTE: the Go backend is being implemented in parallel by another engineer.
// Field names here assume standard `encoding/json` camelCase struct tags
// derived from internal/store/types.go. Small mismatches are possible and
// are called out in the frontend completion report — this file is the
// single place to reconcile them once the real API is live.

export type Role = "admin" | "user";
export type UserStatus = "active" | "disabled";
export type LinkStatus = "active" | "disabled";
export type RedirectStatus = 301 | 302 | 307 | 308;

export interface User {
  id: string;
  email: string;
  emailVerified: boolean;
  name: string;
  role: Role;
  status: UserStatus;
  maxLinks: number | null;
  roleLocked: boolean;
  mustChangePassword: boolean;
  hasPassword: boolean;
  lastLoginAt: string | null;
  createdAt: string;
  updatedAt: string;
  linksCount?: number;
}

export interface Me extends User {
  csrfToken: string;
  capabilities: string[];
}

export interface OIDCIdentity {
  id: string;
  userId: string;
  issuer: string;
  subject: string;
  email: string;
  name: string;
  createdAt: string;
  lastLoginAt: string | null;
}

export interface SessionInfo {
  id: string;
  userId: string;
  ip: string;
  userAgent: string;
  createdAt: string;
  expiresAt: string;
  lastSeenAt: string;
  current?: boolean;
}

export interface ApiKey {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  lastUsedAt: string | null;
  expiresAt: string | null;
  revokedAt: string | null;
  createdAt: string;
}

export interface UserCreated extends User {
  generatedPassword?: string;
}

export interface ApiKeyCreated extends ApiKey {
  key: string; // one-time full value
}

export interface UTM {
  source: string;
  medium: string;
  campaign: string;
  term: string;
  content: string;
}

export interface Link {
  id: string;
  code: string;
  shortUrl: string;
  targetUrl: string;
  title: string;
  description: string;
  userId: string | null;
  redirectStatus: RedirectStatus;
  hasPassword: boolean;
  expiresAt: string | null;
  maxClicks: number | null;
  clickCount: number;
  lastClickAt: string | null;
  status: LinkStatus;
  deletedAt: string | null;
  utm: UTM;
  passQuery: boolean;
  tags: string[];
  createdAt: string;
  updatedAt: string;
  sparkline?: number[];
}

export interface Click {
  id: number;
  linkId: string;
  ts: string;
  ip: string;
  country: string;
  region: string;
  city: string;
  referrer: string;
  referrerHost: string;
  device: string;
  os: string;
  browser: string;
  isBot: boolean;
  lang: string;
  utm: Partial<UTM>;
}

export interface StatsSeriesPoint {
  bucket: string;
  clicks: number;
  uniques: number;
  bots: number;
}

export interface StatsBreakdownItem {
  key: string;
  clicks: number;
  pct: number;
}

export interface LinkStats {
  series: StatsSeriesPoint[];
  totals: { clicks: number; uniques: number; bots: number };
  byCountry: StatsBreakdownItem[];
  byDevice: StatsBreakdownItem[];
  byOS: StatsBreakdownItem[];
  byBrowser: StatsBreakdownItem[];
  byReferrer: StatsBreakdownItem[];
}

export interface StatsOverview {
  totals: { clicks: number; uniques: number; activeLinks: number };
  series: StatsSeriesPoint[];
  topLinks: Link[];
  topReferrer: string | null;
  deltaPct: number | null;
}

export interface RecentActivityItem {
  id: string;
  linkId: string;
  code: string;
  shortUrl: string;
  country: string;
  device: string;
  ts: string;
  referrerHost: string;
}

export interface AuditEntry {
  id: string;
  ts: string;
  actorUserId: string;
  actorEmail?: string;
  actorIp: string;
  action: string;
  targetType: string;
  targetId: string;
  meta: Record<string, unknown> | null;
}

export interface AuthStatus {
  setupRequired: boolean;
  oidcEnabled: boolean;
  oidcDisplayName: string;
  localLogin: boolean;
  registration: "closed" | "open" | "invite";
  siteName: string;
}

export interface AdminSettings {
  siteName: string;
  baseUrl: string;
  registration: "closed" | "open" | "invite";
  defaultRedirectStatus: RedirectStatus;
  countBots: boolean;
  ipMode: "full" | "anonymize" | "hash" | "none";
  blockedDomains: string[];
  maxLinksPerUser: number;
  fetchTitles: boolean;
  oidcEnabled: boolean;
  oidcAutoCreate: boolean;
  oidcAutoLinkByEmail: boolean;
  clickRetentionDays: number;
  ipLocationEnabled: boolean;
  ipLocationBaseUrl: string;
  mcpEnabled: boolean;
}

export interface AdminSystemInfo {
  version: string;
  commit: string;
  dbDriver: string;
  dbSizeBytes: number;
  cacheHitRatio: number;
  cacheEntries: number;
  queueDepth: number;
  droppedClicks: number;
  uptimeSeconds: number;
  lastBackupAt: string | null;
  detectedProxyIp?: string;
  oidcEnabled: boolean;
  smtpEnabled: boolean;
  gotifyEnabled: boolean;
  ipLocationEnabled: boolean;
  mcpEnabled: boolean;
}

export interface IPLocation {
  ip: string;
  country: string;
  countryCode: string;
  city: string;
  subdivision: string;
  subdivisionCode: string;
  postal: string;
  latitude: number;
  longitude: number;
  timezone: string;
  asn: number;
  asnOrganization: string;
}

// --- Notifications ---

export type NotificationKind =
  | "user.registered"
  | "link.expiring_soon"
  | "security.password_changed"
  | "system.backup_failed";

export type NotificationChannel = "email" | "gotify" | "browser";

export interface Notification {
  id: string;
  kind: NotificationKind;
  title: string;
  body: string;
  data: Record<string, unknown> | null;
  priority: "low" | "normal" | "high";
  readAt: string | null;
  createdAt: string;
}

export interface NotificationPreferences {
  channels: Record<NotificationKind, NotificationChannel[]>;
  emailAddress?: string;
  gotifyConfigured: boolean;
}

export interface Page<T> {
  items: T[];
  nextCursor: string | null;
  total?: number;
}

export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
    fields?: Record<string, string>;
    requestId?: string;
  };
}
