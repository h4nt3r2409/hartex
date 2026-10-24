package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"
)

// dingTalkChannel implements the DingTalk custom robot.
//
// Platform characteristics (which determine the implementation trade-offs here):
//   - A single robot is limited to 20 messages/minute; excess sends are silently dropped (HTTP may still be 200),
//     so rate limiting must be done on the client side; see DefaultRatePerMin.
//   - Security settings are one of three: signing / custom keyword / IP whitelist. Signing is the only option that does not depend on
//     message content, so only signing is supported (a bare webhook with none of the three enabled is also supported).
//   - Both success and failure return HTTP 200, distinguished by errcode in the body — not checking errcode
//     would record a delivery failure as a success.
type dingTalkChannel struct{}

func (dingTalkChannel) Kind() string { return KindDingTalk }

func (dingTalkChannel) DefaultRatePerMin() int { return 20 }

// The DingTalk Webhook address carries an access_token and is itself a credential, so it is masked as a whole.
func (dingTalkChannel) SecretKeys() []string { return []string{"webhook", "secret"} }

// The destination is the DingTalk Webhook address itself; changing the address requires re-stating the signing secret for the new address at the same time.
func (dingTalkChannel) DestinationKeys() []string { return []string{"webhook"} }

func (dingTalkChannel) Validate(cfg map[string]any) error {
	hook := cfgString(cfg, "webhook")
	if hook == "" {
		return errors.New("Webhook address is missing")
	}
	if err := validateHTTPURL(hook); err != nil {
		return fmt.Errorf("invalid Webhook address: %w", err)
	}
	return nil
}

// Send delivers one message. When there is a backlink and it is a single item, use ActionCard (with a button); otherwise use markdown.
func (c dingTalkChannel) Send(ctx context.Context, cfg map[string]any, m Message) (int, error) {
	hook := cfgString(cfg, "webhook")
	if err := c.Validate(cfg); err != nil {
		return 0, Permanent(err)
	}
	endpoint, err := dingTalkSignedURL(hook, cfgString(cfg, "secret"), time.Now())
	if err != nil {
		return 0, Permanent(err)
	}

	title := markdownTitle(m)
	// DingTalk markdown bodies have no explicit byte limit, but an upper bound is still enforced to prevent evidence fields from growing abnormally.
	text, kept := markdownBody(m, 20000)

	var payload any
	if !m.Batch && len(m.Items) == 1 && m.Items[0].DetailURL != "" {
		payload = map[string]any{
			"msgtype": "actionCard",
			"actionCard": map[string]any{
				"title":          title,
				"text":           text,
				"btnOrientation": "0",
				"singleTitle":    "View details",
				"singleURL":      m.Items[0].DetailURL,
			},
		}
	} else {
		payload = map[string]any{
			"msgtype":  "markdown",
			"markdown": map[string]any{"title": title, "text": text},
		}
	}

	raw, err := doJSON(ctx, "POST", endpoint, nil, payload)
	if err != nil {
		return 0, err
	}
	// DingTalk hides business errors inside a 200 response.
	var res struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return 0, fmt.Errorf("failed to parse DingTalk response: %w (%s)", err, snippet(raw))
	}
	if res.ErrCode != 0 {
		// 301000 is a signature verification failure and 310000 is a keyword mismatch — both are configuration errors
		// that retrying will not fix.
		return 0, Permanent(fmt.Errorf("DingTalk returned an error (%d): %s", res.ErrCode, res.ErrMsg))
	}
	return kept, nil
}

// dingTalkSignedURL appends timestamp and sign parameters to the webhook according to the official signing rules.
//
// Rule: the string to sign = timestamp + "\n" + secret, and the **key of the HMAC-SHA256 is also secret**;
// the result is base64-encoded and then URL-encoded. timestamp is in milliseconds. When secret is empty, return it unchanged,
// to support robots with signing disabled.
func dingTalkSignedURL(hook, secret string, now time.Time) (string, error) {
	if secret == "" {
		return hook, nil
	}
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "\n" + secret))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	u, err := url.Parse(hook)
	if err != nil {
		// Do not pass err through: the error text from url.Parse contains the complete address (including access_token).
		return "", fmt.Errorf("failed to parse Webhook address: %s", redactRequestTarget(hook))
	}
	q := u.Query()
	q.Set("timestamp", ts)
	q.Set("sign", sign)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// validateHTTPURL validates that the address is usable and the protocol is supported, and performs an internal-network check for literal IP targets.
//
// Two points to note:
//
//  1. **The error message must be redacted**. url.Parse itself returns *url.Error, whose Error() carries
//     the **complete original address**, and in this feature the addresses of these vendors have credentials embedded in them (DingTalk access_token,
//     WeCom key, Telegram bot token, Feishu hook id). This used to `return err` directly,
//     so the "invalid address format" error leaked the credentials into the 400 response of the test endpoint,
//     the last_error stored on every delivery, the server logs, and the delivery history API.
//
//  2. **Literal IPs are judged as internal-network directly**, while domains are left for the dial phase (blockInternalDial is the final
//     effective point and also covers DNS rebinding). Checking here is for giving a hint at configuration save time,
//     rather than waiting until the first delivery fails.
//
// Restricting protocols is defensive: file:///gopher:// and the like would make http.Client behave unexpectedly
// (though they are already blocked by the scheme check, there is no reason to open up this surface).
func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("cannot parse address (%s)", redactRequestTarget(raw))
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http or https is supported. Received value: %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("host name is missing")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && isBlockedDialIP(ip) && !allowLocalTargets() {
		return fmt.Errorf("not sending to local or link-local address %s (if you need to send to a local service, set %s=1)", ip, AllowLocalTargetsEnv)
	}
	return nil
}
