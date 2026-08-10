package models

import "time"

type ProcessedEvent struct {
	EventID    string    `gorm:"primaryKey"`
	Topic      string    `gorm:"index;not null"`
	OccurredAt time.Time `gorm:"not null"`
	CreatedAt  time.Time `gorm:"not null"`
}

type QuarantinedEvent struct {
	ID         uint      `gorm:"primaryKey"`
	Topic      string    `gorm:"index;not null"`
	Partition  int32     `gorm:"not null"`
	Offset     int64     `gorm:"not null"`
	Payload    string    `gorm:"type:text;not null"`
	Reason     string    `gorm:"type:text;not null"`
	CreatedAt  time.Time `gorm:"not null"`
	ReplayedAt *time.Time
}

type AuditEvent struct {
	EventID         string    `gorm:"primaryKey" json:"event_id"`
	ActorAccountID  uint64    `gorm:"index" json:"actor_account_id"`
	TargetAccountID uint64    `gorm:"index" json:"target_account_id"`
	Action          string    `gorm:"index;not null" json:"action"`
	Outcome         string    `gorm:"index;not null" json:"outcome"`
	RequestID       string    `gorm:"index" json:"request_id"`
	OccurredAt      time.Time `gorm:"index;not null" json:"occurred_at"`
}

type DailyMetric struct {
	Day                time.Time `gorm:"primaryKey" json:"day"`
	GMV                int64     `json:"gmv"`
	Revenue            int64     `json:"revenue"`
	OrderCount         uint64    `json:"order_count"`
	PaymentAttempts    uint64    `json:"payment_attempts"`
	SuccessfulPayments uint64    `json:"successful_payments"`
}

type ProductDailyMetric struct {
	Day       time.Time `gorm:"primaryKey"`
	ProductID string    `gorm:"primaryKey"`
	Quantity  uint64
	GMV       int64
}

// OrderFact is the normalized input used by the reporting projection.
type OrderFact struct {
	EventID    string
	OccurredAt time.Time
	OrderID    uint64
	ProductID  string
	Quantity   uint64
	GMV        int64
	Status     string
}

// PaymentFact is the normalized input used by the reporting projection.
type PaymentFact struct {
	EventID    string
	OccurredAt time.Time
	PaymentID  string
	OrderID    uint64
	Amount     int64
	Status     string
}

type ProductMetric struct {
	ProductID string
	Quantity  uint64
	GMV       int64
}

type Dashboard struct {
	From               time.Time
	To                 time.Time
	GMV                int64
	Revenue            int64
	OrderCount         uint64
	AverageOrderValue  float64
	PaymentAttempts    uint64
	SuccessfulPayments uint64
	PaymentSuccessRate float64
	TopProducts        []ProductMetric
}
