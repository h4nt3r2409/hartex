package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/notify"
)

// Global settings keys (stored in the settings key-value table; no table creation needed).
const (
	// settingNotifyEnabled is the master switch for push. Default on: it is used for a
	// one-click kill switch during maintenance, not as the condition for enabling the
	// feature—the real enabling condition is "is any channel configured".
	settingNotifyEnabled = "notify_enabled"
	// settingNotifyPublicBaseURL is the external access address used to generate
	// vulnerability detail backlinks (e.g. https://artex.example.com). If empty, messages
	// carry no backlink button.
	// The project has no reusable external address configuration, so a new entry is added here.
	settingNotifyPublicBaseURL = "notify_public_base_url"
	// settingNotifyDigestMinutes is the digest-mode interval (minutes).
	settingNotifyDigestMinutes = "notify_digest_interval_min"
)

const (
	// notifyTick is the polling interval of the delivery engine. 3 seconds is the engine's
	// real-time upper bound, and also the main source of latency between "vulnerability
	// persisted" and "message reaches IM".
	notifyTick = 3 * time.Second
	// When notifyBackgroundDisabledEnv is set, the server does not start the background
	// delivery loop (newNotifier(s).Run). Integration tests call stepRealtime·stepDigest
	// directly to verify distribution results deterministically; if a background loop with a
	// 3-second interval processes the same channel concurrently, aggregation wavers with
	// timing. This is a test-only switch to prevent that; if the variable is absent, it starts
	// as usual (production behavior unchanged).
	notifyBackgroundDisabledEnv = "ARTEX_NOTIFY_BACKGROUND_DISABLED"
	// notifyLease is the lease duration when claiming deliveries. It must be significantly
	// greater than the worst-case duration of a single delivery (the notify package's HTTP
	// client timeout is 15 seconds), otherwise the same row would be delivered by two
	// dispatchers at the same time.
	notifyLease = 3 * time.Minute
	// notifyFanOutPerTick limits the number of events dispatched per round, to avoid expanding
	// the entire historical backlog into delivery tasks at once when a channel is first enabled.
	notifyFanOutPerTick = 200
	// notifyDefaultDigestMinutes is the default value of the digest interval.
	notifyDefaultDigestMinutes = 30
	// notifyUnlimitedBurstPerTick is the per-round delivery cap for a channel with no rate
	// limit. Its purpose is to prevent "a channel configured as unlimited + a single scan
	// producing thousands of vulnerabilities" from stalling a single round into a long block.
	notifyUnlimitedBurstPerTick = 50
	// notifyMaxSendsPerChannelPerTick is the maximum number of deliveries per channel per round.
	//
	// This cap is derived backwards from the **lease duration**: claiming stamps the row with a
	// lease (notifyLease = 3 minutes); if the number of serial deliveries in one round is large
	// enough that the worst-case duration exceeds the lease, the later ones' leases expire before
	// they are sent. Within a single process it does not matter (Run is a single goroutine running
	// serially, ticks do not re-enter), but when **two processes share the same database**, the
	// peer will re-claim rows with expired leases and send them again, and will also
	// double-increment attempts and mark failures while the original process is still delivering.
	//
	// Value: 3-minute lease / 30-second single timeout = 6 is **exactly using the full lease**
	// with zero margin and must not be chosen; choosing 5 leaves a 30-second margin on the
	// worst-case 150 seconds. This relationship is pinned down by
	// TestNotifyTickBudgetFitsWithinLease—changing any of notifyLease, notifySendTimeout or this
	// value will make that assertion fail.
	notifyMaxSendsPerChannelPerTick = 5
	// notifySendTimeout is the timeout for a single delivery. It also determines the value of the
	// previous constant; their product must not exceed notifyLease, see
	// TestNotifyTickBudgetFitsWithinLease.
	notifySendTimeout = 30 * time.Second
)

