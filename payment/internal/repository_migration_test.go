package internal

import (
	"testing"
	"time"

	"github.com/Tuananh165art/GoshopX/payment/models"
	"github.com/Tuananh165art/GoshopX/pkg/migrations"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCheckoutRetryMigrationUpgradesExistingPaymentDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:payment-migration?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// Model the schema left by payment migration 001 before CheckoutKey existed.
	require.NoError(t, db.Exec(`CREATE TABLE transactions (
		id integer primary key autoincrement,
		created_at datetime, updated_at datetime, deleted_at datetime,
		order_id integer, user_id integer, payment_id text, total_price integer,
		settled_price integer, currency text, status varchar(20), reservation_ids text
	)`).Error)
	require.NoError(t, db.AutoMigrate(&migrations.Record{}))
	require.NoError(t, db.Create(&migrations.Record{Service: "payment", Version: "001_initial", AppliedAt: time.Now().UTC()}).Error)

	_, err = NewPostgresRepository(db)
	require.NoError(t, err)
	require.True(t, db.Migrator().HasColumn(&models.Transaction{}, "CheckoutKey"))
	require.True(t, db.Migrator().HasIndex(&models.Transaction{}, "idx_transactions_checkout_key"))
}
