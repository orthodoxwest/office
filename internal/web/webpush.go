package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/orthodoxwest/office/internal/push"
)

// maxPushBody bounds the JSON we read from subscription requests. A browser
// PushSubscription plus a schedule is well under this.
const maxPushBody = 16 << 10 // 16 KiB

// subscribeRequest is the body posted to /push/subscribe: the browser
// PushSubscription and the reminder schedule the user configured.
type subscribeRequest struct {
	Subscription push.Subscription `json:"subscription"`
	Schedule     push.Schedule     `json:"schedule"`
}

// endpointRequest is the body for endpoint-only actions (unsubscribe, test).
type endpointRequest struct {
	Endpoint string `json:"endpoint"`
}

// decodePushJSON reads and decodes a size-limited JSON request body. Unknown
// fields are ignored on purpose: a browser's PushSubscription.toJSON() carries
// extras (expirationTime, and possibly future keys) that we don't model.
func decodePushJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPushBody))
	if err := dec.Decode(dst); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return false
	}
	return true
}

// handleVAPIDPublicKey returns the application-server public key the browser
// needs to create a subscription. 404 when push is not configured, which the
// client treats as "push unavailable" and hides the UI.
func (s *Server) handleVAPIDPublicKey(w http.ResponseWriter, r *http.Request) {
	if s.push == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{"publicKey": s.push.PublicKey()})
}

// handlePushSubscribe stores (or refreshes) a subscription and its schedule.
func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	if s.push == nil {
		http.Error(w, "push not enabled", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req subscribeRequest
	if !decodePushJSON(w, r, &req) {
		return
	}
	if err := s.push.Subscribe(req.Subscription, req.Schedule); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePushUnsubscribe forgets a subscription by endpoint.
func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if s.push == nil {
		http.Error(w, "push not enabled", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req endpointRequest
	if !decodePushJSON(w, r, &req) {
		return
	}
	if err := s.push.Unsubscribe(req.Endpoint); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePushTest sends one notification to a stored subscription so the user
// can confirm delivery end-to-end. This is the manual counterpart to the
// scheduler that a follow-up will add; it proves the VAPID keys, encryption,
// and service-worker handler all work.
func (s *Server) handlePushTest(w http.ResponseWriter, r *http.Request) {
	if s.push == nil {
		http.Error(w, "push not enabled", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req endpointRequest
	if !decodePushJSON(w, r, &req) {
		return
	}
	rec, ok, err := s.push.Store().Get(req.Endpoint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "unknown subscription", http.StatusNotFound)
		return
	}

	today := time.Now().In(userLocation(r)).Format("2006-01-02")
	payload := push.Payload{
		Title: "Divine Office",
		Body:  "Push notifications are working. You'll be reminded before each hour you chose.",
		URL:   requestBaseURL(r) + "/?date=" + today,
		Tag:   "office-test",
	}
	if err := s.push.Send(r.Context(), rec.Subscription, payload); err != nil {
		if errors.Is(err, push.ErrSubscriptionGone) {
			http.Error(w, "subscription expired — re-enable notifications", http.StatusGone)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