// notifyBackoff is the backoff sequence for failed retries, indexed by the number of attempts made.
// 3 chances (including the first) corresponds to db.MaxNotifyAttempts; the two must be changed together.
var notifyBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	30 * time.Second,
}

// Delivery failure/delay reasons. Stored in notification_deliveries.last_error and, via
// notify_api.go's delivery history response, displayed verbatim in the notification page's
// "delivery history" table (delivery-list.tsx). Therefore, being user-facing text, it is kept
// in Korean (diagnostic logs via log.Printf preserve the original, Z2).
const (
	errDeliveryChannelKindUnregistered = "Unregistered channel type: %q"
	errDeliverySnapshotUnrenderable    = "Could not parse the event snapshot, so this vulnerability could not be turned into a message"
	errDeliveryChannelLengthCapped     = "This message reached the channel length limit, so only the first %d were sent and the rest are deferred to the next batch"
	errDeliveryNoDeliveredCount        = "The channel did not report the number of messages sent (delivered=%d)"
	errDeliveryRetryExhausted          = "Still failed after %d retries: %s"
	errDeliverySnapshotEmpty           = "The event snapshot for delivery item #%d is empty"
	errDeliverySnapshotParse           = "Failed to parse the event snapshot of delivery item #%d: %w"
	errDeliveryBatchAllUnparseable     = "Failed to parse all %d deliveries in the summary batch"
)

// Notifier is the delivery engine for vulnerability push notifications.
//
// It runs alongside Scheduler as an independent goroutine (see server.New). It deliberately
// does not reuse Scheduler's tick: the push real-time requirement (3 seconds) differs from the
// trigger's business rhythm, and their failures are independent—a stalled push should not
// affect agent triggering.
type Notifier struct {
	s  *Server
	pg *db.DB

	// mu protects buckets. There are few channels and low contention; a single mutex is
	// enough, and it is not worth introducing a finer-grained structure for it.
	mu      sync.Mutex
	buckets map[int64]*notifyBucket
}

// notifyBucket is a single channel's token bucket.
//
// A token bucket is used instead of a "count per minute then reset" sliding window because the
// latter has terrible boundary effects: sending 20 at the end of a window and another 20 the
// next instant is 40 in one second to the platform, which gets rate-limited; a token bucket
// refills at a constant rate and naturally avoids this burst.
type notifyBucket struct {
	tokens   float64
	lastFill time.Time
}

func newNotifier(s *Server) *Notifier {
	return &Notifier{s: s, pg: s.m.pg, buckets: map[int64]*notifyBucket{}}
}

// Run loops until ctx ends. Started once by server.New.
func (n *Notifier) Run(ctx context.Context) {
	if n.pg == nil {
		return
	}
	t := time.NewTicker(notifyTick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n.step(ctx)
		}
	}
}

// step runs one round: first dispatch new events, then deliver due tasks.
//
// Any step failure only logs and does not interrupt the loop—a notification system failure
// must never escalate into a process-level problem. Each tick is independent; the next round
// retries naturally.
func (n *Notifier) step(ctx context.Context) {
	if !n.enabled() {
		return
	}
	if _, _, err := n.pg.FanOutPendingEvents(ctx, notifyFanOutPerTick); err != nil {
		log.Printf("[notify] failed to dispatch events: %v", err)
		return
	}
	channels, err := n.pg.ListNotificationChannels(ctx)
	if err != nil {
		log.Printf("[notify] failed to read channels: %v", err)
		return
	}
	baseURL := n.publicBaseURL()
	for _, ch := range channels {
		if !ch.IsEnabled() {
			continue
		}
		// The token bucket's unit is **message count** (equivalent to HTTP request count),
		// not vulnerability count. In realtime mode the two are the same (one vulnerability,
		// one message); in digest mode a whole batch of vulnerabilities is combined into one
		// message, so only one token is consumed.
		//
		// Both modes ask the token bucket first, then claim according to the allowance—the
		// order must not be reversed, otherwise deliveries blocked by rate limiting will
		// already have consumed retry attempts.
		now := time.Now()
		if ch.Mode == db.NotifyModeDigest {
			tokens, claimLimit := digestTickPlan()
			if n.takeTokens(ch.ID, ch.RatePerMin, tokens, now) <= 0 {
				continue
			}
			n.stepDigest(ctx, ch, claimLimit, baseURL)
			continue
		}
		allow := n.takeTokens(ch.ID, ch.RatePerMin, notifyMaxSendsPerChannelPerTick, now)
		if allow <= 0 {
			continue
		}
		n.stepRealtime(ctx, ch, allow, baseURL)
	}
}

