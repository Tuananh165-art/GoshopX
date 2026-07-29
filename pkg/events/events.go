package events

import (
	"time"

	"github.com/google/uuid"
)

type Envelope struct {
	Version    string    `json:"version"`
	EventID    string    `json:"event_id"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
	AccountID  uint64    `json:"account_id,omitempty"`
	EventKey   string    `json:"event_key,omitempty"`
	Data       any       `json:"data"`
}

func New(eventType string, accountID uint64, eventKey string, data any) Envelope {
	return Envelope{
		Version:    "v1",
		EventID:    uuid.NewString(),
		EventType:  eventType,
		OccurredAt: time.Now().UTC(),
		AccountID:  accountID,
		EventKey:   eventKey,
		Data:       data,
	}
}
