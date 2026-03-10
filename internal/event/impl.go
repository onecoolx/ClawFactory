package event

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
)

// webhookPayload is the JSON body sent to webhook subscribers.
type webhookPayload struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Detail     string `json:"detail"`
	Timestamp  string `json:"timestamp"`
}

// StoreEventBus implements EventBus backed by StateStore.
type StoreEventBus struct {
	store  store.StateStore
	client *http.Client
}

// NewStoreEventBus creates a new StoreEventBus.
func NewStoreEventBus(s store.StateStore) *StoreEventBus {
	return &StoreEventBus{
		store:  s,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Publish persists an event synchronously and dispatches webhooks asynchronously.
func (b *StoreEventBus) Publish(event model.Event) error {
	if err := b.store.SaveEvent(event); err != nil {
		return err
	}

	go b.dispatchWebhooks(event)

	return nil
}

// ListEvents returns events matching the given filter.
func (b *StoreEventBus) ListEvents(filter model.EventFilter) ([]model.Event, error) {
	return b.store.ListEvents(filter)
}

// dispatchWebhooks sends HTTP POST callbacks to matching webhook subscribers.
func (b *StoreEventBus) dispatchWebhooks(event model.Event) {
	webhooks, err := b.store.ListWebhooks()
	if err != nil {
		slog.Warn("failed to list webhooks for dispatch", "error", err)
		return
	}

	payload := webhookPayload{
		EventID:    event.EventID,
		EventType:  event.EventType,
		EntityType: event.EntityType,
		EntityID:   event.EntityID,
		Detail:     event.Detail,
		Timestamp:  event.CreatedAt.Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("failed to marshal webhook payload", "error", err)
		return
	}

	for _, wh := range webhooks {
		if matchesEventType(wh.EventTypes, event.EventType) {
			go b.sendWebhook(wh.URL, body)
		}
	}
}

// matchesEventType checks if the event type is in the subscription's event types list.
func matchesEventType(eventTypes []string, eventType string) bool {
	for _, et := range eventTypes {
		if et == eventType {
			return true
		}
	}
	return false
}

// sendWebhook sends an HTTP POST to the given URL with the JSON body.
func (b *StoreEventBus) sendWebhook(url string, body []byte) {
	resp, err := b.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Warn("webhook dispatch failed", "url", url, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("webhook dispatch non-2xx response", "url", url, "status", resp.StatusCode)
	}
}
