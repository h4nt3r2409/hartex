package notify

// Snapshot is the contract for the notification_events.snapshot JSONB column.
// The writer is the vulnerability-persistence transaction in the db layer; the
// readers are the delivery engine and filter matching in the server layer. It is
// defined in this package because it is the "notification domain" payload: db
// only serializes it and does not understand the field meanings.
//
// Why redundantly store vulnerability fields instead of looking them up at render
// time: a vulnerability may later be renamed, re-graded, or have its status
// changed, but pushed content should reflect the conclusion **at the time the
// event occurred** — looking it up would yield the dangerously misleading "later
// changed to low". In addition, fan-out and rendering therefore do not need to
// JOIN the findings/tasks/assets tables.
type Snapshot struct {
	// Event type: finding_created / finding_status_changed
	Kind      string  `json:"kind"`
	FindingID int64   `json:"finding_id"`
	TaskID    int64   `json:"task_id"`
	VulnClass string  `json:"vulnclass"`
	Name      string  `json:"name"`
	Severity  string  `json:"severity"`
	Summary   string  `json:"summary"`
	AssetIDs  []int64 `json:"asset_ids"`
	// Non-empty only when kind=finding_status_changed.
	FromStatus string `json:"from_status,omitempty"`
	ToStatus   string `json:"to_status,omitempty"`
}

// Item is a vulnerability pending push, for rendering by a channel.
type Item struct {
	FindingID int64
	Name      string
	VulnClass string
	Severity  string
	Summary   string
	// Assets is the resolved display name of the assets (e.g. domain/IP).
	// Filled in by the server layer — this package does not touch the database
	// and cannot obtain the names.
	Assets []string
	// DetailURL is the link back to the vulnerability details; empty means
	// public_base_url is not configured, and it is omitted when rendering.
	DetailURL string
	// Only used by status change events; when both are non-empty it renders as
	// "pending → fixed".
	FromStatus string
	ToStatus   string
}

// IsStatusChange reports whether this entry is a status change event.
func (i Item) IsStatusChange() bool { return i.FromStatus != "" || i.ToStatus != "" }

// Title returns the display title of the entry: the human-assigned name takes
// priority, falling back to the vulnerability type vulnclass, and when both are
// empty a placeholder is used — an empty title is never output.
func (i Item) Title() string {
	if i.Name != "" {
		return i.Name
	}
	if i.VulnClass != "" {
		return i.VulnClass
	}
	return "(unnamed vulnerability)"
}

// Message is the complete content of one channel send.
type Message struct {
	// Length 1 for a single push; a whole batch for a digest push.
	// An empty slice is invalid; the caller must guarantee at least one item.
	Items []Item
	// When Batch=true, render as a digest message (different title, includes the
	// time window and item count).
	Batch bool
	// WindowMinutes is the digest period (minutes), used only when Batch=true for
	// the wording "last N minutes". Deliberately passed in explicitly by
	// configuration rather than computed at render time with time.Since:
	// rendering stays deterministic, which makes it testable.
	WindowMinutes int
	// HomeURL is the platform panel address (global public_base_url); empty means
	// no panel entry is included.
	HomeURL string
}