// digestTickPlan returns the token consumption and batch size upper bound for digest channels
// this round.
//
// The two return values are **two different dimensions**, which is exactly why this is a
// separate function:
//
//   - tokens is the message count. A batch of vulnerabilities is combined into one message and
//     one HTTP request is sent, so it is always 1. rate_per_min therefore still applies to
//     digest (at most this many digest messages per minute).
//   - claimLimit is the maximum number of vulnerabilities this batch holds. It is constrained
//     only by the in-memory upper bound and is unrelated to the request budget.
//
// There was once an attempt to make rate_per_min apply to digest by passing the per-round
// request budget (notifyMaxSendsPerChannelPerTick, derived from the lease) directly down as the
// batch size. The consequence was that a channel with rate_per_min=20 only refilled 1 token in
// a 3-second tick, so each digest message held only 1 vulnerability—digest degenerated into
// "realtime push with digest wording," readers receiving a string of "1 new vulnerability in
// the last 30 minutes," and db.MaxDigestBatchSize never being reached.
//
// This symptom is hard to spot in end-to-end tests (existing cases all manually pass a
// sufficiently large limit to stepDigest, bypassing the allowance calculation in step), so the
// decision is consolidated here and pinned directly by
// TestDigestTickPlanDecouplesBatchSizeFromSendBudget.
func digestTickPlan() (tokens, claimLimit int) {
	return 1, db.MaxDigestBatchSize
}

// stepRealtime claims and delivers a channel's realtime tasks, one vulnerability per message.
func (n *Notifier) stepRealtime(ctx context.Context, ch *db.NotificationChannel, allow int, baseURL string) {
	deliveries, err := n.pg.ClaimRealtimeDeliveries(ctx, ch.ID, allow, notifyLease)
	if err != nil {
		log.Printf("[notify] failed to claim realtime deliveries channel=%d: %v", ch.ID, err)
		return
	}
	if len(deliveries) == 0 {
		return
	}
	channel, cfg, ok := n.adapt(ch)
	if !ok {
		_ = n.pg.FailDeliveries(ctx, deliveryIDs(deliveries), fmt.Sprintf(trMsg(errDeliveryChannelKindUnregistered), ch.Kind))
		return
	}
	for _, dl := range deliveries {
		msg, err := n.renderSingle(ctx, dl, baseURL)
		if err != nil {
			// A rendering failure is a local data problem; retrying will not make it better.
			_ = n.pg.FailDeliveries(ctx, []int64{dl.ID}, err.Error())
			continue
		}
		n.send(ctx, channel, cfg, msg, []*db.NotificationDelivery{dl})
	}
}

