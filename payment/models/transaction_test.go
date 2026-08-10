package models

import (
	"reflect"
	"testing"
)

func TestTransactionDoesNotPersistLegacyCustomerID(t *testing.T) {
	if _, ok := reflect.TypeOf(Transaction{}).FieldByName("CustomerId"); ok {
		t.Fatal("Transaction must not persist the legacy customer_id field")
	}
}
