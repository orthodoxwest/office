package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// testSubscription builds a subscription with a real P-256 public key and a
// 16-byte auth secret, so the webpush library's encryption step succeeds and
// the request actually reaches the injected HTTP client.
func testSubscription(t *testing.T, endpoint string) Subscription {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating ECDH key: %v", err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatalf("reading auth bytes: %v", err)
	}
	return Subscription{
		Endpoint: endpoint,
		Keys: Keys{
			P256dh: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
			Auth:   base64.RawURLEncoding.EncodeToString(auth),
		},
	}
}

// mockClient records calls and returns a canned response or error.
type mockClient struct {
	status int
	err    error
	calls  int
}

func (c *mockClient) Do(*http.Request) (*http.Response, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return &http.Response{
		StatusCode: c.status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

func newTestManager(t *testing.T, client webpush.HTTPClient) *Manager {
	t.Helper()
	priv, pub, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("generating VAPID keys: %v", err)
	}
	store, err := NewFileStore(filepath.Join(t.TempDir(), "subs.json"))
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}
	m := NewManager(Config{PublicKey: pub, PrivateKey: priv, Subject: "mailto:test@example.com"}, store)
	m.client = client
	return m
}

func TestFileStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subs.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	rec := Record{Subscription: Subscription{Endpoint: "https://push.example/abc", Keys: Keys{Auth: "a", P256dh: "p"}}}
	if err := store.Put(rec); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Reopen to confirm persistence.
	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, ok, err := reopened.Get(rec.Subscription.Endpoint)
	if err != nil || !ok {
		t.Fatalf("Get after reopen: ok=%v err=%v", ok, err)
	}
	if got.Subscription.Keys.Auth != "a" {
		t.Errorf("persisted auth = %q, want %q", got.Subscription.Keys.Auth, "a")
	}

	if err := reopened.Delete(rec.Subscription.Endpoint); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := reopened.Get(rec.Subscription.Endpoint); ok {
		t.Error("record still present after Delete")
	}
}

func TestSubscribePreservesCreated(t *testing.T) {
	m := newTestManager(t, &mockClient{status: 201})
	sub := testSubscription(t, "https://push.example/keep")

	if err := m.Subscribe(sub, Schedule{LeadMinutes: 10}); err != nil {
		t.Fatalf("first Subscribe: %v", err)
	}
	first, _, _ := m.store.Get(sub.Endpoint)

	if err := m.Subscribe(sub, Schedule{LeadMinutes: 5}); err != nil {
		t.Fatalf("second Subscribe: %v", err)
	}
	second, _, _ := m.store.Get(sub.Endpoint)

	if !second.Created.Equal(first.Created) {
		t.Errorf("Created changed on re-subscribe: %v -> %v", first.Created, second.Created)
	}
	if second.Schedule.LeadMinutes != 5 {
		t.Errorf("schedule not updated: LeadMinutes = %d, want 5", second.Schedule.LeadMinutes)
	}
}

func TestSubscribeRejectsIncomplete(t *testing.T) {
	m := newTestManager(t, &mockClient{status: 201})
	if err := m.Subscribe(Subscription{Endpoint: "https://push.example/x"}, Schedule{}); err == nil {
		t.Error("expected error for subscription missing keys")
	}
}

func TestSendSuccess(t *testing.T) {
	client := &mockClient{status: 201}
	m := newTestManager(t, client)
	sub := testSubscription(t, "https://push.example/ok")

	if err := m.Send(context.Background(), sub, Payload{Title: "t", Body: "b", URL: "/"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if client.calls != 1 {
		t.Errorf("HTTP calls = %d, want 1", client.calls)
	}
}

func TestSendGonePrunesSubscription(t *testing.T) {
	client := &mockClient{status: http.StatusGone}
	m := newTestManager(t, client)
	sub := testSubscription(t, "https://push.example/gone")
	if err := m.Subscribe(sub, Schedule{}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	err := m.Send(context.Background(), sub, Payload{Title: "t"})
	if !errors.Is(err, ErrSubscriptionGone) {
		t.Fatalf("Send error = %v, want ErrSubscriptionGone", err)
	}
	if _, ok, _ := m.store.Get(sub.Endpoint); ok {
		t.Error("gone subscription was not pruned")
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv(EnvPublicKey, "")
	t.Setenv(EnvPrivateKey, "")
	t.Setenv(EnvSubject, "")
	if _, ok := ConfigFromEnv(); ok {
		t.Error("ConfigFromEnv ok=true with no env set")
	}

	t.Setenv(EnvPublicKey, "pub")
	t.Setenv(EnvPrivateKey, "priv")
	t.Setenv(EnvSubject, "mailto:a@b.c")
	cfg, ok := ConfigFromEnv()
	if !ok {
		t.Fatal("ConfigFromEnv ok=false with full env set")
	}
	if cfg.PublicKey != "pub" || cfg.Subject != "mailto:a@b.c" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}
