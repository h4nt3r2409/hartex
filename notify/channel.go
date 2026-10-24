package notify

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
)

// Channel is the adapter for a notification channel. Implementations must be
// **stateless**: the same instance is reused concurrently by multiple channel
// configurations, and credentials are always passed in through the cfg
// parameter.
type Channel interface {
	// Kind returns the channel type identifier, which must match the registry
	// key.
	Kind() string
	// Validate is called when saving configuration to check required fields and
	// format. The returned error is shown directly to the configurer, so the
	// wording should say "which field is missing" rather than a vague
	// "invalid configuration".
	Validate(cfg map[string]any) error
	// Send delivers one message and returns the **number of entries actually
	// delivered** along with an error.
	//
	// Why return a count: every platform has a message length limit, and when a
	// digest message cannot fit the whole batch it is truncated. If the caller
	// unconditionally marks the whole batch as delivered, the truncated entries
	// simply disappear — they are not visible in the message, the delivery
	// history shows success, and nowhere can it be discovered that a
	// vulnerability was never sent. With kept returned, the caller marks only
	// the first kept entries and leaves the rest for the next batch.
	//
	// A returned error indicates delivery failure, where *PermanentError means
	// it should not be retried. On failure kept is meaningless and the caller
	// should ignore it.
	Send(ctx context.Context, cfg map[string]any, m Message) (int, error)
	// DefaultRatePerMin returns the channel's officially recommended per-minute
	// delivery limit, used as the default rate limit when creating a new channel
	// instance. Returning 0 means no known limit.
	DefaultRatePerMin() int
	// SecretKeys returns the names of keys in this channel's configuration that
	// are credentials. The API masks the values of these keys when echoing and
	// keeps the stored original value when an update receives a masked value.
	// Only the implementation itself knows which fields count as credentials
	// (the entire WeCom webhook URL is a credential, whereas for DingTalk only
	// the secret part is), so this knowledge must come from the channel and must
	// not be guessed by the layer above.
	SecretKeys() []string
	// DestinationKeys returns the names of keys in this channel's configuration
	// that determine "where messages are sent".
	//
	// Like SecretKeys, this is security-related: the destination address and the
	// credentials are two independent sets of fields. If "change only the
	// address, keep the credentials as-is" were allowed, anyone who can modify
	// channel configuration could send the real credentials from the database to
	// a server they control, and channel configuration masking would be
	// completely meaningless.
	// See PrepareConfigUpdate for details.
	DestinationKeys() []string
}

// registry is the channel registry. Explicit literals are used deliberately
// instead of init() self-registration: this way "which channels exist" can be
// seen in one place, and adding a channel exposes omissions at compile time
// rather than relying on runtime side effects.
var registry = map[string]Channel{
	KindDingTalk: dingTalkChannel{},
	KindFeishu:   feishuChannel{},
	KindWeCom:    weComChannel{},
	KindWebhook:  webhookChannel{},
	KindTelegram: telegramChannel{},
	KindEmail:    emailChannel{},
}

// Get looks up a channel implementation by type.
func Get(kind string) (Channel, bool) {
	c, ok := registry[kind]
	return c, ok
}

// ValidKind reports whether kind is a supported channel type.
func ValidKind(kind string) bool {
	_, ok := registry[kind]
	return ok
}

// Kinds returns all supported channel types in lexicographic order (for stable
// display in a UI dropdown).
func Kinds() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// PermanentError marks a delivery failure that should not be retried: bad
// credentials, rejected by the destination, invalid request body, etc. Retrying
// only makes sense for transient failures (network jitter, rate limiting, a 5xx
// from the peer); repeatedly backing off and retrying a permanent failure will
// not succeed and will drown the real error in retry logs.
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// Permanent marks err as a permanent failure. Returns nil when err is nil, so it
// can be written as `return Permanent(someCheck())`.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

// IsPermanent reports whether err's chain carries a permanent failure marker.
func IsPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}

// ---- Configuration-reading helpers ----
//
// Channel configuration comes from a JSONB column in the database; after
// encoding/json deserialization it is a map[string]any, where numbers are always
// float64 and arrays are []any. The helpers below unify this conversion layer and
// tolerate type deviations caused by users leaving things blank in the UI (e.g.
// entering a port as a string).

// cfgString gets a string configuration item, trimming leading and trailing
// whitespace — copying and pasting from a web form easily brings it along.
func cfgString(cfg map[string]any, key string) string {
	v, ok := cfg[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// cfgInt gets an integer configuration item, accepting both float64 (the JSON
// default) and string sources.
func cfgInt(cfg map[string]any, key string) int {
	switch v := cfg[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

// cfgBool gets a boolean configuration item, accepting the strings "true"/"1".
func cfgBool(cfg map[string]any, key string) bool {
	switch v := cfg[key].(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		return s == "true" || s == "1" || s == "yes"
	default:
		return false
	}
}

// cfgStrings gets a string-array configuration item, automatically trimming
// whitespace and dropping empty strings.
func cfgStrings(cfg map[string]any, key string) []string {
	raw, ok := cfg[key].([]any)
	if !ok {
		// Also accept a single string, convenient for form submissions with only
		// one value.
		if s := cfgString(cfg, key); s != "" {
			return []string{s}
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			continue
		}
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// cfgMap gets a string-map configuration item (e.g. custom HTTP headers),
// trimming whitespace from keys and values and dropping empty keys.
func cfgMap(cfg map[string]any, key string) map[string]string {
	raw, ok := cfg[key].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		out[k] = s
	}
	return out
}
