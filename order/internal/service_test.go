package internal

import "testing"

func TestOrderCreatedEvent(t *testing.T) {
	tests := []struct {
		name           string
		cashOnDelivery bool
		wantType       string
		wantKey        string
	}{
		{name: "online payment", wantType: "order.created", wantKey: "order-created"},
		{name: "cash on delivery", cashOnDelivery: true, wantType: "order.cod_created", wantKey: "order-cod-created"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotKey := orderCreatedEvent(tt.cashOnDelivery)
			if gotType != tt.wantType || gotKey != tt.wantKey {
				t.Fatalf("orderCreatedEvent(%t) = (%q, %q), want (%q, %q)", tt.cashOnDelivery, gotType, gotKey, tt.wantType, tt.wantKey)
			}
		})
	}
}
