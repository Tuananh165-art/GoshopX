package migrations

import (
	"time"

	"gorm.io/gorm"
)

// Record tracks a migration version in each service-owned database.
type Record struct {
	Service   string    `gorm:"primaryKey;size:100"`
	Version   string    `gorm:"primaryKey;size:100"`
	AppliedAt time.Time `gorm:"not null"`
}

func (Record) TableName() string { return "schema_migrations" }

// Run applies a migration once and records it atomically from the service's perspective.
// The callback must be idempotent because a process can stop after applying the schema
// change but before recording the version.
func Run(db *gorm.DB, service, version string, apply func(*gorm.DB) error) error {
	if err := db.AutoMigrate(&Record{}); err != nil {
		return err
	}

	var record Record
	err := db.Where("service = ? AND version = ?", service, version).First(&record).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	if err := apply(db); err != nil {
		return err
	}
	return db.Create(&Record{Service: service, Version: version, AppliedAt: time.Now().UTC()}).Error
}