// stepDigest aggregates a channel's pending deliveries into one message and sends it when the
// batch is due.
func (n *Notifier) stepDigest(ctx context.Context, ch *db.NotificationChannel, allow int, baseURL string) {
	window := n.digestInterval()
	due, err := n.pg.DigestBatchDue(ctx, ch.ID, window)
	if err != nil {
		log.Printf("[notify] failed to determine digest batch channel=%d: %v", ch.ID, err)
		return
	}
	if !due {
		return
	}
	deliveries, err := n.pg.ClaimDigestBatch(ctx, ch.ID, allow, notifyLease)
	if err != nil {
		log.Printf("[notify] failed to claim digest batch channel=%d: %v", ch.ID, err)
		return
	}
	if len(deliveries) == 0 {
		return
	}
	channel, cfg, ok := n.adapt(ch)
	if !ok {
		_ = n.pg.FailDeliveries(ctx, deliveryIDs(deliveries), fmt.Sprintf(trMsg(errDeliveryChannelKindUnregistered), ch.Kind))
		return
	}
	msg, included, err := n.renderBatch(ctx, deliveries, baseURL, int(window.Minutes()))
	if err != nil {
		_ = n.pg.FailDeliveries(ctx, deliveryIDs(deliveries), err.Error())
		return
	}
	// Deliveries whose snapshots are broken and could not enter the message must be explicitly
	// marked failed. Otherwise they stay outside included and enter neither the message nor the
	// failure list—when sending succeeds their status is missed by the later batch marking,
	// staying in sending forever until the lease expires and they are repeatedly claimed.
	if skipped := excludeDeliveries(deliveries, included); len(skipped) > 0 {
		reason := errDeliverySnapshotUnrenderable
		if fErr := n.pg.FailDeliveries(ctx, deliveryIDs(skipped), reason); fErr != nil {
			log.Printf("[notify] failed to mark broken-snapshot deliveries failed channel=%s ids=%v: %v", ch.Kind, deliveryIDs(skipped), fErr)
		}
		log.Printf("[notify] skipped %d deliveries with unparseable snapshots channel=%d", len(skipped), ch.ID)
	}
	// Only the ones that entered the message are handed to send: included[i] corresponds
	// strictly to msg.Items[i], and send relies on this correspondence to record "the channel
	// reported it held the first K items" onto the correct delivery rows.
	n.send(ctx, channel, cfg, msg, included)
}

