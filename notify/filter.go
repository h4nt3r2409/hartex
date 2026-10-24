package notify

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Filter is the contract for the notification_channels.filter JSONB column: the
// filter conditions of a channel instance. All fields are optional, and the
// default is "no filtering" — which is exactly the fallback semantics for
// malformed configuration, see ParseFilter.
type Filter struct {
	// MinSeverity is the minimum severity threshold (low/medium/high/critical);
	// empty = no threshold.
	MinSeverity string `json:"min_severity"`
	// An empty TaskIDs / AssetIDs array means no restriction; when non-empty, the
	// event must intersect it.
	TaskIDs  []int64 `json:"task_ids"`
	AssetIDs []int64 `json:"asset_ids"`
	// An empty VulnClassInclude means accept everything; when non-empty, vulnclass
	// must hit at least one of the keywords.
	// VulnClassExclude excludes on hitting any keyword (exclusion takes priority
	// over inclusion).
	// Matching is a case-insensitive substring — safer than regex: a misconfigured
	// regex will not silently disable the channel.
	VulnClassInclude []string `json:"vulnclass_include"`
	VulnClassExclude []string `json:"vulnclass_exclude"`
	// OnStatusChange determines whether this channel receives finding status
	// change events (meaningful only in realtime mode).
	OnStatusChange bool `json:"on_status_change"`
}

// ParseFilter parses the channel filter configuration.
//
// **Never returns an error.** This is a deliberate design choice: when the filter
// configuration is malformed it always degrades to a zero-value Filter
// (= no filtering = everything matches), because for a vulnerability
// notification system, **pushing one extra item is far better than silently
// missing a high-severity one**. Turning a parse failure into "do not push" would
// give the user a channel that looks configured but actually pushes nothing —
// the worst failure mode.
func ParseFilter(raw []byte) Filter {
	var f Filter
	if len(raw) == 0 {
		return f
	}
	// On parse failure f keeps its zero value, i.e. no filtering.
	_ = json.Unmarshal(raw, &f)
	return f
}

// ValidMinSeverity reports whether s is a valid severity threshold (an empty
// string means no threshold).
func ValidMinSeverity(s string) bool {
	if s == "" {
		return true
	}
	_, ok := severityRank[s]
	return ok
}

// Validate checks the **value-restricted** fields in the filter configuration,
// to be called when saving a channel.
//
// Why it must be caught at write time: Match's judgment for an unknown threshold
// is `rank >= 0`, which is always true — that is, a typo in min_severity ("hgih")
// makes the filter **silently fail** and turn into "push everything". This is
// consistent with the package's "rather push too much than miss one" trade-off
// (nothing is missed), but the consequence is that the user believes they are
// doing tiered pushing while actually dumping every vulnerability into the group,
// with no sign that they configured it wrong. This kind of "silent degradation"
// is exactly what should be caught at the entry point.
//
// Note that Validate is used only on the **write** path. The read path still uses
// ParseFilter's lenient semantics, so bad values already present in historical
// data do not make the channel entirely unreadable.
func (f Filter) Validate() error {
	if !ValidMinSeverity(f.MinSeverity) {
		return fmt.Errorf("invalid minimum severity: %q. Choose one of low / medium / high / critical, or leave empty for no restriction", f.MinSeverity)
	}
	return nil
}

// Match determines whether an event should be delivered to a channel carrying
// these filter conditions.
//
// **Never returns an error**, for the same reason as ParseFilter: any internal
// anomaly is treated as "match". Judgment order: event type → severity threshold
// → task/asset scope → vulnerability type keywords.
func Match(f Filter, s Snapshot) bool {
	// Status change events are received only by channels that explicitly enable
	// them. Off by default, because the vast majority of users expect "push" to
	// mean "a new vulnerability was found", not a running log following every
	// status transition.
	if s.Kind == EventFindingStatusChanged && !f.OnStatusChange {
		return false
	}
	if !AtLeast(s.Severity, f.MinSeverity) {
		return false
	}
	if len(f.TaskIDs) > 0 && !slices.Contains(f.TaskIDs, s.TaskID) {
		return false
	}
	if len(f.AssetIDs) > 0 && !intersectsInt(f.AssetIDs, s.AssetIDs) {
		return false
	}
	// Exclusion takes priority: hitting any exclude keyword eliminates the event,
	// even if it also hits the include list.
	if len(f.VulnClassExclude) > 0 && containsAnyFold(s.VulnClass, f.VulnClassExclude) {
		return false
	}
	if len(f.VulnClassInclude) > 0 && !containsAnyFold(s.VulnClass, f.VulnClassInclude) {
		return false
	}
	return true
}

func intersectsInt(a, b []int64) bool {
	// A linear scan over a small set suffices; both sides are on the order of
	// "a few dozen hand-checked items", so building a map costs more than it gains.
	for _, v := range b {
		if slices.Contains(a, v) {
			return true
		}
	}
	return false
}

// containsAnyFold reports whether s contains any keyword in keywords
// (case-insensitive).
func containsAnyFold(s string, keywords []string) bool {
	lower := strings.ToLower(s)
	for _, kw := range keywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw != "" && strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
