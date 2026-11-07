package internal

import "testing"

func TestProposalReservationIDFitsInventoryColumn(t *testing.T) {
	first := proposalReservationID("proposal-1", "product-1")
	if len(first) > 64 {
		t.Fatalf("reservation ID length = %d, want <= 64", len(first))
	}
	if first != proposalReservationID("proposal-1", "product-1") {
		t.Fatal("reservation ID must be stable for retries")
	}
	if first == proposalReservationID("proposal-1", "product-2") {
		t.Fatal("different products must have different reservation IDs")
	}
	if first == proposalReservationID("proposal-2", "product-1") {
		t.Fatal("different proposals must have different reservation IDs")
	}
}