// send delivers and transitions status according to the result.
//
// The same batch of deliveries (possibly dozens in digest mode) shares one send result: either
// all delivered or the whole batch retried. It does not retry item by item—a digest message is
// one message, and resending part of it would corrupt the batch semantics.
//
// The only exception is **splitting caused by the channel length limit**: the channel reports it
// actually held only the first K items, so from item K+1 onward they must be left for the next
// batch rather than being marked successful along with the rest. Otherwise the truncated
// vulnerabilities end up neither in the message nor in the failure list and vanish completely.
func (n *Notifier) send(ctx context.Context, channel notify.Channel, cfg map[string]any, msg notify.Message, deliveries []*db.NotificationDelivery) {
	// Cap each delivery to avoid one stuck channel holding up all remaining channels this round.
	sendCtx, cancel := context.WithTimeout(ctx, notifySendTimeout)
	defer cancel()
	delivered, err := channel.Send(sendCtx, cfg, msg)
	if err == nil && delivered > 0 {
		if delivered > len(deliveries) {
			// The channel cannot report more delivered than there are deliveries; if it does,
			// the rendering layer miscalculated—treat all as delivered and log the problem,
			// better than corrupting the records.
			log.Printf("[notify] channel reported delivered count %d exceeding delivery count %d channel=%s, treating all as delivered",
				delivered, len(deliveries), channel.Kind())
			delivered = len(deliveries)
		}
		sent, rest := deliveries[:delivered], deliveries[delivered:]
		if err := n.pg.MarkDeliveriesSent(ctx, deliveryIDs(sent)); err != nil {
			log.Printf("[notify] failed to mark delivered channel=%s ids=%v: %v", channel.Kind(), deliveryIDs(sent), err)
		}
		if len(rest) > 0 {
			// This message has reached the channel length limit: the rest are immediately
			// requeued and continued by the next tick. Use DeferDeliveries rather than
			// RescheduleDeliveries—this is not a failure and should not consume the retry
			// budget (the optimistic +1 at claim time is subtracted back there).
			if err := n.pg.DeferDeliveries(ctx, deliveryIDs(rest),
				fmt.Sprintf(trMsg(errDeliveryChannelLengthCapped), delivered)); err != nil {
				log.Printf("[notify] failed to queue deferred segment channel=%s ids=%v: %v", channel.Kind(), deliveryIDs(rest), err)
			}
		}
		return
	}
	if err == nil {
		// The channel neither errored nor reported how many were delivered. Treat it as a
		// failure (with backoff) so this delivery is not repeatedly claimed yet never marked off.
		err = fmt.Errorf(trMsg(errDeliveryNoDeliveredCount), delivered)
	}

	// Failure handling is decided **per item**, not by judging on the whole batch's maximum
	// attempt count.
	//
	// It used to be `if maxAttempts(deliveries) >= MaxNotifyAttempts` to fail the whole batch,
	// but attempt counts differ across items in a batch: an old delivery that has already
	// retried twice (attempts=2) would drag a brand-new delivery (attempts=1) in the same batch
	// into failed—a new vulnerability would be permanently lost without a single retry, exactly
	// the opposite of the original intent "don't let old rows drag new rows down."
	permanent := notify.IsPermanent(err)
	var failIDs, exhaustedIDs []int64
	byDelay := map[time.Duration][]int64{}
	for _, dl := range deliveries {
		switch {
		case permanent:
			failIDs = append(failIDs, dl.ID)
		case dl.Attempts >= db.MaxNotifyAttempts:
			exhaustedIDs = append(exhaustedIDs, dl.ID)
		default:
			delay := notifyBackoff[min(dl.Attempts, len(notifyBackoff)-1)]
			byDelay[delay] = append(byDelay[delay], dl.ID)
		}
	}

	if len(failIDs) > 0 {
		if fErr := n.pg.FailDeliveries(ctx, failIDs, err.Error()); fErr != nil {
			log.Printf("[notify] error marking failure status channel=%s ids=%v: %v", channel.Kind(), failIDs, fErr)
		}
	}
	if len(exhaustedIDs) > 0 {
		reason := fmt.Sprintf(trMsg(errDeliveryRetryExhausted), db.MaxNotifyAttempts, err)
		if fErr := n.pg.FailDeliveries(ctx, exhaustedIDs, reason); fErr != nil {
			log.Printf("[notify] error marking failure status channel=%s ids=%v: %v", channel.Kind(), exhaustedIDs, fErr)
		}
	}
	// Reschedule grouped by delay: there are only 3 backoff tiers, so the number of groups is
	// naturally small and there is no need to issue a separate UPDATE per item (which would make
	// a 500-item batch produce 500 round trips).
	for delay, group := range byDelay {
		if rErr := n.pg.RescheduleDeliveries(ctx, group, delay, err.Error()); rErr != nil {
			log.Printf("[notify] failed to reschedule deliveries channel=%s ids=%v: %v", channel.Kind(), group, rErr)
		}
	}
	if len(failIDs)+len(exhaustedIDs) > 0 {
		log.Printf("[notify] delivery failed channel=%d kind=%s permanent failures=%d retries exhausted=%d pending retry=%d: %s",
			deliveries[0].ChannelID, channel.Kind(), len(failIDs), len(exhaustedIDs), len(byDelay), err)
	}
}

// excludeDeliveries returns those in all that are not in keep (compared by pointer identity).
// Used to find deliveries that "could not enter the message"—they must be explicitly handled
// and must not be left in limbo.
func excludeDeliveries(all, keep []*db.NotificationDelivery) []*db.NotificationDelivery {
	inKeep := make(map[*db.NotificationDelivery]bool, len(keep))
	for _, dl := range keep {
		inKeep[dl] = true
	}
	var out []*db.NotificationDelivery
	for _, dl := range all {
		if !inKeep[dl] {
			out = append(out, dl)
		}
	}
	return out
}

