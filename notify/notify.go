// Package notify implements the IM / email push channel adaptation layer for
// vulnerability findings.
//
// Layering: this package is a **leaf package** and depends only on the standard
// library. It knows nothing about the database or the server. Channel
// configuration is passed in as map[string]any (corresponding to the
// notification_channels.config JSONB column), and the content to push is passed
// in as a Message. The benefit of this split is that the genuinely error-prone
// parts — signature computation, UTF-8 truncation, filter matching — can be
// unit-tested without PostgreSQL, while the host only has to orchestrate on the
// server side.
//
// Concurrency contract: Channel implementations must be **stateless**. The same
// Channel instance is reused concurrently by multiple channel configurations
// (even multiple bot instances of the same channel); all credentials are passed
// in through the cfg parameter, and things like webhook URLs must not be cached
// into the implementation's own fields.
package notify

// Channel type identifiers. The values are also the valid set for
// notification_channels.kind, validated by a whitelist on the server side
// (same as findings.status: no DB CHECK, so adding channels later is easy).
const (
	KindDingTalk = "dingtalk" // DingTalk custom robot
	KindFeishu   = "feishu"   // Feishu (including Lark) custom robot
	KindWeCom    = "wecom"    // WeCom group robot
	KindWebhook  = "webhook"  // Generic webhook: custom method/headers/JSON template
	KindTelegram = "telegram" // Telegram Bot API
	KindEmail    = "email"    // SMTP email
)

// Event types, corresponding to notification_events.kind.
const (
	EventFindingCreated       = "finding_created"
	EventFindingStatusChanged = "finding_status_changed"
)

// InitKind is the fallback value for an empty kind in config.
const InitKind = KindDingTalk

// severityRank maps a vulnerability severity to a comparable ordinal. Unknown
// severities return 0, so any min_severity setting keeps unknown severities
// out — when in doubt, don't push, to avoid flooding with false positives.
var severityRank = map[string]int{
	"low":      1,
	"medium":   2,
	"high":     3,
	"critical": 4,
}

// SeverityRank returns the ordinal for a severity; unknown severities return 0.
func SeverityRank(severity string) int { return severityRank[severity] }

// severityLabels / statusLabels hold the English display strings rendered into
// push and email messages.
var severityLabels = map[string]string{
	"critical": "🔴 Critical",
	"high":     "🟠 High",
	"medium":   "🟡 Medium",
	"low":      "🔵 Low",
}

var statusLabels = map[string]string{
	"pending":        "Pending",
	"in_progress":    "In progress",
	"confirmed":      "Confirmed",
	"resolved":       "Resolved",
	"fixed":          "Fixed",
	"false_positive": "False positive",
	"ignored":        "Ignored",
	"duplicate":      "Duplicate",
	"risk_accepted":  "Risk accepted",
}

// SeverityLabel returns the English severity name with emoji, used for message
// titles and card colors. Unknown severities are echoed back as-is; nothing is
// invented.
func SeverityLabel(severity string) string {
	if v, ok := severityLabels[severity]; ok {
		return v
	}
	return severity
}

// StatusLabel translates a disposition status into English, for status-change
// messages. Unknown statuses are echoed back as-is.
func StatusLabel(status string) string {
	if v, ok := statusLabels[status]; ok {
		return v
	}
	return status
}

// AtLeast reports whether severity meets the min threshold. An empty min means
// no threshold, so everything passes. Note that unknown severities have ordinal
// 0 and will be rejected by any non-empty min (see the severityRank comment).
func AtLeast(severity, min string) bool {
	if min == "" {
		return true
	}
	return SeverityRank(severity) >= SeverityRank(min)
}
