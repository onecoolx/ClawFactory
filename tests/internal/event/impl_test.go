package event_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/event"
	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"pgregory.net/rapid"
)

// webhookPayload mirrors the unexported event.webhookPayload for test decoding.
type webhookPayload struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Detail     string `json:"detail"`
	Timestamp  string `json:"timestamp"`
}

func newTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "clawfactory-event-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	s, err := store.NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// genEventType generates a random event type from the defined constants.
func genEventType(t *rapid.T) string {
	types := []string{
		model.EventAgentRegistered,
		model.EventAgentDeregistered,
		model.EventAgentOffline,
		model.EventTaskAssigned,
		model.EventTaskCompleted,
		model.EventTaskFailed,
		model.EventTaskRequeued,
		model.EventWorkflowSubmitted,
		model.EventWorkflowCompleted,
		model.EventWorkflowFailed,
	}
	return types[rapid.IntRange(0, len(types)-1).Draw(t, "eventTypeIdx")]
}

// Feature: v03-observability, Property 39: Event persistence round-trip consistency
// Validates: Requirements 3.2
func TestProperty39_EventPersistenceRoundTrip(t *testing.T) {
	s := newTestStore(t)
	bus := event.NewStoreEventBus(s)

	rapid.Check(t, func(t *rapid.T) {
		eventID := rapid.StringMatching(`^evt-[a-z0-9]{8}$`).Draw(t, "eventID")
		eventType := genEventType(t)
		entityType := rapid.SampledFrom([]string{"agent", "task", "workflow"}).Draw(t, "entityType")
		entityID := rapid.StringMatching(`^[a-z]{3}-[a-z0-9]{6}$`).Draw(t, "entityID")
		detail := rapid.SampledFrom([]string{`{}`, `{"key":"value"}`, `{"count":42}`}).Draw(t, "detail")

		ev := model.Event{
			EventID:    eventID,
			EventType:  eventType,
			EntityType: entityType,
			EntityID:   entityID,
			Detail:     detail,
			CreatedAt:  time.Now().UTC().Truncate(time.Second),
		}

		err := bus.Publish(ev)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		events, err := bus.ListEvents(model.EventFilter{})
		if err != nil {
			t.Fatalf("ListEvents failed: %v", err)
		}

		var found bool
		for _, e := range events {
			if e.EventID == eventID {
				found = true
				if e.EventType != eventType {
					t.Fatalf("EventType mismatch: got %q, want %q", e.EventType, eventType)
				}
				if e.EntityType != entityType {
					t.Fatalf("EntityType mismatch: got %q, want %q", e.EntityType, entityType)
				}
				if e.EntityID != entityID {
					t.Fatalf("EntityID mismatch: got %q, want %q", e.EntityID, entityID)
				}
				if e.Detail != detail {
					t.Fatalf("Detail mismatch: got %q, want %q", e.Detail, detail)
				}
				break
			}
		}
		if !found {
			t.Fatalf("published event %q not found in ListEvents result", eventID)
		}
	})
}

