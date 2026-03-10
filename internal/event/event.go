// Package event defines the event bus interface for platform event publishing and querying.
package event

import "github.com/clawfactory/clawfactory/internal/model"

// EventBus defines the interface for publishing and querying platform events.
type EventBus interface {
	// Publish persists an event and asynchronously dispatches webhooks.
	Publish(event model.Event) error
	// ListEvents returns events matching the given filter.
	ListEvents(filter model.EventFilter) ([]model.Event, error)
}
