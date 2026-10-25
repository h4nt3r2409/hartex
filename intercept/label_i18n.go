package intercept

// English table of interception verdict action labels (allow/deny/ask). It is
// used in verdict messages. The enum keys (allow/deny/ask) and the source marker
// "[模型]" are stable identifiers and are not translated.
var judgeActionLabels = map[string]string{
	"allow": "allow",
	"deny":  "deny",
	"ask":   "ask",
}
