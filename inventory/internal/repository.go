package internal

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/Tuananh165art/GoshopX/inventory/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrStockNotFound = errors.New("stock not found")

type Repository interface {
	Close()
	UpsertStock(ctx context.Context, productID string, quantity, reorderLevel int32) (*models.Stock, error)
	GetAvailability(ctx context.Context, productID string) (*models.Availability, error)
	ReserveStock(ctx context.Context, reservationID string, accountID uint64, productID string, quantity int32, expiresAt time.Time, source string) (*models.StockReservation, *models.Availability, error)
	ReleaseReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error)
	CommitReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error)
	ListLowStock(ctx context.Context, limit uint64) ([]*models.Stock, error)
	CleanupExpiredReservations(ctx context.Context) (int64, error)
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) (Repository, error) {
	if err := db.AutoMigrate(&models.Stock{}, &models.StockReservation{}, &models.InventoryEventOutbox{}); err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	return &postgresRepository{db: db}, nil
}

func (repository *postgresRepository) Close() {
	sqlDB, err := repository.db.DB()
	if err == nil {
		if err := sqlDB.Close(); err != nil {
			log.Println("Error closing postgres repository")
			log.Println(err)
		}
	}
}

func (repository *postgresRepository) UpsertStock(ctx context.Context, productID string, quantity, reorderLevel int32) (*models.Stock, error) {
	if quantity < 0 || reorderLevel < 0 {
		return nil, ErrInvalidQuantity
	}

	stock := &models.Stock{
		ProductID:    productID,
		Quantity:     quantity,
		ReorderLevel: reorderLevel,
		UpdatedAt:    time.Now().UTC(),
	}

	if err := repository.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "product_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"quantity", "reorder_level", "updated_at"}),
	}).Create(stock).Error; err != nil {
		return nil, err
	}

	return stock, nil
}

func (repository *postgresRepository) GetAvailability(ctx context.Context, productID string) (*models.Availability, error) {
	if _, err := repository.CleanupExpiredReservations(ctx); err != nil {
		return nil, err
	}

	stock, reservedQuantity, err := repository.loadAvailability(ctx, productID)
	if err != nil {
		return nil, err
	}

	return toAvailability(stock, reservedQuantity), nil
}

func (repository *postgresRepository) ReserveStock(ctx context.Context, reservationID string, accountID uint64, productID string, quantity int32, expiresAt time.Time, source string) (*models.StockReservation, *models.Availability, error) {
	if quantity <= 0 {
		return nil, nil, ErrInvalidQuantity
	}

	tx := repository.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if _, err := repository.cleanupExpiredReservationsTx(ctx, tx, productID); err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	stock, err := repository.lockStock(ctx, tx, productID)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	var reservation models.StockReservation
	existingActive := false
	if reservationID != "" {
		err = tx.WithContext(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reservation_id = ?", reservationID).
			First(&reservation).Error
		if err == nil {
			existingActive = reservation.Status == models.ReservationActive
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return nil, nil, err
		}
	}

	reservedOther, err := repository.sumReservedQuantityTx(ctx, tx, productID, reservationID)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	availableBefore := stock.Quantity - reservedOther
	if quantity > availableBefore {
		tx.Rollback()
		return nil, nil, ErrInsufficientStock
	}

	now := time.Now().UTC()
	if existingActive {
		reservation.AccountID = accountID
		reservation.ProductID = productID
		reservation.Quantity = quantity
		reservation.Source = source
		reservation.Status = models.ReservationActive
		reservation.ExpiresAt = expiresAt
		reservation.UpdatedAt = now
		if err := tx.WithContext(ctx).Save(&reservation).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}
	} else {
		reservation = models.StockReservation{
			ReservationID: reservationID,
			AccountID:     accountID,
			ProductID:     productID,
			Quantity:      quantity,
			Status:        models.ReservationActive,
			Source:        source,
			ExpiresAt:     expiresAt,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.WithContext(ctx).Create(&reservation).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}
	}

	availability := toAvailability(stock, reservedOther+quantity)
	if err := repository.writeOutbox(ctx, tx, "inventory."+string(models.ReservationActive), reservation.ReservationID, models.EventData{
		ProductID:      productID,
		ReservationID:  reservation.ReservationID,
		Quantity:       quantity,
		ReorderLevel:   stock.ReorderLevel,
		AvailableQty:   availability.AvailableQuantity,
		ReservedQty:    availability.ReservedQuantity,
		ReservationFor: source,
		Status:         string(models.ReservationActive),
	}); err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, nil, err
	}

	return &reservation, availability, nil
}

func (repository *postgresRepository) ReleaseReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error) {
	return repository.transitionReservation(ctx, reservationID, models.ReservationReleased)
}

func (repository *postgresRepository) CommitReservation(ctx context.Context, reservationID string) (*models.StockReservation, *models.Availability, error) {
	return repository.transitionReservation(ctx, reservationID, models.ReservationCommitted)
}