// Feature: v03-observability, Property 41: Webhook subscription matching correctness
// Validates: Requirements 4.5, 4.6
func TestProperty41_WebhookSubscriptionMatching(t *testing.T) {
	s := newTestStore(t)
	bus := event.NewStoreEventBus(s)

	rapid.Check(t, func(t *rapid.T) {
		// Set up a test HTTP server that records received payloads.
		var mu sync.Mutex
		var received []webhookPayload

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var p webhookPayload
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			mu.Lock()
			received = append(received, p)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		// Pick 1-3 event types for the subscription.
		allTypes := []string{
			model.EventAgentRegistered,
			model.EventTaskCompleted,
			model.EventWorkflowFailed,
		}
		subCount := rapid.IntRange(1, len(allTypes)).Draw(t, "subCount")
		subscribedTypes := allTypes[:subCount]

		webhookID := rapid.StringMatching(`^wh-[a-z0-9]{6}$`).Draw(t, "webhookID")
		err := s.SaveWebhook(model.WebhookSubscription{
			WebhookID:  webhookID,
			URL:        srv.URL,
			EventTypes: subscribedTypes,
			CreatedAt:  time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("SaveWebhook failed: %v", err)
		}

		// Publish an event with a random type.
		eventType := genEventType(t)
		eventID := rapid.StringMatching(`^evt-[a-z0-9]{8}$`).Draw(t, "eventID")
		ev := model.Event{
			EventID:    eventID,
			EventType:  eventType,
			EntityType: "task",
			EntityID:   "task-001",
			Detail:     `{"info":"test"}`,
			CreatedAt:  time.Now().UTC().Truncate(time.Second),
		}

		err = bus.Publish(ev)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		// Wait for async webhook dispatch.
		time.Sleep(200 * time.Millisecond)

		// Check if the event type is in the subscribed types.
		shouldMatch := false
		for _, st := range subscribedTypes {
			if st == eventType {
				shouldMatch = true
				break
			}
		}

		mu.Lock()
		defer mu.Unlock()

		if shouldMatch {
			// Should have received exactly one callback for this event.
			found := false
			for _, p := range received {
				if p.EventID == eventID {
					found = true
					if p.EventType != eventType {
						t.Fatalf("webhook payload EventType mismatch: got %q, want %q", p.EventType, eventType)
					}
					if p.EntityType != "task" {
						t.Fatalf("webhook payload EntityType mismatch: got %q, want %q", p.EntityType, "task")
					}
					if p.EntityID != "task-001" {
						t.Fatalf("webhook payload EntityID mismatch: got %q, want %q", p.EntityID, "task-001")
					}
					if p.Detail != `{"info":"test"}` {
						t.Fatalf("webhook payload Detail mismatch: got %q", p.Detail)
					}
					if p.Timestamp == "" {
						t.Fatal("webhook payload Timestamp is empty")
					}
					break
				}
			}
			if !found {
				t.Fatalf("expected webhook callback for event %q (type %q) but none received", eventID, eventType)
			}
		} else {
			// Should NOT have received a callback for this event.
			for _, p := range received {
				if p.EventID == eventID {
					t.Fatalf("unexpected webhook callback for event %q (type %q, subscribed: %v)", eventID, eventType, subscribedTypes)
				}
			}
		}

		// Clean up webhook for next iteration.
		_ = s.DeleteWebhook(webhookID)
	})
}

// TestWebhookFailureDoesNotBlock verifies that webhook callback failures don't block event publishing.
func TestWebhookFailureDoesNotBlock(t *testing.T) {
	s := newTestStore(t)
	bus := event.NewStoreEventBus(s)

	// Create a webhook pointing to a server that returns 500.
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failSrv.Close()

	err := s.SaveWebhook(model.WebhookSubscription{
		WebhookID:  "wh-fail-001",
		URL:        failSrv.URL,
		EventTypes: []string{model.EventTaskCompleted},
		CreatedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("SaveWebhook failed: %v", err)
	}

	ev := model.Event{
		EventID:    "evt-fail-001",
		EventType:  model.EventTaskCompleted,
		EntityType: "task",
		EntityID:   "task-fail-001",
		Detail:     `{}`,
		CreatedAt:  time.Now().UTC(),
	}

	// Publish should return nil even though webhook will fail.
	err = bus.Publish(ev)
	if err != nil {
		t.Fatalf("Publish should not return error when webhook fails, got: %v", err)
	}

	// Wait for async dispatch to complete.
	time.Sleep(200 * time.Millisecond)

	// Verify the event was still persisted.
	events, err := bus.ListEvents(model.EventFilter{})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}

	found := false
	for _, e := range events {
		if e.EventID == "evt-fail-001" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("event should be persisted even when webhook callback fails")
	}
}