// adapt obtains the channel implementation and parses its config.
// ok=false means the kind is unregistered; the delivery should be failed immediately rather
// than retried indefinitely.
func (n *Notifier) adapt(ch *db.NotificationChannel) (notify.Channel, map[string]any, bool) {
	channel, ok := notify.Get(ch.Kind)
	if !ok {
		return nil, nil, false
	}
	var cfg map[string]any
	if len(ch.Config) > 0 {
		// Give an empty map when config parsing fails: the channel's own Validate will report
		// "which field is missing", and that error guides the user to a fix better than a JSON
		// parse error.
		_ = json.Unmarshal(ch.Config, &cfg)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return channel, cfg, true
}

// renderSingle renders a single vulnerability message.
func (n *Notifier) renderSingle(ctx context.Context, dl *db.NotificationDelivery, baseURL string) (notify.Message, error) {
	snap, err := parseSnapshot(dl)
	if err != nil {
		return notify.Message{}, err
	}
	item, err := n.itemFor(ctx, snap, baseURL)
	if err != nil {
		return notify.Message{}, err
	}
	return notify.Message{Items: []notify.Item{item}, HomeURL: baseURL}, nil
}

// renderBatch renders a digest message. Snapshots are parsed item by item—a single broken one
// just skips that item, not taking down the whole digest batch.
//
// The returned included and msg.Items **correspond strictly one-to-one** (i-th delivery ↔ i-th
// item). This correspondence is a hard requirement: the caller decides which first K deliveries
// to mark delivered based on "the channel reported it held the first K items." If a broken
// snapshot is skipped here without removing the skipped delivery from included, the indices
// become misaligned—a broken item that should fail would be marked delivered, and a good item
// would be misjudged as undelivered. The broken ones are explicitly marked failed by the
// caller, see stepDigest.
func (n *Notifier) renderBatch(ctx context.Context, deliveries []*db.NotificationDelivery, baseURL string, windowMinutes int) (notify.Message, []*db.NotificationDelivery, error) {
	items := make([]notify.Item, 0, len(deliveries))
	included := make([]*db.NotificationDelivery, 0, len(deliveries))
	for _, dl := range deliveries {
		snap, err := parseSnapshot(dl)
		if err != nil {
			// A broken snapshot enters neither the message nor included—its handling is the
			// caller's responsibility (explicitly mark failed, rather than sneaking it into
			// "delivered").
			log.Printf("[notify] skipping unparseable snapshot in digest batch delivery=%d: %v", dl.ID, err)
			continue
		}
		item, err := n.itemFor(ctx, snap, baseURL)
		if err != nil {
			return notify.Message{}, nil, err
		}
		items = append(items, item)
		included = append(included, dl)
	}
	if len(items) == 0 {
		return notify.Message{}, nil, fmt.Errorf(trMsg(errDeliveryBatchAllUnparseable), len(deliveries))
	}
	return notify.Message{
		Items:         items,
		Batch:         true,
		WindowMinutes: windowMinutes,
		HomeURL:       baseURL,
	}, included, nil
}

// itemFor renders an event snapshot into a push item, additionally resolving asset names and
// detail backlinks.
func (n *Notifier) itemFor(ctx context.Context, snap notify.Snapshot, baseURL string) (notify.Item, error) {
	assets, err := n.pg.NotificationAssetNames(ctx, snap.AssetIDs)
	if err != nil {
		// Failure to resolve asset names should not block the push: not getting the names is
		// much lighter than not getting the notification, at the cost of one fewer asset line
		// in the message.
		log.Printf("[notify] failed to resolve asset names finding=%d: %v", snap.FindingID, err)
	}
	item := notify.Item{
		FindingID:  snap.FindingID,
		Name:       snap.Name,
		VulnClass:  snap.VulnClass,
		Severity:   snap.Severity,
		Summary:    snap.Summary,
		Assets:     assets,
		FromStatus: snap.FromStatus,
		ToStatus:   snap.ToStatus,
	}
	if baseURL != "" {
		// The detail page route is in web/src/app/(main)/function/findings/detail/page.tsx,
		// which reads the vulnerability id from the query parameter id.
		item.DetailURL = fmt.Sprintf("%s/function/findings/detail?id=%d", baseURL, snap.FindingID)
	}
	return item, nil
}

// takeTokens takes **at most want** tokens from the channel's token bucket and returns the
// number actually taken.
//
// One token = one message (one HTTP request). In realtime mode the caller passes as many as it
// wants; in digest mode a whole batch of vulnerabilities sends only one message, so it passes 1.
//
// The bucket capacity is the channel's per-minute limit, refilled at a constant rate.
// ratePerMin<=0 means no rate limit; it returns a finite but sufficiently large value to
// prevent a single round from being held up by an unbounded backlog.
//
// The want cap is necessary: without it the bucket could only be drained entirely, while the
// caller still has its own per-round cap; extra tokens taken are unused and vanish before the
// next refill—the accumulated burst capacity is never reachable, and even "no pending
// deliveries this round" would still be charged.
func (n *Notifier) takeTokens(channelID int64, ratePerMin, want int, now time.Time) int {
	if want <= 0 {
		return 0
	}
	if ratePerMin <= 0 {
		return min(want, notifyUnlimitedBurstPerTick)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	b := n.buckets[channelID]
	if b == nil {
		b = &notifyBucket{tokens: float64(ratePerMin), lastFill: now}
		n.buckets[channelID] = b
	}
	// Refill according to actual elapsed time, at a rate of ratePerMin/60 per second.
	if elapsed := now.Sub(b.lastFill).Seconds(); elapsed > 0 {
		b.tokens = minF(float64(ratePerMin), b.tokens+elapsed*float64(ratePerMin)/60)
		b.lastFill = now
	}
	// Add a tiny epsilon before truncating: the token count is accumulated as a float, and
	// when filling over two steps 0.5 + 0.5 may yield 0.9999999999, which int() truncates to
	// 0—a mathematically full bucket would yield no tokens. 1e-9 is far less than one token and
	// will not let a real shortfall through.
	take := min(int(b.tokens+1e-9), want)
	if take <= 0 {
		return 0
	}
	b.tokens -= float64(take)
	return take
}

// enabled reads the master switch.
func (n *Notifier) enabled() bool {
	return n.pg.GetBool(settingNotifyEnabled, true)
}

// publicBaseURL returns the external address used for backlinks, with the trailing slash removed.
func (n *Notifier) publicBaseURL() string {
	v, ok, err := n.pg.GetSetting(settingNotifyPublicBaseURL)
	if err != nil || !ok {
		return ""
	}
	return trimTrailingSlash(v)
}

// digestInterval returns the digest interval, falling back to the default value when invalid or unset.
func (n *Notifier) digestInterval() time.Duration {
	v, ok, err := n.pg.GetSetting(settingNotifyDigestMinutes)
	if err != nil || !ok {
		return time.Duration(notifyDefaultDigestMinutes) * time.Minute
	}
	m := 0
	if _, err := fmt.Sscanf(v, "%d", &m); err != nil || m <= 0 {
		return time.Duration(notifyDefaultDigestMinutes) * time.Minute
	}
	return time.Duration(m) * time.Minute
}

// parseSnapshot parses the snapshot of the event corresponding to a delivery.
func parseSnapshot(dl *db.NotificationDelivery) (notify.Snapshot, error) {
	var snap notify.Snapshot
	if len(dl.Snapshot) == 0 {
		return snap, fmt.Errorf(trMsg(errDeliverySnapshotEmpty), dl.ID)
	}
	if err := json.Unmarshal(dl.Snapshot, &snap); err != nil {
		return snap, fmt.Errorf(trMsg(errDeliverySnapshotParse), dl.ID, err)
	}
	if snap.Kind == "" {
		// The event type follows the event row; the copy in the snapshot may have been written
		// by an older version.
		snap.Kind = dl.EventKind
	}
	return snap, nil
}

func deliveryIDs(deliveries []*db.NotificationDelivery) []int64 {
	out := make([]int64, 0, len(deliveries))
	for _, dl := range deliveries {
		out = append(out, dl.ID)
	}
	return out
}

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