func (repository *postgresRepository) ListLowStock(ctx context.Context, limit uint64) ([]*models.Stock, error) {
	if _, err := repository.CleanupExpiredReservations(ctx); err != nil {
		return nil, err
	}

	var stocks []*models.Stock
	query := repository.db.WithContext(ctx).Where("quantity <= reorder_level").Order("quantity ASC")
	if limit > 0 {
		query = query.Limit(int(limit))
	}
	if err := query.Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func (repository *postgresRepository) CleanupExpiredReservations(ctx context.Context) (int64, error) {
	tx := repository.db.WithContext(ctx).Begin()
	count, err := repository.cleanupExpiredReservationsTx(ctx, tx, "")
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (repository *postgresRepository) transitionReservation(ctx context.Context, reservationID string, nextStatus models.ReservationStatus) (*models.StockReservation, *models.Availability, error) {
	tx := repository.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if _, err := repository.cleanupExpiredReservationsTx(ctx, tx, ""); err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	var reservation models.StockReservation
	if err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reservation_id = ?", reservationID).
		First(&reservation).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	stock, err := repository.lockStock(ctx, tx, reservation.ProductID)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	if reservation.Status == nextStatus || reservation.Status == models.ReservationExpired {
		reservedQuantity, err := repository.sumReservedQuantityTx(ctx, tx, reservation.ProductID, "")
		if err != nil {
			tx.Rollback()
			return nil, nil, err
		}
		if err := tx.Commit().Error; err != nil {
			return nil, nil, err
		}
		return &reservation, toAvailability(stock, reservedQuantity), nil
	}

	if nextStatus == models.ReservationCommitted && reservation.Status == models.ReservationActive {
		stock.Quantity -= reservation.Quantity
		stock.UpdatedAt = time.Now().UTC()
		if stock.Quantity < 0 {
			tx.Rollback()
			return nil, nil, ErrInsufficientStock
		}
		if err := tx.WithContext(ctx).Save(stock).Error; err != nil {
			tx.Rollback()
			return nil, nil, err
		}
	}

	reservation.Status = nextStatus
	reservation.UpdatedAt = time.Now().UTC()
	if err := tx.WithContext(ctx).Save(&reservation).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	reservedQuantity, err := repository.sumReservedQuantityTx(ctx, tx, reservation.ProductID, "")
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}
	availability := toAvailability(stock, reservedQuantity)
	if err := repository.writeOutbox(ctx, tx, "inventory."+string(nextStatus), reservation.ReservationID, models.EventData{
		ProductID:      reservation.ProductID,
		ReservationID:  reservation.ReservationID,
		Quantity:       reservation.Quantity,
		ReorderLevel:   stock.ReorderLevel,
		AvailableQty:   availability.AvailableQuantity,
		ReservedQty:    availability.ReservedQuantity,
		ReservationFor: reservation.Source,
		Status:         string(nextStatus),
	}); err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, nil, err
	}

	return &reservation, availability, nil
}

func (repository *postgresRepository) lockStock(ctx context.Context, tx *gorm.DB, productID string) (*models.Stock, error) {
	var stock models.Stock
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&stock, "product_id = ?", productID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStockNotFound
		}
		return nil, err
	}
	return &stock, nil
}

func (repository *postgresRepository) loadAvailability(ctx context.Context, productID string) (*models.Stock, int32, error) {
	var stock models.Stock
	if err := repository.db.WithContext(ctx).First(&stock, "product_id = ?", productID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, ErrStockNotFound
		}
		return nil, 0, err
	}

	reservedQuantity, err := repository.sumReservedQuantityTx(ctx, repository.db, productID, "")
	if err != nil {
		return nil, 0, err
	}
	return &stock, reservedQuantity, nil
}

func (repository *postgresRepository) cleanupExpiredReservationsTx(ctx context.Context, tx *gorm.DB, productID string) (int64, error) {
	query := tx.WithContext(ctx).
		Model(&models.StockReservation{}).
		Where("status = ? AND expires_at <= ?", models.ReservationActive, time.Now().UTC())
	if productID != "" {
		query = query.Where("product_id = ?", productID)
	}
	result := query.Updates(map[string]any{
		"status":     models.ReservationExpired,
		"updated_at": time.Now().UTC(),
	})
	return result.RowsAffected, result.Error
}

func (repository *postgresRepository) sumReservedQuantityTx(ctx context.Context, tx *gorm.DB, productID string, excludeReservationID string) (int32, error) {
	type result struct {
		Total int64
	}
	row := result{}
	query := tx.WithContext(ctx).Model(&models.StockReservation{}).
		Select("COALESCE(SUM(quantity), 0) as total").
		Where("product_id = ? AND status = ? AND expires_at > ?", productID, models.ReservationActive, time.Now().UTC())
	if excludeReservationID != "" {
		query = query.Where("reservation_id <> ?", excludeReservationID)
	}
	if err := query.Scan(&row).Error; err != nil {
		return 0, err
	}
	return int32(row.Total), nil
}

func (repository *postgresRepository) writeOutbox(ctx context.Context, tx *gorm.DB, eventType, eventKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	record := &models.InventoryEventOutbox{
		EventKey:  eventKey + ":" + eventType,
		EventType: eventType,
		Payload:   string(body),
		CreatedAt: time.Now().UTC(),
	}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error
}

func toAvailability(stock *models.Stock, reservedQuantity int32) *models.Availability {
	available := stock.Quantity - reservedQuantity
	if available < 0 {
		available = 0
	}
	return &models.Availability{
		ProductID:         stock.ProductID,
		TotalQuantity:     stock.Quantity,
		ReservedQuantity:  reservedQuantity,
		AvailableQuantity: available,
		ReorderLevel:      stock.ReorderLevel,
		HasActiveLowStock: available <= stock.ReorderLevel,
	}
}

