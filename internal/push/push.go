// Package push implements Web Push (RFC 8030/8291) notification support for
// the Divine Office PWA.
//
// It is deliberately narrow: it stores browser push subscriptions, sends
// encrypted notifications to them, and exposes a VAPID public key so the
// client can subscribe. Deciding *when* to send each hour's reminder — the
// always-on scheduler — is intentionally left to a follow-up; a subscription's
// desired Schedule is captured and stored here so that work only has to add
// the ticker, not re-plumb the data model.
//
// Push is the one stateful corner of an otherwise stateless server (the
// office.ics feed encodes its whole schedule in the URL). Subscriptions live
// behind the Store interface so the backing store can change without touching
// the handlers; the default is a small JSON file suitable for a single
// instance with a mounted volume.
package push

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Keys are the base64url-encoded ECDH/auth secrets a browser hands back from
// PushSubscription.getKey(), used to encrypt the notification payload.
type Keys struct {
	Auth   string `json:"auth"`
	P256dh string `json:"p256dh"`
}

// Subscription is a browser PushSubscription: the push-service endpoint plus
// the client's encryption keys.
type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     Keys   `json:"keys"`
}

// Schedule mirrors the office.ics reminder configuration: which hours to be
// reminded of, at what local times, on which days, and how far ahead. It is
// stored with each subscription so a future scheduler can act on it. Nothing
// in this package fires on a Schedule yet.
type Schedule struct {
	// Hours maps an hour slug ("lauds", "vespers", …) to a "HH:MM" 24-hour
	// local time. An empty map means the subscriber wants no timed reminders
	// (e.g. they subscribed only to receive test pushes).
	Hours map[string]string `json:"hours,omitempty"`
	// Days holds lowercase three-letter day names ("mon", "sun"); empty means
	// every day.
	Days []string `json:"days,omitempty"`
	// LeadMinutes is how many minutes before the hour to notify; 0 means at
	// the hour.
	LeadMinutes int `json:"leadMinutes"`
	// TZ is an IANA timezone name ("America/New_York"); empty falls back to
	// UTC, matching the ics feed.
	TZ string `json:"tz,omitempty"`
}

// Record is a stored subscription together with its schedule and bookkeeping
// timestamps. The endpoint (inside Subscription) is the primary key.
type Record struct {
	Subscription Subscription `json:"subscription"`
	Schedule     Schedule     `json:"schedule"`
	Created      time.Time    `json:"created"`
	Updated      time.Time    `json:"updated"`
}

// Store persists push subscriptions. Implementations must be safe for
// concurrent use by multiple HTTP handlers. Endpoints identify records.
type Store interface {
	// Put inserts or replaces the record for its endpoint.
	Put(rec Record) error
	// Delete removes the record for endpoint; removing a missing endpoint is
	// not an error.
	Delete(endpoint string) error
	// Get returns the record for endpoint and whether it was found.
	Get(endpoint string) (Record, bool, error)
	// All returns every stored record.
	All() ([]Record, error)
}

// Config holds the VAPID application-server identity. All three fields are
// required for push to be enabled.
type Config struct {
	// PublicKey and PrivateKey are the base64url VAPID keypair (see
	// GenerateVAPIDKeys). The public key is also handed to the browser as the
	// applicationServerKey.
	PublicKey  string
	PrivateKey string
	// Subject identifies this application server to the push service, as a
	// "mailto:" or "https:" URI (RFC 8292 §2.1).
	Subject string
}

// Manager sends Web Push notifications and owns the subscription store.
type Manager struct {
	cfg    Config
	store  Store
	client webpush.HTTPClient // nil uses the library default; injected in tests
	now    func() time.Time
}

// NewManager returns a Manager backed by store. cfg must be complete; callers
// gate on ConfigFromEnv returning ok before constructing one.
func NewManager(cfg Config, store Store) *Manager {
	return &Manager{cfg: cfg, store: store, now: time.Now}
}

// PublicKey returns the VAPID public key the client needs to subscribe.
func (m *Manager) PublicKey() string { return m.cfg.PublicKey }

// Store exposes the underlying subscription store.
func (m *Manager) Store() Store { return m.store }

// Subscribe records (or refreshes) a subscription and its schedule.
func (m *Manager) Subscribe(sub Subscription, sched Schedule) error {
	if sub.Endpoint == "" || sub.Keys.Auth == "" || sub.Keys.P256dh == "" {
		return fmt.Errorf("push: subscription missing endpoint or keys")
	}
	now := m.now()
	created := now
	if existing, ok, err := m.store.Get(sub.Endpoint); err == nil && ok {
		created = existing.Created
	}
	return m.store.Put(Record{
		Subscription: sub,
		Schedule:     sched,
		Created:      created,
		Updated:      now,
	})
}

// Unsubscribe forgets a subscription by endpoint.
func (m *Manager) Unsubscribe(endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("push: empty endpoint")
	}
	return m.store.Delete(endpoint)
}

// Payload is the JSON body delivered to the service worker's push handler.
// The worker turns it into a Notification and routes a click to URL.
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag,omitempty"`
}

// ErrSubscriptionGone reports that the push service has permanently rejected a
// subscription (HTTP 404/410); the caller should delete it. Errors from Send
// wrap this when applicable.
var ErrSubscriptionGone = fmt.Errorf("push: subscription gone")

// Send delivers one encrypted notification to a subscription. On a 404/410
// from the push service it prunes the stored subscription and returns an error
// wrapping ErrSubscriptionGone.
func (m *Manager) Send(ctx context.Context, sub Subscription, payload Payload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("push: marshaling payload: %w", err)
	}

	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{Auth: sub.Keys.Auth, P256dh: sub.Keys.P256dh},
	}, &webpush.Options{
		HTTPClient:      m.client,
		Subscriber:      m.cfg.Subject,
		VAPIDPublicKey:  m.cfg.PublicKey,
		VAPIDPrivateKey: m.cfg.PrivateKey,
		TTL:             30,
		Urgency:         webpush.UrgencyNormal,
	})
	if err != nil {
		return fmt.Errorf("push: sending notification: %w", err)
	}
	defer resp.Body.Close()
	// Drain so the underlying connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		_ = m.store.Delete(sub.Endpoint)
		return fmt.Errorf("push: endpoint rejected with %d: %w", resp.StatusCode, ErrSubscriptionGone)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push: push service returned %d", resp.StatusCode)
	}
	return nil
}

// GenerateVAPIDKeys returns a fresh base64url VAPID keypair for bootstrapping a
// deployment. The private key is a secret; the public key is safe to embed.
func GenerateVAPIDKeys() (privateKey, publicKey string, err error) {
	return webpush.GenerateVAPIDKeys()
}
