package io.github.anand34577.shortr.data

import kotlinx.serialization.Serializable

// Mirrors the server's JSON (internal/server/dto.go). Unknown fields are
// ignored, so a newer server never breaks an older app.

@Serializable
data class ClientConfig(
    val name: String = "",
    val siteName: String = "Shortr",
    val baseUrl: String = "",
    val apiVersion: Int = 0,
    val oidc: OidcConfig? = null,
)

@Serializable
data class OidcConfig(
    val issuer: String,
    val clientId: String,
    val displayName: String = "Single Sign-On",
    val scopes: List<String> = listOf("openid", "profile", "email", "offline_access"),
)

@Serializable
data class Me(
    val id: String,
    val email: String,
    val name: String = "",
    val role: String = "user",
    val capabilities: List<String> = emptyList(),
) {
    val displayName: String get() = name.ifBlank { email.substringBefore('@') }
}

@Serializable
data class Utm(
    val source: String? = null,
    val medium: String? = null,
    val campaign: String? = null,
    val term: String? = null,
    val content: String? = null,
)

@Serializable
data class Link(
    val id: String,
    val code: String,
    val shortUrl: String,
    val targetUrl: String,
    val title: String = "",
    val description: String? = null,
    val redirectStatus: Int = 302,
    val hasPassword: Boolean = false,
    val expiresAt: String? = null,
    val maxClicks: Int? = null,
    val clickCount: Long = 0,
    val lastClickAt: String? = null,
    val status: String = "active",
    val tags: List<String> = emptyList(),
    val passQuery: Boolean = true,
    val utm: Utm = Utm(),
    val createdAt: String,
    val updatedAt: String,
    val deletedAt: String? = null,
) {
    val isActive: Boolean get() = status == "active" && deletedAt == null
    val displayTitle: String get() = title.ifBlank { targetUrl.removePrefix("https://").removePrefix("http://").removePrefix("www.") }
}

@Serializable
data class Page<T>(val items: List<T> = emptyList(), val nextCursor: String? = null)

@Serializable
data class SeriesPoint(val bucket: String, val clicks: Long = 0, val uniques: Long = 0, val bots: Long = 0)

@Serializable
data class BreakdownRow(val key: String, val clicks: Long = 0, val pct: Double = 0.0)

@Serializable
data class LinkTotals(val clicks: Long = 0, val uniques: Long = 0, val bots: Long = 0)

@Serializable
data class LinkStats(
    val series: List<SeriesPoint> = emptyList(),
    val totals: LinkTotals = LinkTotals(),
    val byCountry: List<BreakdownRow> = emptyList(),
    val byDevice: List<BreakdownRow> = emptyList(),
    val byOS: List<BreakdownRow> = emptyList(),
    val byBrowser: List<BreakdownRow> = emptyList(),
    val byReferrer: List<BreakdownRow> = emptyList(),
)

@Serializable
data class OverviewTotals(val clicks: Long = 0, val uniques: Long = 0, val activeLinks: Long = 0)

@Serializable
data class Overview(
    val totals: OverviewTotals = OverviewTotals(),
    val series: List<SeriesPoint> = emptyList(),
    val topLinks: List<Link> = emptyList(),
    val topReferrer: String? = null,
    val deltaPct: Double? = null,
)

@Serializable
data class RecentClick(
    val id: String,
    val linkId: String,
    val code: String,
    val shortUrl: String = "",
    val country: String = "",
    val device: String = "",
    val ts: String,
    val referrerHost: String = "",
)

@Serializable
data class AliasCheck(val available: Boolean, val reason: String? = null)

@Serializable
data class ApiErrorBody(val error: ApiErrorDetail)

@Serializable
data class ApiErrorDetail(
    val code: String = "",
    val message: String = "",
    val fields: Map<String, String>? = null,
)

/** What the link editor sends for create; edits send only changed fields. */
data class LinkDraft(
    val targetUrl: String,
    val code: String = "",
    val title: String = "",
    val tags: List<String> = emptyList(),
    val expiresAt: String? = null,
    val maxClicks: Int? = null,
    val password: String = "",
    val redirectStatus: Int? = null,
    val passQuery: Boolean = true,
)
